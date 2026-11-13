package parser

import "testing"

// TestSystemVariableAtFormsTiDBOracle asserts orrery agrees with a live TiDB v8.5.5 on
// systemVariableAtCases (container test).
func TestSystemVariableAtFormsTiDBOracle(t *testing.T) {
	holdFormsToTiDB(t, systemVariableAtCases)
}
