package parser

import "testing"

// TestLimitParamMarkerOracle asserts orrery agrees with a live MySQL 8.0 on a `?`
// parameter marker as a LIMIT/OFFSET operand. MySQL accepts `?` only in a
// prepared statement, so the oracle is read through prepareParses (PREPARE),
// not a direct exec, where `?` is a 1064 syntax error. PREPARE accepts `?` in
// any position, so it cannot measure orrery's deliberate restriction of `?` to
// the LIMIT operand; that restriction is pinned by TestLimitParamMarkerTree.
func TestLimitParamMarkerOracle(t *testing.T) {
	o := startParserOracle(t)
	for _, c := range []struct {
		sql    string
		accept bool
	}{
		{"SELECT * FROM t LIMIT ?", true},
		{"SELECT * FROM t LIMIT ?, ?", true},
		{"SELECT * FROM t LIMIT ? OFFSET ?", true},
	} {
		if got := o.prepareParses(t, c.sql); got != c.accept {
			t.Fatalf("MySQL prepareParses(%q)=%v, want %v", c.sql, got, c.accept)
		}
		_, perr := Parse(c.sql)
		if orreryAccepts := perr == nil; orreryAccepts != c.accept {
			t.Errorf("DIVERGENCE orrery accepts=%v, MySQL accepts=%v for %q: %v",
				orreryAccepts, c.accept, c.sql, perr)
		}
	}
}
