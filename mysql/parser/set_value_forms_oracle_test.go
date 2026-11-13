package parser

import "testing"

// TestSetValueFormsOracle asserts orrery agrees with a live MySQL 8.0 on setValueFormCases
// (container test). The verdict comes from PREPARE, which parses the statement without
// running it.
func TestSetValueFormsOracle(t *testing.T) {
	o := startParserOracle(t)
	for _, c := range setValueFormCases {
		t.Run(c.sql, func(t *testing.T) {
			if got := o.prepareAccepts(t, c.sql); got != c.accept {
				t.Fatalf("MySQL 8.0 accepts=%v, want %v for %q", got, c.accept, c.sql)
			}
			orreryAccepts, err := parsesAsOneStatement(c.sql)
			if orreryAccepts != c.accept {
				t.Errorf("DIVERGENCE orrery accepts=%v, MySQL 8.0 accepts=%v for %q: %v",
					orreryAccepts, c.accept, c.sql, err)
			}
		})
	}
}
