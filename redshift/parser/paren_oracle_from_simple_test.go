package parser

import "testing"

// TestParenOracleFromSimple transcribes pg/parser/SCENARIOS-pg-paren-dispatch.md §2.2
// (FROM-clause simple-shape corpus) into oracle probes. Each case names the
// canonical FROM-item shape and asserts the OrreryStatus the harness must see
// to be PG-17 aligned. Table fixtures T, U, V, W are pre-created by
// StartParenOracle; see paren_oracle_test.go.
//
// The section covers 21 canonical shapes split into three buckets:
//   - single paren-wrapped relation / malformed content → PGReject/OrreryRejected
//   - paren-wrapped joined_table (all join_type + qual permutations) → OrreryJoinedTable
//   - paren-wrapped disallowed trailers (alias, ONLY, TABLESAMPLE, WITH
//     ORDINALITY, FOR UPDATE, comma-list, LATERAL-prefixed single relation)
//     → PGReject/OrreryRejected
//
// These probes are the baseline fence for `(` dispatch on FROM items:
// §2.3/§2.4/§2.5 layer subquery, nested-join, and mixed-shape variants on
// top of the same infrastructure.
func TestParenOracleFromSimple(t *testing.T) {
	o := StartParenOracle(t)

	cases := []struct {
		name     string
		sql      string
		expected OrreryStatus
	}{
		// --- single paren-wrapped relation: PG rejects (not joined_table) ---
		{
			name:     "single relation in parens",
			sql:      `SELECT * FROM (T)`,
			expected: OrreryRejected,
		},
		{
			name:     "double-wrapped single relation",
			sql:      `SELECT * FROM ((T))`,
			expected: OrreryRejected,
		},

		// --- canonical joined_table shapes: PG accepts, JoinExpr ---
		{
			name:     "JOIN ON TRUE",
			sql:      `SELECT * FROM (T JOIN U ON TRUE)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "CROSS JOIN",
			sql:      `SELECT * FROM (T CROSS JOIN U)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "LEFT JOIN ON TRUE",
			sql:      `SELECT * FROM (T LEFT JOIN U ON TRUE)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "RIGHT JOIN ON TRUE",
			sql:      `SELECT * FROM (T RIGHT JOIN U ON TRUE)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "FULL JOIN ON TRUE",
			sql:      `SELECT * FROM (T FULL JOIN U ON TRUE)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "INNER JOIN ON TRUE",
			sql:      `SELECT * FROM (T INNER JOIN U ON TRUE)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "NATURAL JOIN",
			sql:      `SELECT * FROM (T NATURAL JOIN U)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "NATURAL LEFT JOIN",
			sql:      `SELECT * FROM (T NATURAL LEFT JOIN U)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "NATURAL FULL OUTER JOIN",
			sql:      `SELECT * FROM (T NATURAL FULL OUTER JOIN U)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "JOIN USING",
			sql:      `SELECT * FROM (T JOIN U USING (a))`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "JOIN USING with alias_clause",
			sql:      `SELECT * FROM (T JOIN U USING (a) AS alias_clause)`,
			expected: OrreryJoinedTable,
		},
		{
			name:     "LEFT OUTER JOIN ON expr",
			sql:      `SELECT * FROM (T LEFT OUTER JOIN U ON T.a = U.a)`,
			expected: OrreryJoinedTable,
		},

		// --- paren content that is not a bare joined_table: PG rejects ---
		{
			name:     "LATERAL-prefixed single relation",
			sql:      `SELECT * FROM LATERAL (T)`,
			expected: OrreryRejected,
		},
		{
			name:     "aliased single relation in parens",
			sql:      `SELECT * FROM (T AS alias)`,
			expected: OrreryRejected,
		},
		{
			name:     "comma-list in parens",
			sql:      `SELECT * FROM (T, U)`,
			expected: OrreryRejected,
		},
		{
			name:     "ONLY in parens",
			sql:      `SELECT * FROM (ONLY T)`,
			expected: OrreryRejected,
		},
		{
			name:     "TABLESAMPLE in parens",
			sql:      `SELECT * FROM (T TABLESAMPLE BERNOULLI(10))`,
			expected: OrreryRejected,
		},
		{
			name:     "WITH ORDINALITY in parens",
			sql:      `SELECT * FROM (T WITH ORDINALITY)`,
			expected: OrreryRejected,
		},
		{
			name:     "FOR UPDATE in parens",
			sql:      `SELECT * FROM (T FOR UPDATE)`,
			expected: OrreryRejected,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assertParenParity(t, o, tc.sql, tc.expected)
		})
	}
}
