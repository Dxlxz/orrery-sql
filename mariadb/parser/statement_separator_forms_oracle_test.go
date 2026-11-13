package parser

import "testing"

// TestStatementSeparatorFormsOracle asserts orrery agrees with a live MariaDB 11.8.8
// on separatorFormCases and createTableQueryFormCases (container test). The
// verdict comes from PREPARE, which parses the statement without running it.
func TestStatementSeparatorFormsOracle(t *testing.T) {
	o := startMariaDB(t)
	for _, cases := range [][]separatorFormCase{separatorFormCases, createTableQueryFormCases} {
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				if got := o.prepareAccepts(t, tc.sql); got != tc.accept {
					t.Fatalf("MariaDB accepts=%v, want %v for %q", got, tc.accept, tc.sql)
				}
				_, err := Parse(tc.sql)
				if orreryAccepts := err == nil; orreryAccepts != tc.accept {
					t.Errorf("DIVERGENCE orrery accepts=%v, MariaDB accepts=%v for %q: %v",
						orreryAccepts, tc.accept, tc.sql, err)
				}
			})
		}
	}
}
