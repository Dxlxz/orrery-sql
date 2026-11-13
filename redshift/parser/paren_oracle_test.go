package parser

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	nodes "github.com/Dxlxz/orrery-sql/redshift/ast"
	"github.com/jackc/pgx/v5/pgconn"
)

// This file builds the PG 17 testcontainer-backed oracle harness for
// pg-paren-dispatch Phase 2 (SCENARIOS §2.1). It mirrors the pattern in
// first_set_oracle_test.go — a database of its own on the package's shared
// server (oracle_server_test.go), per-probe transaction + rollback. Exports are consumed by the corpus
// workers for sections 2.2–2.8.

// ParenOracle wraps a database on the shared PG 17 server for `(` dispatch
// probes. Tables T, U, V, W and helper function probe_f() are created in it
// at startup so §2.2–§2.7 corpus probes can assume them.
type ParenOracle struct {
	db  *sql.DB
	ctx context.Context
}

// parenOracleSetupError is captured inside parenOracleOnce.Do and
// re-observed by every caller so the first Docker failure propagates to
// all tests (fail in CI, skip locally). Same rationale as
// oracleSetupError in first_set_oracle_test.go.
var parenOracleSetupError error

var (
	parenOracleOnce sync.Once
	parenOracleInst *ParenOracle
)

// StartParenOracle returns the singleton ParenOracle. Callers from
// other test files (§2.2–§2.7 corpus workers) use this entry point.
// On Docker-unavailable local dev the caller skips; in CI it fails.
func StartParenOracle(t *testing.T) *ParenOracle {
	t.Helper()
	parenOracleOnce.Do(func() {
		ctx := context.Background()
		db, err := OracleDatabase(ctx, "orrery_paren")
		if err != nil {
			parenOracleSetupError = err
			return
		}

		// Pre-create fixtures. Columns `a, b, x, y, doc` cover every
		// column referenced by §2.2–§2.7 scenarios. Functions
		// probe_f()/probe_g() exist so corpus shapes like
		// `LATERAL f(t.x)` and `ROWS FROM (f(...), g(...))` can hit
		// real objects (reject vs accept classification then reflects
		// the grammar question, not an "undefined function" error).
		setup := []string{
			`CREATE TABLE T (a int, b int, x int, y int, doc xml)`,
			`CREATE TABLE U (a int, b int, x int, y int)`,
			`CREATE TABLE V (a int, b int)`,
			`CREATE TABLE W (a int, b int)`,
			`CREATE TABLE foo (a int)`,
			`CREATE FUNCTION probe_f(i int) RETURNS int LANGUAGE sql AS 'SELECT $1'`,
			`CREATE FUNCTION probe_g(i int) RETURNS int LANGUAGE sql AS 'SELECT $1'`,
		}
		for _, s := range setup {
			if _, err := db.ExecContext(ctx, s); err != nil {
				db.Close()
				parenOracleSetupError = fmt.Errorf("fixture %q: %w", s, err)
				return
			}
		}

		parenOracleInst = &ParenOracle{db: db, ctx: ctx}
	})

	if parenOracleSetupError != nil {
		// CI must fail — silently skipping erases the entire regression
		// fence this harness exists to provide.
		t.Fatalf("paren oracle unavailable in CI: %v", parenOracleSetupError)
	}
	return parenOracleInst
}

// PGStatus classifies a PG 17 probe outcome for `SELECT * FROM (...)`
// dispatch questions. We collapse to a 2-way axis (Accept/Reject)
// because the AST-shape distinction is carried on the orrery side; PG's
// ScalarArrayOpExpr vs SubLink differences are tested in §3.x corpus.
type PGStatus int

const (
	// PGAccept — the probe parsed successfully, i.e. PG did NOT raise
	// 42601 syntax_error. Semantic errors (42704 undefined_object,
	// 42P01 undefined_table, etc.) still count as Accept for `(`-
	// dispatch purposes: the grammar decision was made before name
	// resolution. Same principle as probeAccept in first_set_oracle.
	PGAccept PGStatus = iota
	PGReject
)

func (s PGStatus) String() string {
	switch s {
	case PGAccept:
		return "accept"
	case PGReject:
		return "reject(42601)"
	default:
		return "unknown"
	}
}

