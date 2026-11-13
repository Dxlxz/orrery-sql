package parser

import "testing"

// TestParamMarkerExprPositionsOracle asserts orrery agrees with a live MySQL 8.0
// on a `?` parameter marker in the SimpleExpr positions beyond LIMIT: a bare
// operand (param_marker in simple_expr), the offset of LEAD/LAG, and a
// window-frame bound. MySQL accepts `?` only under PREPARE, so the oracle is
// read through prepareParses. The analyzer boundary's rejection of a standalone
// marker is pinned by the catalog tests.
func TestParamMarkerExprPositionsOracle(t *testing.T) {
	o := startParserOracle(t)
	for _, c := range []struct {
		sql    string
		accept bool
	}{
		{"SELECT ?", true},
		{"SELECT 1 + ?", true},
		{"SELECT LEAD(a, ?) OVER () FROM t", true},
		{"SELECT ROW_NUMBER() OVER (ROWS ? PRECEDING) FROM t", true},
		{"SELECT ROW_NUMBER() OVER (ROWS BETWEEN ? FOLLOWING AND UNBOUNDED PRECEDING) FROM t", true},
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
