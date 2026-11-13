package parser

import (
	"context"
	"testing"
)

// TestSetListFormsOceanBaseOracle asserts orrery agrees with a live OceanBase on
// setListCases. oceanBaseOracle runs on the package's own OceanBase (see
// testcontainer.OceanBase):
//
//	go test ./oceanbase/parser/ -run 'SetListForms' -count=1
func TestSetListFormsOceanBaseOracle(t *testing.T) {
	db := oceanBaseOracle(t)
	ctx := context.Background()
	for _, c := range setListCases {
		t.Run(c.sql, func(t *testing.T) {
			if got := prepareAcceptsOceanBase(t, ctx, db, c.sql); got != c.accept {
				t.Fatalf("OceanBase accepts=%v, want %v for %q", got, c.accept, c.sql)
			}
			orreryAccepts, err := parsesAsOneStatement(c.sql)
			if orreryAccepts != c.accept {
				t.Errorf("DIVERGENCE orrery accepts=%v, OceanBase accepts=%v for %q: %v",
					orreryAccepts, c.accept, c.sql, err)
			}
		})
	}
}
