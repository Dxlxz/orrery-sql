package parser

import (
	"context"
	"testing"
)

// TestStatementSeparatorFormsOceanBaseOracle asserts orrery agrees with a live OceanBase CE 4.3.5.6
// on separatorFormCases and createTableQueryFormCases (DSN-gated, see oceanBaseOracle). The
// verdict comes from PREPARE, which parses the statement without running it.
func TestStatementSeparatorFormsOceanBaseOracle(t *testing.T) {
	db := oceanBaseOracle(t)
	ctx := context.Background()
	for _, cases := range [][]separatorFormCase{separatorFormCases, createTableQueryFormCases} {
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				if got := prepareAcceptsOceanBase(t, ctx, db, tc.sql); got != tc.accept {
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
}
