package parser

import "testing"

// TestAlterColumnFormsOracle asserts orrery agrees with a live mysql:8.0 on
// alterColumnFormCases (container test). The verdict comes from PREPARE, which
// parses the statement without running it: 1064 is the parser's rejection, and
// any other server error means the text parsed and failed later.
func TestAlterColumnFormsOracle(t *testing.T) {
	o := startParserOracle(t)
	for _, c := range alterColumnFormCases {
		t.Run(c.sql, func(t *testing.T) {
			if accepts := o.prepareAccepts(t, c.sql); accepts != c.accept {
				t.Fatalf("MySQL accepts=%v, want %v for %q", accepts, c.accept, c.sql)
			}
			orreryAccepts, perr := parsesAsOneStatement(c.sql)
			if orreryAccepts != c.accept {
				t.Errorf("DIVERGENCE orrery accepts=%v, MySQL accepts=%v for %q: %v",
					orreryAccepts, c.accept, c.sql, perr)
			}
		})
	}
}
