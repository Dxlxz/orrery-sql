package parser

import (
	"context"
	"testing"
)

// TestReservedWordFormsOceanBaseOracle asserts orrery agrees with a live
// OceanBase on reservedWordCases. oceanBaseOracle runs on the package's own
// OceanBase (see testcontainer.OceanBase):
//
//	go test ./oceanbase/parser/ -run ReservedWord -count=1
func TestReservedWordFormsOceanBaseOracle(t *testing.T) {
	db := oceanBaseOracle(t)
	ctx := context.Background()
	for _, c := range reservedWordCases {
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

// TestReservedWordSweepOceanBaseOracle holds every registered keyword to a live OceanBase.
func TestReservedWordSweepOceanBaseOracle(t *testing.T) {
	db := oceanBaseOracle(t)
	ctx := context.Background()
	checkReservedWordSweep(t, func(sql string) bool {
		return prepareAcceptsOceanBase(t, ctx, db, sql)
	})
}