// OrreryStatus classifies how orrery's parser routed the paren-enclosed
// FROM item. The classifier inspects the parsed AST of
// `SELECT * FROM (...)` and reports which node was produced — that IS
// the routing decision the harness needs to validate.
type OrreryStatus int

const (
	// OrrerySubquery — FROM item is *nodes.RangeSubselect, i.e. the `(`
	// was routed to select_with_parens.
	OrrerySubquery OrreryStatus = iota
	// OrreryJoinedTable — FROM item is *nodes.JoinExpr, i.e. `(` was
	// routed to `(` joined_table `)`.
	OrreryJoinedTable
	// OrreryOther — FROM item parsed but is neither subquery nor
	// joined_table (RangeVar, RangeFunction, RangeTableFunc, JsonTable,
	// etc.). Accept-but-different-shape; corpus workers can filter on
	// this.
	OrreryOther
	// OrreryRejected — orrery returned a parse error.
	OrreryRejected
)

func (s OrreryStatus) String() string {
	switch s {
	case OrrerySubquery:
		return "subquery"
	case OrreryJoinedTable:
		return "joined_table"
	case OrreryOther:
		return "other"
	case OrreryRejected:
		return "reject"
	default:
		return "unknown"
	}
}

// ParenProbeResult is one probe's two-sided record. §2.2–§2.7 corpus
// workers collect these and the mismatch detector scans for
// pg_status vs orrery_status disagreements.
type ParenProbeResult struct {
	SQL        string
	PGStatus   PGStatus
	OrreryStatus OrreryStatus
	PGError    string // SQLSTATE + message on PGReject; empty on Accept
	OrreryError  string // orrery parse error message on OrreryRejected; empty otherwise
	Duration   time.Duration
}

// ProbeParen runs one SQL statement through both PG and orrery and
// returns the side-by-side probe record. Caller owns the *testing.T
// integration (assertParenParity does that); ProbeParen itself never
// calls t.Fatal so it can be used from loops and from non-test code.
//
// Classifies the first FROM item (index 0). For non-first FROM items
// (LATERAL probes in §2.6), use ProbeParenAt.
func ProbeParen(ctx context.Context, o *ParenOracle, sqlStr string) *ParenProbeResult {
	return ProbeParenAt(ctx, o, sqlStr, 0)
}

// ProbeParenAt is the general form: classify the FROM-item at the
// supplied 0-based index. Used by §2.6 LATERAL probes where the
// interesting node sits at Items[1].
func ProbeParenAt(ctx context.Context, o *ParenOracle, sqlStr string, index int) *ParenProbeResult {
	start := time.Now()
	pgStatus, pgSQLState, pgMsg := classifyPG(ctx, o, sqlStr)
	orreryStatus, orreryErr := classifyOrreryAt(sqlStr, index)
	r := &ParenProbeResult{
		SQL:        sqlStr,
		PGStatus:   pgStatus,
		OrreryStatus: orreryStatus,
		OrreryError:  orreryErr,
		Duration:   time.Since(start),
	}
	if pgStatus == PGReject {
		r.PGError = fmt.Sprintf("%s: %s", pgSQLState, pgMsg)
	}
	return r
}

