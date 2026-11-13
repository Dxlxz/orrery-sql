package parser

import "testing"

// TestTableSampleFormsOracle asserts orrery agrees with a live MariaDB on
// tableSampleCases (container test). The verdict comes from PREPARE, which parses
// the statement without running it.
func TestTableSampleFormsOracle(t *testing.T) {
	o := startMariaDB(t)
	for _, c := range tableSampleCases {
		t.Run(c.sql, func(t *testing.T) {
			if got := o.prepareAccepts(t, c.sql); got != c.accept {
				t.Fatalf("MariaDB accepts=%v, want %v for %q", got, c.accept, c.sql)
			}
			orreryAccepts, err := parsesAsOneStatement(c.sql)
			if orreryAccepts != c.accept {
				t.Errorf("DIVERGENCE orrery accepts=%v, MariaDB accepts=%v for %q: %v",
					orreryAccepts, c.accept, c.sql, err)
			}
		})
	}
}
