package catalog

import (
	"strings"
	"testing"
)

// TestScenario_C18 covers Section C18 "SHOW CREATE TABLE elision rules" from
// mysql/catalog/SCENARIOS-mysql-implicit-behavior.md. Each subtest runs the
// scenario's DDL on both the MySQL 8.0 container and the orrery catalog, then
// asserts that orrery's ShowCreateTable output matches the oracle on the
// specific elision rule under test.
//
// Critical section: every mismatch here breaks SDL round-trip. Failures are
// recorded as t.Error (not t.Fatal) so all 15 scenarios run in one pass, and
// each orrery gap is documented in mysql/catalog/scenarios_bug_queue/c18.md.
func TestScenario_C18(t *testing.T) {
	mc, cleanup := scenarioContainer(t)
	defer cleanup()

	// -----------------------------------------------------------------
	// 18.1 Column charset elided when equal to table default
	// -----------------------------------------------------------------
	t.Run("18_1_column_charset_elision", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		ddl := `CREATE TABLE t (
			a VARCHAR(10),
			b VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci`
		runOnBoth(t, mc, c, ddl)

		mysqlCreate := oracleShow(t, mc, "SHOW CREATE TABLE t")
		orreryCreate := c.ShowCreateTable("testdb", "t")

		// Column a: no column-level CHARACTER SET / COLLATE.
		if strings.Contains(aLine(mysqlCreate, "`a`"), "CHARACTER SET") {
			t.Errorf("oracle: column a should not have CHARACTER SET; got %q", mysqlCreate)
		}
		if strings.Contains(aLine(orreryCreate, "`a`"), "CHARACTER SET") {
			t.Errorf("orrery: column a should not have CHARACTER SET; got %q", orreryCreate)
		}
		// Column b: has non-default collation, so CHARACTER SET/COLLATE rendered.
		if !strings.Contains(aLine(mysqlCreate, "`b`"), "utf8mb4_unicode_ci") {
			t.Errorf("oracle: column b should have COLLATE utf8mb4_unicode_ci; got %q", mysqlCreate)
		}
		if !strings.Contains(aLine(orreryCreate, "`b`"), "utf8mb4_unicode_ci") {
			t.Errorf("orrery: column b should have COLLATE utf8mb4_unicode_ci; got %q", orreryCreate)
		}
	})

	// -----------------------------------------------------------------
	// 18.2 NOT NULL elision: TIMESTAMP shows NULL, others hide it
	// -----------------------------------------------------------------
	t.Run("18_2_null_elision_timestamp", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		ddl := `CREATE TABLE t (
			i    INT,
			i_nn INT NOT NULL,
			ts   TIMESTAMP NULL DEFAULT NULL,
			ts_nn TIMESTAMP NOT NULL DEFAULT '2020-01-01'
		)`
		runOnBoth(t, mc, c, ddl)

		mysqlCreate := oracleShow(t, mc, "SHOW CREATE TABLE t")
		orreryCreate := c.ShowCreateTable("testdb", "t")

		// Oracle: i has no explicit NULL, ts has explicit NULL.
		if strings.Contains(aLine(mysqlCreate, "`i` "), "NULL DEFAULT") {
			// `i int DEFAULT NULL` is fine; we only object to `NULL DEFAULT NULL`.
		}
		if !strings.Contains(aLine(mysqlCreate, "`ts` "), "NULL DEFAULT NULL") {
			t.Errorf("oracle: ts TIMESTAMP should render explicit NULL; got %q", aLine(mysqlCreate, "`ts` "))
		}
		// orrery comparison
		if strings.Contains(aLine(orreryCreate, "`i` "), "`i` int NULL") {
			t.Errorf("orrery: i should not render NULL keyword; got %q", aLine(orreryCreate, "`i` "))
		}
		if !strings.Contains(aLine(orreryCreate, "`ts` "), "NULL") {
			t.Errorf("orrery: ts TIMESTAMP should render explicit NULL; got %q", aLine(orreryCreate, "`ts` "))
		}
	})

	// -----------------------------------------------------------------
	// 18.3 ENGINE always rendered
	// -----------------------------------------------------------------
	t.Run("18_3_engine_always_rendered", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE t (a INT)")

		mysqlCreate := oracleShow(t, mc, "SHOW CREATE TABLE t")
		orreryCreate := c.ShowCreateTable("testdb", "t")

		if !strings.Contains(mysqlCreate, "ENGINE=InnoDB") {
			t.Errorf("oracle: expected ENGINE=InnoDB; got %q", mysqlCreate)
		}
		if !strings.Contains(orreryCreate, "ENGINE=InnoDB") {
			t.Errorf("orrery: expected ENGINE=InnoDB; got %q", orreryCreate)
		}
	})

	// -----------------------------------------------------------------
	// 18.4 AUTO_INCREMENT elided when counter == 1
	// -----------------------------------------------------------------
	t.Run("18_4_auto_increment_elided_when_one", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE t (id INT AUTO_INCREMENT PRIMARY KEY)")

		mysqlCreate := oracleShow(t, mc, "SHOW CREATE TABLE t")
		orreryCreate := c.ShowCreateTable("testdb", "t")

		if strings.Contains(mysqlCreate, "AUTO_INCREMENT=") {
			t.Errorf("oracle: AUTO_INCREMENT= should be elided; got %q", mysqlCreate)
		}
		if strings.Contains(orreryCreate, "AUTO_INCREMENT=") {
			t.Errorf("orrery: AUTO_INCREMENT= should be elided; got %q", orreryCreate)
		}
	})

	// -----------------------------------------------------------------
	// 18.5 DEFAULT CHARSET always rendered
	// -----------------------------------------------------------------
	t.Run("18_5_default_charset_always_rendered", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE tnocs (x INT)")

		mysqlCreate := oracleShow(t, mc, "SHOW CREATE TABLE tnocs")
		orreryCreate := c.ShowCreateTable("testdb", "tnocs")

		if !strings.Contains(mysqlCreate, "DEFAULT CHARSET=utf8mb4") {
			t.Errorf("oracle: DEFAULT CHARSET=utf8mb4 missing; got %q", mysqlCreate)
		}
		if !strings.Contains(mysqlCreate, "COLLATE=utf8mb4_uca1400_ai_ci") {
			t.Errorf("oracle: COLLATE=utf8mb4_uca1400_ai_ci missing; got %q", mysqlCreate)
		}
		if !strings.Contains(orreryCreate, "DEFAULT CHARSET=utf8mb4") {
			t.Errorf("orrery: DEFAULT CHARSET=utf8mb4 missing; got %q", orreryCreate)
		}
		if !strings.Contains(orreryCreate, "COLLATE=utf8mb4_uca1400_ai_ci") {
			t.Errorf("orrery: COLLATE=utf8mb4_uca1400_ai_ci missing; got %q", orreryCreate)
		}
	})

	// -----------------------------------------------------------------
	// 18.6 ROW_FORMAT elided when not explicitly specified
	// -----------------------------------------------------------------
	t.Run("18_6_row_format_elision", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE t (a INT)")
		runOnBoth(t, mc, c, "CREATE TABLE t2 (a INT) ROW_FORMAT=DYNAMIC")

		mysqlT := oracleShow(t, mc, "SHOW CREATE TABLE t")
		mysqlT2 := oracleShow(t, mc, "SHOW CREATE TABLE t2")
		orreryT := c.ShowCreateTable("testdb", "t")
		orreryT2 := c.ShowCreateTable("testdb", "t2")

		if strings.Contains(mysqlT, "ROW_FORMAT=") {
			t.Errorf("oracle: implicit ROW_FORMAT should be elided; got %q", mysqlT)
		}
		if !strings.Contains(mysqlT2, "ROW_FORMAT=DYNAMIC") {
			t.Errorf("oracle: explicit ROW_FORMAT=DYNAMIC missing; got %q", mysqlT2)
		}
		if strings.Contains(orreryT, "ROW_FORMAT=") {
			t.Errorf("orrery: implicit ROW_FORMAT should be elided; got %q", orreryT)
		}
		if !strings.Contains(orreryT2, "ROW_FORMAT=DYNAMIC") {
			t.Errorf("orrery: explicit ROW_FORMAT=DYNAMIC missing; got %q", orreryT2)
		}
	})

	// -----------------------------------------------------------------
	// 18.7 Table-level COLLATE rendering rules
	// -----------------------------------------------------------------
	t.Run("18_7_table_collate_rendering", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE t_prim (x INT) CHARACTER SET latin1")
		runOnBoth(t, mc, c, "CREATE TABLE t_nonprim (x INT) CHARACTER SET latin1 COLLATE latin1_bin")
		runOnBoth(t, mc, c, "CREATE TABLE t_0900 (x INT) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci")

		mysqlPrim := oracleShow(t, mc, "SHOW CREATE TABLE t_prim")
		mysqlNonPrim := oracleShow(t, mc, "SHOW CREATE TABLE t_nonprim")
		mysql0900 := oracleShow(t, mc, "SHOW CREATE TABLE t_0900")
		orreryPrim := c.ShowCreateTable("testdb", "t_prim")
		orreryNonPrim := c.ShowCreateTable("testdb", "t_nonprim")
		orrery0900 := c.ShowCreateTable("testdb", "t_0900")

		// t_prim: DEFAULT CHARSET=latin1, NO COLLATE=.
		if !strings.Contains(mysqlPrim, "DEFAULT CHARSET=latin1") {
			t.Errorf("oracle t_prim: missing DEFAULT CHARSET=latin1; got %q", mysqlPrim)
		}
		// MariaDB writes the collation even when it is the charset's own
		// default, where MySQL 8.0 elides it. Measured on mariadb:11.8.8.
		if !strings.Contains(mysqlPrim, "COLLATE=latin1_swedish_ci") {
			t.Errorf("oracle t_prim: missing COLLATE=latin1_swedish_ci; got %q", mysqlPrim)
		}
		if !strings.Contains(orreryPrim, "COLLATE=latin1_swedish_ci") {
			t.Errorf("orrery t_prim: missing COLLATE=latin1_swedish_ci; got %q", orreryPrim)
		}
		// t_nonprim: COLLATE=latin1_bin.
		if !strings.Contains(mysqlNonPrim, "COLLATE=latin1_bin") {
			t.Errorf("oracle t_nonprim: missing COLLATE=latin1_bin; got %q", mysqlNonPrim)
		}
		if !strings.Contains(orreryNonPrim, "COLLATE=latin1_bin") {
			t.Errorf("orrery t_nonprim: missing COLLATE=latin1_bin; got %q", orreryNonPrim)
		}
		// t_0900 names utf8mb4_0900_ai_ci in its DDL, which MariaDB has as a
		// non-default collation, so the table reports what was asked for.
		// Measured on mariadb:11.8.8.
		if !strings.Contains(mysql0900, "COLLATE=utf8mb4_0900_ai_ci") {
			t.Errorf("oracle t_0900: missing COLLATE=utf8mb4_0900_ai_ci; got %q", mysql0900)
		}
		if !strings.Contains(orrery0900, "COLLATE=utf8mb4_0900_ai_ci") {
			t.Errorf("orrery t_0900: missing COLLATE=utf8mb4_0900_ai_ci; got %q", orrery0900)
		}
	})

	// -----------------------------------------------------------------
	// 18.8 KEY_BLOCK_SIZE elision
	// -----------------------------------------------------------------
	t.Run("18_8_key_block_size_elision", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE t_nokbs (a INT)")
		runOnBoth(t, mc, c, "CREATE TABLE t_kbs (a INT) KEY_BLOCK_SIZE=4")

		mysqlNo := oracleShow(t, mc, "SHOW CREATE TABLE t_nokbs")
		mysqlYes := oracleShow(t, mc, "SHOW CREATE TABLE t_kbs")
		orreryNo := c.ShowCreateTable("testdb", "t_nokbs")
		orreryYes := c.ShowCreateTable("testdb", "t_kbs")

		if strings.Contains(mysqlNo, "KEY_BLOCK_SIZE=") {
			t.Errorf("oracle t_nokbs: KEY_BLOCK_SIZE should be elided; got %q", mysqlNo)
		}
		if !strings.Contains(mysqlYes, "KEY_BLOCK_SIZE=4") {
			t.Errorf("oracle t_kbs: missing KEY_BLOCK_SIZE=4; got %q", mysqlYes)
		}
		if strings.Contains(orreryNo, "KEY_BLOCK_SIZE=") {
			t.Errorf("orrery t_nokbs: KEY_BLOCK_SIZE should be elided; got %q", orreryNo)
		}
		if !strings.Contains(orreryYes, "KEY_BLOCK_SIZE=4") {
			t.Errorf("orrery t_kbs: missing KEY_BLOCK_SIZE=4; got %q", orreryYes)
		}
	})

	// -----------------------------------------------------------------
	// 18.9 COMPRESSION elision
	// -----------------------------------------------------------------
	// MariaDB has no COMPRESSION table option — it answers 1911, "Unknown
	// option 'COMPRESSION'", and spells page compression PAGE_COMPRESSED,
	// which it writes back as an engine-defined option. MySQL 8.0 has
	// COMPRESSION and elides it when unset. Measured on mariadb:11.8.8.
	t.Run("18_9_no_compression_option", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE t_nocomp (a INT)")
		if strings.Contains(oracleShow(t, mc, "SHOW CREATE TABLE t_nocomp"), "COMPRESSION=") {
			t.Errorf("oracle t_nocomp: unexpected COMPRESSION=")
		}
		if strings.Contains(c.ShowCreateTable("testdb", "t_nocomp"), "COMPRESSION=") {
			t.Errorf("orrery t_nocomp: unexpected COMPRESSION=")
		}

		if _, err := mc.db.ExecContext(mc.ctx,
			"CREATE TABLE t_comp (a INT) COMPRESSION='ZLIB'"); err == nil {
			t.Errorf("oracle: expected COMPRESSION='ZLIB' to be refused")
		} else if !strings.Contains(err.Error(), "1911") {
			t.Errorf("oracle: expected 1911 for an unknown option, got %v", err)
		}
	})

	// -----------------------------------------------------------------
	// 18.10 STATS_PERSISTENT / STATS_AUTO_RECALC / STATS_SAMPLE_PAGES
	// -----------------------------------------------------------------
	t.Run("18_10_stats_clauses_elision", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE t_nostats (a INT)")
		runOnBoth(t, mc, c, "CREATE TABLE t_stats (a INT) STATS_PERSISTENT=1 STATS_AUTO_RECALC=0 STATS_SAMPLE_PAGES=32")

		mysqlNo := oracleShow(t, mc, "SHOW CREATE TABLE t_nostats")
		mysqlYes := oracleShow(t, mc, "SHOW CREATE TABLE t_stats")
		orreryNo := c.ShowCreateTable("testdb", "t_nostats")
		orreryYes := c.ShowCreateTable("testdb", "t_stats")

		for _, clause := range []string{"STATS_PERSISTENT=", "STATS_AUTO_RECALC=", "STATS_SAMPLE_PAGES="} {
			if strings.Contains(mysqlNo, clause) {
				t.Errorf("oracle t_nostats: %s should be elided; got %q", clause, mysqlNo)
			}
			if strings.Contains(orreryNo, clause) {
				t.Errorf("orrery t_nostats: %s should be elided; got %q", clause, orreryNo)
			}
		}
		for _, want := range []string{"STATS_PERSISTENT=1", "STATS_AUTO_RECALC=0", "STATS_SAMPLE_PAGES=32"} {
			if !strings.Contains(mysqlYes, want) {
				t.Errorf("oracle t_stats: missing %s; got %q", want, mysqlYes)
			}
			if !strings.Contains(orreryYes, want) {
				t.Errorf("orrery t_stats: missing %s; got %q", want, orreryYes)
			}
		}
	})

	// -----------------------------------------------------------------
	// 18.11 MIN_ROWS / MAX_ROWS / AVG_ROW_LENGTH elision
	// -----------------------------------------------------------------
	t.Run("18_11_min_max_avg_rows_elision", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE t_nominmax (a INT)")
		runOnBoth(t, mc, c, "CREATE TABLE t_minmax (a INT) MIN_ROWS=10 MAX_ROWS=1000 AVG_ROW_LENGTH=256")

		mysqlNo := oracleShow(t, mc, "SHOW CREATE TABLE t_nominmax")
		mysqlYes := oracleShow(t, mc, "SHOW CREATE TABLE t_minmax")
		orreryNo := c.ShowCreateTable("testdb", "t_nominmax")
		orreryYes := c.ShowCreateTable("testdb", "t_minmax")

		for _, clause := range []string{"MIN_ROWS=", "MAX_ROWS=", "AVG_ROW_LENGTH="} {
			if strings.Contains(mysqlNo, clause) {
				t.Errorf("oracle t_nominmax: %s should be elided; got %q", clause, mysqlNo)
			}
			if strings.Contains(orreryNo, clause) {
				t.Errorf("orrery t_nominmax: %s should be elided; got %q", clause, orreryNo)
			}
		}
		for _, want := range []string{"MIN_ROWS=10", "MAX_ROWS=1000", "AVG_ROW_LENGTH=256"} {
			if !strings.Contains(mysqlYes, want) {
				t.Errorf("oracle t_minmax: missing %s; got %q", want, mysqlYes)
			}
			if !strings.Contains(orreryYes, want) {
				t.Errorf("orrery t_minmax: missing %s; got %q", want, orreryYes)
			}
		}
	})

	// -----------------------------------------------------------------
	// 18.12 TABLESPACE clause elision
	// -----------------------------------------------------------------

	// -----------------------------------------------------------------
	// 18.13 PACK_KEYS / CHECKSUM / DELAY_KEY_WRITE elision
	// -----------------------------------------------------------------
	t.Run("18_13_pack_checksum_delay_elision", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, "CREATE TABLE t_none (a INT)")
		runOnBoth(t, mc, c, "CREATE TABLE t_opts (a INT) PACK_KEYS=1 CHECKSUM=1 DELAY_KEY_WRITE=1")

		mysqlNo := oracleShow(t, mc, "SHOW CREATE TABLE t_none")
		mysqlYes := oracleShow(t, mc, "SHOW CREATE TABLE t_opts")
		orreryNo := c.ShowCreateTable("testdb", "t_none")
		orreryYes := c.ShowCreateTable("testdb", "t_opts")

		for _, clause := range []string{"PACK_KEYS=", "CHECKSUM=", "DELAY_KEY_WRITE="} {
			if strings.Contains(mysqlNo, clause) {
				t.Errorf("oracle t_none: %s should be elided; got %q", clause, mysqlNo)
			}
			if strings.Contains(orreryNo, clause) {
				t.Errorf("orrery t_none: %s should be elided; got %q", clause, orreryNo)
			}
		}
		// Oracle may reject or normalize; only assert presence where oracle shows them.
		for _, want := range []string{"PACK_KEYS=1", "CHECKSUM=1", "DELAY_KEY_WRITE=1"} {
			if strings.Contains(mysqlYes, want) && !strings.Contains(orreryYes, want) {
				t.Errorf("orrery t_opts: oracle has %s but orrery does not; got orrery=%q", want, orreryYes)
			}
		}
	})

	// -----------------------------------------------------------------
	// 18.14 Per-index COMMENT and KEY_BLOCK_SIZE inside index clauses
	// -----------------------------------------------------------------
	t.Run("18_14_per_index_comment_kbs", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		ddl := `CREATE TABLE t (
			id INT PRIMARY KEY,
			a  INT,
			b  INT,
			KEY ix_plain (a),
			KEY ix_cmt (b) COMMENT 'hello'
		) KEY_BLOCK_SIZE=4`
		runOnBoth(t, mc, c, ddl)

		mysqlCreate := oracleShow(t, mc, "SHOW CREATE TABLE t")
		orreryCreate := c.ShowCreateTable("testdb", "t")

		// ix_plain: no COMMENT, no per-index KEY_BLOCK_SIZE.
		plainMy := aLine(mysqlCreate, "ix_plain")
		plainOrrery := aLine(orreryCreate, "ix_plain")
		if strings.Contains(plainMy, "COMMENT") {
			t.Errorf("oracle ix_plain: should not have COMMENT; got %q", plainMy)
		}
		if strings.Contains(plainMy, "KEY_BLOCK_SIZE") {
			t.Errorf("oracle ix_plain: should not have KEY_BLOCK_SIZE; got %q", plainMy)
		}
		if strings.Contains(plainOrrery, "COMMENT") {
			t.Errorf("orrery ix_plain: should not have COMMENT; got %q", plainOrrery)
		}
		if strings.Contains(plainOrrery, "KEY_BLOCK_SIZE") {
			t.Errorf("orrery ix_plain: should not have KEY_BLOCK_SIZE; got %q", plainOrrery)
		}
		// ix_cmt: COMMENT 'hello' present; no KEY_BLOCK_SIZE.
		cmtMy := aLine(mysqlCreate, "ix_cmt")
		cmtOrrery := aLine(orreryCreate, "ix_cmt")
		if !strings.Contains(cmtMy, "COMMENT 'hello'") {
			t.Errorf("oracle ix_cmt: missing COMMENT 'hello'; got %q", cmtMy)
		}
		if !strings.Contains(cmtOrrery, "COMMENT 'hello'") {
			t.Errorf("orrery ix_cmt: missing COMMENT 'hello'; got %q", cmtOrrery)
		}
		// Table-level KEY_BLOCK_SIZE=4 present.
		if !strings.Contains(mysqlCreate, "KEY_BLOCK_SIZE=4") {
			t.Errorf("oracle: missing table-level KEY_BLOCK_SIZE=4; got %q", mysqlCreate)
		}
		if !strings.Contains(orreryCreate, "KEY_BLOCK_SIZE=4") {
			t.Errorf("orrery: missing table-level KEY_BLOCK_SIZE=4; got %q", orreryCreate)
		}
	})

	// -----------------------------------------------------------------
	// 18.15 USING BTREE/HASH only when algorithm explicit
	// -----------------------------------------------------------------
	t.Run("18_15_using_algorithm_explicit", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		ddl := `CREATE TABLE t (
			id INT,
			a  INT,
			b  INT,
			KEY ix_default (a),
			KEY ix_btree (a) USING BTREE,
			KEY ix_hash (b) USING HASH
		) ENGINE=InnoDB`
		runOnBoth(t, mc, c, ddl)

		mysqlCreate := oracleShow(t, mc, "SHOW CREATE TABLE t")
		orreryCreate := c.ShowCreateTable("testdb", "t")

		// ix_default: no USING clause.
		defaultMy := aLine(mysqlCreate, "ix_default")
		defaultOrrery := aLine(orreryCreate, "ix_default")
		if strings.Contains(defaultMy, "USING") {
			t.Errorf("oracle ix_default: should not have USING clause; got %q", defaultMy)
		}
		if strings.Contains(defaultOrrery, "USING") {
			t.Errorf("orrery ix_default: should not have USING clause; got %q", defaultOrrery)
		}
		// ix_btree: has USING clause.
		btreeMy := aLine(mysqlCreate, "ix_btree")
		btreeOrrery := aLine(orreryCreate, "ix_btree")
		if !strings.Contains(btreeMy, "USING") {
			t.Errorf("oracle ix_btree: missing USING clause; got %q", btreeMy)
		}
		if !strings.Contains(btreeOrrery, "USING") {
			t.Errorf("orrery ix_btree: missing USING clause; got %q", btreeOrrery)
		}
		// ix_hash: oracle may or may not render USING (InnoDB rewrites HASH→BTREE).
		// Contract: if oracle renders USING, orrery must too.
		hashMy := aLine(mysqlCreate, "ix_hash")
		hashOrrery := aLine(orreryCreate, "ix_hash")
		if strings.Contains(hashMy, "USING") && !strings.Contains(hashOrrery, "USING") {
			t.Errorf("orrery ix_hash: oracle has USING but orrery does not; oracle=%q orrery=%q", hashMy, hashOrrery)
		}
	})
}

// aLine returns the line from multi-line text s that contains needle. Empty
// string if no match. Used by C18 to grab a single column/index line out of
// SHOW CREATE TABLE output for per-line substring checks.
func aLine(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
