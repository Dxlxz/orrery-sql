package parser

import "testing"

// TestOptimizeTableOrTables holds OPTIMIZE's TABLE/TABLES keyword to MariaDB
// 11.8. table_or_tables is TABLE or TABLES and is required, so `OPTIMIZE TABLES
// t` parses and `OPTIMIZE t` with neither is a 1064; each is measured against
// the server and orrery must agree.
func TestOptimizeTableOrTables(t *testing.T) {
	o := startMariaDB(t)
	for _, sql := range []string{
		"OPTIMIZE TABLE t1",
		"OPTIMIZE TABLES t1, t2",
		"OPTIMIZE NO_WRITE_TO_BINLOG TABLES t1",
		"OPTIMIZE LOCAL TABLE t1",
		"OPTIMIZE t1",
		"OPTIMIZE",
	} {
		t.Run(sql, func(t *testing.T) {
			mariaAccepts := o.prepareAccepts(t, sql)
			orreryAccepts, perr := parsesAsOneStatement(sql)
			if orreryAccepts != mariaAccepts {
				t.Errorf("DIVERGENCE orrery accepts=%v, MariaDB accepts=%v for %q: %v",
					orreryAccepts, mariaAccepts, sql, perr)
			}
		})
	}
}
