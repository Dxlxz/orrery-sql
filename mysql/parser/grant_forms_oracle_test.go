package parser

import "testing"

// TestGrantFormsOracle asserts orrery agrees with a live MySQL 8.0 on
// grantFormCases (container test). The verdict comes from PREPARE, which
// parses the statement without running it.
func TestGrantFormsOracle(t *testing.T) {
	o := startParserOracle(t)
	for _, tc := range grantFormCases {
		t.Run(tc.sql, func(t *testing.T) {
			if got := o.prepareAccepts(t, tc.sql); got != tc.accept {
				t.Fatalf("MySQL 8.0 accepts=%v, want %v for %q", got, tc.accept, tc.sql)
			}
			_, err := Parse(tc.sql)
			if orreryAccepts := err == nil; orreryAccepts != tc.accept {
				t.Errorf("DIVERGENCE orrery accepts=%v, MySQL 8.0 accepts=%v for %q: %v",
					orreryAccepts, tc.accept, tc.sql, err)
			}
		})
	}
}
