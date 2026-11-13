package parser

import "testing"

// TestReservedBareWordFormsTiDBOracle asserts orrery agrees with a live TiDB v8.5.5 on
// reservedBareWordCases (container test).
func TestReservedBareWordFormsTiDBOracle(t *testing.T) {
	holdFormsToTiDB(t, reservedBareWordCases)
}
