package parser

import "testing"

// TestWildcardItemFormsOracle asserts orrery agrees with a live MariaDB 11.8 on
// wildcardItemCases (container test). The verdict comes from PREPARE, which
// parses the statement without running it.
func TestWildcardItemFormsOracle(t *testing.T) {
	o := startMariaDB(t)
	for _, tc := range wildcardItemCases {
		t.Run(tc.sql, func(t *testing.T) {
			if got := o.prepareAccepts(t, tc.sql); got != tc.accept {
				t.Fatalf("MariaDB 11.8 accepts=%v, want %v for %q", got, tc.accept, tc.sql)
			}
			_, err := Parse(tc.sql)
			if orreryAccepts := err == nil; orreryAccepts != tc.accept {
				t.Errorf("DIVERGENCE orrery accepts=%v, MariaDB 11.8 accepts=%v for %q: %v",
					orreryAccepts, tc.accept, tc.sql, err)
			}
		})
	}
}
