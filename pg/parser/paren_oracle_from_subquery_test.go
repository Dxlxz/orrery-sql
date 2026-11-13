package parser

import "testing"

// TestParenOracleFromSubquery transcribes SCENARIOS-pg-paren-dispatch.md §2.3
// (FROM-clause subquery-shape corpus) into oracle probes. Each case asserts
// the OrreryStatus orrery must emit to be PG-17 aligned for a paren-wrapped
// SELECT/VALUES/TABLE/WITH/set-op subquery FROM item.
//
// Coverage outline:
//   - bare subqueries (plain SELECT, alias, column-alias, N-deep paren nest)
//   - VALUES — bare VALUES inside parens as a FROM item, and with alias
//   - CTE-wrapped SELECT inside parens (WITH ... SELECT)
//   - TABLE name as subquery body
//   - UNION / INTERSECT / EXCEPT set-ops, including the BYT-9315 shape with
//     parenthesized operands which historically misrouted to joined_table.
//
// Fixture tables T/U/V/W and function probe_f() are created by
// StartParenOracle (see paren_oracle_test.go).
func TestParenOracleFromSubquery(t *testing.T) {
	o := StartParenOracle(t)

	cases := []struct {
		name     string
		sql      string
		expected OrreryStatus
	}{
		// --- bare subquery shapes ---
		{
			name:     "plain SELECT subquery",
			sql:      `SELECT * FROM (SELECT 1)`,
			expected: OrrerySubquery,
		},
		{
			name:     "subquery with alias",
			sql:      `SELECT * FROM (SELECT 1) AS s`,
			expected: OrrerySubquery,
		},
		{
			name:     "subquery with column alias list",
			sql:      `SELECT * FROM (SELECT 1) s(x)`,
			expected: OrrerySubquery,
		},
		{
			name:     "double-wrapped SELECT",
			sql:      `SELECT * FROM ((SELECT 1))`,
			expected: OrrerySubquery,
		},
		{
			name:     "triple-wrapped SELECT",
			sql:      `SELECT * FROM (((SELECT 1)))`,
			expected: OrrerySubquery,
		},
		{
			name:     "four-wrapped SELECT",
			sql:      `SELECT * FROM ((((SELECT 1))))`,
			expected: OrrerySubquery,
		},

		// --- VALUES shapes ---
		// Verified against PG 17 oracle: `(VALUES (1))` as a bare FROM
		// item parses as a paren-wrapped select_with_parens — the paren
		// wraps a values_clause, which is a valid select_no_parens
		// alternative. The SCENARIOS "needs alias… verify" note was
		// wrong: PG parses this shape, it only complains about a missing
		// alias on a bare (unwrapped) `VALUES` FROM item. Orrery matches:
		// OrrerySubquery.
		{
			name:     "bare VALUES in parens accepted",
			sql:      `SELECT * FROM (VALUES (1))`,
			expected: OrrerySubquery,
		},
		{
			name:     "VALUES with alias accepted",
			sql:      `SELECT * FROM (VALUES (1)) AS v(a)`,
			expected: OrrerySubquery,
		},
		// Verified against PG 17 oracle: `((VALUES (1)) AS v(a))` is
		// rejected with 42601 syntax error. PG's grammar does not allow
		// an alias on the inner `select_with_parens` — `(subquery) AS v`
		// is only the outer FROM-item alias position, not an inner one.
		// Orrery agrees: OrreryRejected. Worth noting the SCENARIOS line
		// suggested this shape should accept.
		{
			name:     "double-wrapped VALUES with inner alias rejected",
			sql:      `SELECT * FROM ((VALUES (1)) AS v(a))`,
			expected: OrreryRejected,
		},

		// --- CTE-wrapped SELECT ---
		{
			name:     "WITH ... SELECT inside parens",
			sql:      `SELECT * FROM (WITH cte AS (SELECT 1) SELECT * FROM cte)`,
			expected: OrrerySubquery,
		},
		{
			name:     "double-wrapped WITH ... SELECT",
			sql:      `SELECT * FROM ((WITH cte AS (SELECT 1) SELECT * FROM cte))`,
			expected: OrrerySubquery,
		},

		// --- TABLE subquery ---
		// `TABLE T` is a set-op-compatible shorthand for `SELECT * FROM T`;
		// paren-wrapped it forms a valid subquery FROM item.
		{
			name:     "TABLE form subquery",
			sql:      `SELECT * FROM (TABLE T)`,
			expected: OrrerySubquery,
		},

		// --- set-op subqueries ---
		{
			name:     "UNION subquery",
			sql:      `SELECT * FROM (SELECT 1 UNION SELECT 2)`,
			expected: OrrerySubquery,
		},
		{
			name:     "UNION ALL multi-operand",
			sql:      `SELECT * FROM (SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3)`,
			expected: OrrerySubquery,
		},
		{
			name:     "INTERSECT subquery",
			sql:      `SELECT * FROM (SELECT 1 INTERSECT SELECT 2)`,
			expected: OrrerySubquery,
		},
		{
			name:     "EXCEPT subquery",
			sql:      `SELECT * FROM (SELECT 1 EXCEPT SELECT 2)`,
			expected: OrrerySubquery,
		},
		// BYT-9315: parenthesized set-op operands used to misroute through
		// the joined_table branch because `(SELECT 1)` looks like a paren
		// dispatch until the UNION keyword reclassifies the whole thing as
		// a set-op subquery.
		{
			name:     "BYT-9315 parenthesized UNION operands",
			sql:      `SELECT * FROM ((SELECT 1) UNION (SELECT 2))`,
			expected: OrrerySubquery,
		},
		{
			name:     "nested set-op with parens",
			sql:      `SELECT * FROM (((SELECT 1) UNION (SELECT 2)) UNION (SELECT 3))`,
			expected: OrrerySubquery,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assertParenParity(t, o, tc.sql, tc.expected)
		})
	}
}
