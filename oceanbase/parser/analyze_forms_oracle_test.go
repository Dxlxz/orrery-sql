package parser

import "testing"

// TestAnalyzeFormsOracle asserts orrery agrees with a live OceanBase on
// analyzeFormCases (container test). The verdict comes from the oracle's
// syntax check, which parses the statement without running it.
func TestAnalyzeFormsOracle(t *testing.T) {
	o := startParserOracle(t)
	for _, tc := range analyzeFormCases {
		t.Run(tc.sql, func(t *testing.T) {
			if got := o.checkSyntax(t, tc.sql); got != tc.accept {
				t.Fatalf("OceanBase accepts=%v, want %v for %q", got, tc.accept, tc.sql)
			}
			_, err := Parse(tc.sql)
			if orreryAccepts := err == nil; orreryAccepts != tc.accept {
				t.Errorf("DIVERGENCE orrery accepts=%v, OceanBase accepts=%v for %q: %v",
					orreryAccepts, tc.accept, tc.sql, err)
			}
		})
	}
}