// classifyPG runs sqlStr inside a transaction with a short timeout and
// rolls back so DDL/DML side effects can't leak between probes. The
// timeout guards against pathological probes hanging the test binary.
func classifyPG(ctx context.Context, o *ParenOracle, sqlStr string) (PGStatus, string, string) {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := o.db.BeginTx(probeCtx, nil)
	if err != nil {
		// Treat infra-level errors as "reject" with a synthetic
		// SQLSTATE so the caller can distinguish them from real 42601.
		// This mirrors how first_set_oracle handles begin failures,
		// except we return a tagged error instead of t.Fatal so
		// ProbeParen stays Fatalling-free.
		return PGReject, "XX000", fmt.Sprintf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, execErr := tx.ExecContext(probeCtx, sqlStr)
	if execErr == nil {
		return PGAccept, "", ""
	}
	var pgErr *pgconn.PgError
	if errors.As(execErr, &pgErr) {
		if pgErr.Code == "42601" {
			return PGReject, pgErr.Code, pgErr.Message
		}
		// Any non-syntax PG error (undefined_object, undefined_table,
		// datatype_mismatch, etc.) means the parser accepted the SQL.
		// That's what `(` dispatch cares about.
		return PGAccept, "", ""
	}
	// Non-PG error (driver / network). Classify as reject and surface
	// the Go error text — the harness will flag the mismatch loudly if
	// it happens.
	return PGReject, "XXNET", execErr.Error()
}

// classifyOrrery parses sqlStr through orrery's pg parser and classifies
// the first FROM item of the first (or only) SELECT statement. The
// classifier only handles `SELECT ... FROM (...)` shapes — that's the
// Phase 2 probe template. For any other input shape the result is
// OrreryOther, which corpus workers treat as "do not assert parity on
// this probe" noise filter.
//
// This is the Items[0] specialization of classifyOrreryAt; kept as a
// compatibility shim so §2.2–§2.5 and §2.7 corpus workers don't have to
// thread an explicit index when the first FROM item is the one being
// probed.
func classifyOrrery(sqlStr string) (OrreryStatus, string) {
	return classifyOrreryAt(sqlStr, 0)
}

// classifyOrreryAt is the general-form classifier: it inspects the
// FROM-item at the caller-specified 0-based index. §2.6 LATERAL probes
// put the LATERAL-prefixed node at Items[1] (Items[0] is the anchor
// relation T), so classifyOrrery alone would silently always report
// OrreryOther for the anchor and never exercise the LATERAL routing
// decision. Corpus workers that care about a non-first FROM item call
// this variant with the right index.
//
// Index out-of-range returns OrreryRejected with an explanatory message so
// unrouteable input, so the caller's expected-status table can still
// assert something meaningful (typically OrreryOther or OrreryRejected).
func classifyOrreryAt(sqlStr string, index int) (OrreryStatus, string) {
	stmts, err := Parse(sqlStr)
	if err != nil {
		return OrreryRejected, err.Error()
	}
	if stmts == nil || len(stmts.Items) == 0 {
		return OrreryRejected, "no statements"
	}
	raw, ok := stmts.Items[0].(*nodes.RawStmt)
	if !ok {
		return OrreryOther, ""
	}
	sel, ok := raw.Stmt.(*nodes.SelectStmt)
	if !ok {
		return OrreryOther, ""
	}
	if sel.FromClause == nil || len(sel.FromClause.Items) == 0 {
		// `SELECT 1` with no FROM — not a paren-routing question.
		return OrreryOther, ""
	}
	if index < 0 || index >= len(sel.FromClause.Items) {
		return OrreryRejected, "FROM item index out of bounds"
	}
	item := sel.FromClause.Items[index]
	switch item.(type) {
	case *nodes.RangeSubselect:
		return OrrerySubquery, ""
	case *nodes.JoinExpr:
		return OrreryJoinedTable, ""
	default:
		// RangeVar, RangeFunction, RangeTableFunc, JsonTable, etc.
		return OrreryOther, ""
	}
}

// expectedOrreryFor maps a PGStatus + SQL into what OrreryStatus the
// harness considers a MATCH. The asymmetry:
//   - PGAccept alone isn't enough: PG accepts both `(SELECT 1)` (subquery)
//     and `(t JOIN u ON TRUE)` (joined_table), but we want orrery to route
//     them to different nodes. The caller passes the EXPECTED orrery
//     shape (OrrerySubquery / OrreryJoinedTable / OrreryOther) alongside
//     PGAccept so the mismatch detector can flag a wrong-side routing.
//   - PGReject requires OrreryRejected.
//
// assertParenParity accepts the expected OrreryStatus directly (caller-
// supplied), so this mapping is encoded in the test table, not here.

// assertParenParity is the main test entry point. It probes PG + orrery
// and fails the test if orrery's routing disagrees with the expected
// PG-aligned outcome. `expected` is the OrreryStatus the caller asserts
// for this SQL — encoding the PG-17 truth. On mismatch the helper
// prints a side-by-side diff of SQL / pg_status / orrery_status /
// errors so debugging doesn't require re-running the probe by hand.
func assertParenParity(t *testing.T, o *ParenOracle, sqlStr string, expected OrreryStatus) {
	t.Helper()
	assertParenParityAt(t, o, sqlStr, 0, expected)
}

// assertParenParityAt is the index-aware variant of assertParenParity.
// §2.6 LATERAL probes call this with index=1 so the classifier inspects
// the LATERAL-prefixed FROM item (Items[1]) rather than the anchor
// relation at Items[0].
func assertParenParityAt(t *testing.T, o *ParenOracle, sqlStr string, index int, expected OrreryStatus) {
	t.Helper()
	r := ProbeParenAt(o.ctx, o, sqlStr, index)

	// Expected pg_status: Reject iff the caller expects OrreryRejected.
	// This couples "PG rejects" to "orrery must also reject" tightly,
	// which is exactly the invariant we want: accept-vs-reject drift is
	// more serious than sub-vs-join drift.
	wantPGReject := expected == OrreryRejected
	if wantPGReject && r.PGStatus != PGReject {
		t.Errorf("parity mismatch for %q:\n"+
			"  expected PG reject (orrery expected: %s)\n"+
			"  got      PG accept, orrery=%s\n"+
			"  duration %v",
			sqlStr, expected, r.OrreryStatus, r.Duration)
		return
	}
	if !wantPGReject && r.PGStatus == PGReject {
		t.Errorf("parity mismatch for %q:\n"+
			"  expected PG accept (orrery expected: %s)\n"+
			"  got      PG reject: %s\n"+
			"  orrery     %s\n"+
			"  duration %v",
			sqlStr, expected, r.PGError, r.OrreryStatus, r.Duration)
		return
	}

	if r.OrreryStatus != expected {
		t.Errorf("parity mismatch for %q:\n"+
			"  expected orrery=%s\n"+
			"  got      orrery=%s (error=%q)\n"+
			"  pg       %s\n"+
			"  duration %v",
			sqlStr, expected, r.OrreryStatus, r.OrreryError, r.PGStatus, r.Duration)
	}
}

// TestParenOracleHarness is the Section 2.1 self-test. It exercises a
// minimal handful of probes to prove:
//  1. Container boots and accepts DDL at startup (fixtures applied).
//  2. ProbeParen round-trips for both accept and reject SQL.
//  3. classifyOrrery distinguishes RangeSubselect vs JoinExpr vs reject.
//  4. assertParenParity reports mismatches with a side-by-side diff.
//  5. Per-probe timing is captured (budget: ~2 min for 200 probes → ~600ms/probe).
//
// The broader corpus (~200 probes across §2.2–§2.7) is deferred to the
// respective sections' test files, each of which calls StartParenOracle
// and assertParenParity using the exports above.
func TestParenOracleHarness(t *testing.T) {
	o := StartParenOracle(t)

	cases := []struct {
		name     string
		sql      string
		expected OrreryStatus
	}{
		{
			name:     "joined_table single-level",
			sql:      `SELECT * FROM (T JOIN U ON TRUE)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "subquery single-level",
			sql:      `SELECT * FROM (SELECT 1)`,
			expected: OrrerySubquery,
		},
		{
			name:     "empty parens rejected",
			sql:      `SELECT * FROM ()`,
			expected: OrreryRejected,
		},
		{
			name:     "single relation in parens rejected",
			sql:      `SELECT * FROM (T)`,
			expected: OrreryRejected,
		},
		{
			name:     "BYT-9315 shape",
			sql:      `SELECT * FROM ((T JOIN U ON TRUE) JOIN V ON TRUE)`,
			expected: OrreryJoinedTable,
		},
		{
			// OrreryOther fixture: bare RangeVar doesn't exercise `(` dispatch
			// and proves the classifier's default branch (RangeVar/
			// RangeFunction/RangeTableFunc/JsonTable) is reachable from the
			// self-test, not only from §2.6 where LATERAL hides behind
			// Items[1].
			name:     "bare relation (OrreryOther)",
			sql:      `SELECT * FROM T`,
			expected: OrreryOther,
		},
	}

	var total time.Duration
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r := ProbeParen(o.ctx, o, tc.sql)
			total += r.Duration
			t.Logf("probe %q → pg=%s orrery=%s in %v",
				summarize(tc.sql), r.PGStatus, r.OrreryStatus, r.Duration)
			assertParenParity(t, o, tc.sql, tc.expected)
		})
	}
	t.Logf("total harness probe time: %v across %d cases (avg %v)",
		total, len(cases), total/time.Duration(max(len(cases), 1)))
}

// summarize trims long SQL for log lines so `go test -v` output stays
// readable when §2.2+ corpus files start logging dozens of probes.
func summarize(sqlStr string) string {
	const limit = 60
	s := strings.ReplaceAll(sqlStr, "\n", " ")
	if len(s) <= limit {
		return s
	}
	return s[:limit-1] + "…"
}
