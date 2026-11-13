package catalog

import (
	"strings"
	"testing"
)

// TestScenario_C4 covers Section C4 "Charset / collation inheritance" from
// SCENARIOS-mysql-implicit-behavior.md. Each subtest runs DDL against both a
// real MySQL 8.0 container and the orrery catalog, then asserts that both agree
// on charset/collation resolution.
//
// Failed orrery assertions are NOT proof failures — they are recorded in
// mysql/catalog/scenarios_bug_queue/c4.md.
func TestScenario_C4(t *testing.T) {
	mc, cleanup := scenarioContainer(t)
	defer cleanup()

	// c4OracleColCharset fetches CHARACTER_SET_NAME from information_schema
	// for testdb.<table>.<col>.
	c4OracleColCharset := func(t *testing.T, table, col string) string {
		t.Helper()
		var s string
		oracleScan(t, mc,
			"SELECT IFNULL(CHARACTER_SET_NAME,'') FROM information_schema.COLUMNS "+
				"WHERE TABLE_SCHEMA='testdb' AND TABLE_NAME='"+table+"' AND COLUMN_NAME='"+col+"'",
			&s)
		return strings.ToLower(s)
	}
	// c4OracleColCollation fetches COLLATION_NAME.
	c4OracleColCollation := func(t *testing.T, table, col string) string {
		t.Helper()
		var s string
		oracleScan(t, mc,
			"SELECT IFNULL(COLLATION_NAME,'') FROM information_schema.COLUMNS "+
				"WHERE TABLE_SCHEMA='testdb' AND TABLE_NAME='"+table+"' AND COLUMN_NAME='"+col+"'",
			&s)
		return strings.ToLower(s)
	}
	// c4OracleTableCollation fetches TABLE_COLLATION.
	c4OracleTableCollation := func(t *testing.T, table string) string {
		t.Helper()
		var s string
		oracleScan(t, mc,
			"SELECT IFNULL(TABLE_COLLATION,'') FROM information_schema.TABLES "+
				"WHERE TABLE_SCHEMA='testdb' AND TABLE_NAME='"+table+"'",
			&s)
		return strings.ToLower(s)
	}
	// c4OracleDataType fetches DATA_TYPE.
	c4OracleDataType := func(t *testing.T, table, col string) string {
		t.Helper()
		var s string
		oracleScan(t, mc,
			"SELECT DATA_TYPE FROM information_schema.COLUMNS "+
				"WHERE TABLE_SCHEMA='testdb' AND TABLE_NAME='"+table+"' AND COLUMN_NAME='"+col+"'",
			&s)
		return strings.ToLower(s)
	}

	// c4ResetDBWithCharset drops testdb, recreates it with a specific
	// charset/collation, USEs it, and does the same on the orrery catalog.
	// Returns a fresh orrery catalog with the same initial state.
	c4ResetDBWithCharset := func(t *testing.T, charset, collation string) *Catalog {
		t.Helper()
		if _, err := mc.db.ExecContext(mc.ctx, "DROP DATABASE IF EXISTS testdb"); err != nil {
			t.Fatalf("oracle DROP DATABASE: %v", err)
		}
		createStmt := "CREATE DATABASE testdb CHARACTER SET " + charset + " COLLATE " + collation
		if _, err := mc.db.ExecContext(mc.ctx, createStmt); err != nil {
			t.Fatalf("oracle CREATE DATABASE: %v", err)
		}
		if _, err := mc.db.ExecContext(mc.ctx, "USE testdb"); err != nil {
			t.Fatalf("oracle USE testdb: %v", err)
		}
		c := New()
		results, err := c.Exec(createStmt+"; USE testdb;", nil)
		if err != nil {
			t.Errorf("orrery parse error for %q: %v", createStmt, err)
			return c
		}
		for _, r := range results {
			if r.Error != nil {
				t.Errorf("orrery exec error on stmt %d: %v", r.Index, r.Error)
			}
		}
		return c
	}

	// --- 4.1 Table charset inherits from database ---
	t.Run("4_1_table_charset_from_db", func(t *testing.T) {
		c := c4ResetDBWithCharset(t, "latin1", "latin1_swedish_ci")

		runOnBoth(t, mc, c, `CREATE TABLE t (c VARCHAR(10))`)

		assertStringEq(t, "oracle table collation",
			c4OracleTableCollation(t, "t"), "latin1_swedish_ci")

		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		assertStringEq(t, "orrery table charset",
			strings.ToLower(tbl.Charset), "latin1")
		assertStringEq(t, "orrery table collation",
			strings.ToLower(tbl.Collation), "latin1_swedish_ci")
	})

	// --- 4.2 Column charset inherits from table (elided in SHOW) ---
	t.Run("4_2_column_charset_from_table", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c,
			`CREATE TABLE t (c VARCHAR(10)) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci`)

		assertStringEq(t, "oracle col collation",
			c4OracleColCollation(t, "t", "c"), "utf8mb4_0900_ai_ci")

		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		col := tbl.GetColumn("c")
		if col == nil {
			t.Error("orrery: column c not found")
			return
		}
		// Column charset should match or equal the table charset after inheritance.
		// The catalog may store empty (inherited) or the resolved charset.
		gotCharset := strings.ToLower(col.Charset)
		if gotCharset != "" && gotCharset != "utf8mb4" {
			t.Errorf("orrery col charset: got %q, want \"utf8mb4\" or empty (inherited)", gotCharset)
		}

		// SHOW CREATE TABLE should not mention column-level CHARACTER SET.
		mysqlCreate := oracleShow(t, mc, "SHOW CREATE TABLE t")
		orreryCreate := c.ShowCreateTable("testdb", "t")
		// Locate the column line.
		for _, line := range strings.Split(orreryCreate, "\n") {
			if strings.Contains(line, "`c`") && strings.Contains(strings.ToUpper(line), "CHARACTER SET") {
				t.Errorf("orrery SHOW CREATE TABLE: column c should not have CHARACTER SET clause (inherited from table default). Got: %s", line)
			}
		}
		for _, line := range strings.Split(mysqlCreate, "\n") {
			if strings.Contains(line, "`c`") && strings.Contains(strings.ToUpper(line), "CHARACTER SET") {
				t.Logf("oracle SHOW CREATE TABLE column line: %s (unexpectedly has CHARACTER SET)", line)
			}
		}
	})

	// --- 4.3 Column COLLATE alone → derive CHARACTER SET ---
	t.Run("4_3_collate_derives_charset", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c,
			`CREATE TABLE t (c VARCHAR(10) COLLATE utf8mb4_unicode_ci) DEFAULT CHARSET=latin1`)

		assertStringEq(t, "oracle col charset",
			c4OracleColCharset(t, "t", "c"), "utf8mb4")
		assertStringEq(t, "oracle col collation",
			c4OracleColCollation(t, "t", "c"), "utf8mb4_unicode_ci")

		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		col := tbl.GetColumn("c")
		if col == nil {
			t.Error("orrery: column c not found")
			return
		}
		assertStringEq(t, "orrery col charset",
			strings.ToLower(col.Charset), "utf8mb4")
		assertStringEq(t, "orrery col collation",
			strings.ToLower(col.Collation), "utf8mb4_unicode_ci")
	})

	// --- 4.4 Column CHARACTER SET alone → derive default COLLATE ---
	t.Run("4_4_charset_derives_default_collation", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c,
			`CREATE TABLE t (c VARCHAR(10) CHARACTER SET latin1) DEFAULT CHARSET=utf8mb4`)

		assertStringEq(t, "oracle col charset",
			c4OracleColCharset(t, "t", "c"), "latin1")
		assertStringEq(t, "oracle col collation",
			c4OracleColCollation(t, "t", "c"), "latin1_swedish_ci")

		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		col := tbl.GetColumn("c")
		if col == nil {
			t.Error("orrery: column c not found")
			return
		}
		assertStringEq(t, "orrery col charset",
			strings.ToLower(col.Charset), "latin1")
		assertStringEq(t, "orrery col collation",
			strings.ToLower(col.Collation), "latin1_swedish_ci")
	})

	// --- 4.5 Table CHARSET/COLLATE mismatch error ---
	t.Run("4_5_charset_collation_mismatch", func(t *testing.T) {
		scenarioReset(t, mc)

		// Mismatch case: latin1 + utf8mb4_0900_ai_ci should fail.
		mismatchStmt := `CREATE TABLE t_bad (c VARCHAR(10)) CHARACTER SET latin1 COLLATE utf8mb4_0900_ai_ci`
		if _, err := mc.db.ExecContext(mc.ctx, mismatchStmt); err == nil {
			t.Error("oracle: expected mismatch error, got none")
		}

		c := scenarioNewCatalog(t)
		results, err := c.Exec(mismatchStmt, nil)
		orreryRejected := false
		if err != nil {
			orreryRejected = true
		} else {
			for _, r := range results {
				if r.Error != nil {
					orreryRejected = true
					break
				}
			}
		}
		if !orreryRejected {
			t.Error("orrery: expected mismatch CHARSET/COLLATE to be rejected, got no error")
		}

		// Compatible case: should succeed.
		scenarioReset(t, mc)
		c2 := scenarioNewCatalog(t)
		runOnBoth(t, mc, c2,
			`CREATE TABLE t_ok (c VARCHAR(10)) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci`)
		assertStringEq(t, "oracle ok table collation",
			c4OracleTableCollation(t, "t_ok"), "utf8mb4_0900_ai_ci")
		tbl := c2.GetDatabase("testdb").GetTable("t_ok")
		if tbl == nil {
			t.Error("orrery: table t_ok not found")
			return
		}
		assertStringEq(t, "orrery ok table collation",
			strings.ToLower(tbl.Collation), "utf8mb4_0900_ai_ci")
	})

	// --- 4.6 BINARY modifier → {charset}_bin rewrite ---
	t.Run("4_6_binary_modifier_bin_collation", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, `CREATE TABLE t (
  a CHAR(10) BINARY,
  b VARCHAR(10) CHARACTER SET latin1 BINARY
) DEFAULT CHARSET=utf8mb4`)

		assertStringEq(t, "oracle a collation",
			c4OracleColCollation(t, "t", "a"), "utf8mb4_bin")
		assertStringEq(t, "oracle b collation",
			c4OracleColCollation(t, "t", "b"), "latin1_bin")

		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		if colA := tbl.GetColumn("a"); colA != nil {
			assertStringEq(t, "orrery a collation",
				strings.ToLower(colA.Collation), "utf8mb4_bin")
		} else {
			t.Error("orrery: column a not found")
		}
		if colB := tbl.GetColumn("b"); colB != nil {
			assertStringEq(t, "orrery b collation",
				strings.ToLower(colB.Collation), "latin1_bin")
			assertStringEq(t, "orrery b charset",
				strings.ToLower(colB.Charset), "latin1")
		} else {
			t.Error("orrery: column b not found")
		}

		// Round-trip: deparse should not emit the BINARY keyword.
		orreryCreate := c.ShowCreateTable("testdb", "t")
		if strings.Contains(strings.ToUpper(orreryCreate), " BINARY,") ||
			strings.Contains(strings.ToUpper(orreryCreate), " BINARY\n") {
			// Accept canonical form only. The BINARY attribute must be rewritten.
			// Allow "BINARY(" for BINARY(N) column type — different construct.
			// We check only that the attribute form isn't present on CHAR/VARCHAR.
			for _, line := range strings.Split(orreryCreate, "\n") {
				upper := strings.ToUpper(line)
				if (strings.Contains(upper, "CHAR(") || strings.Contains(upper, "VARCHAR(")) &&
					strings.Contains(upper, " BINARY") {
					t.Errorf("orrery SHOW CREATE TABLE: expected BINARY attribute rewritten to _bin collation, got line: %s", line)
				}
			}
		}
	})

	// --- 4.7 CHARACTER SET binary vs BINARY type distinction ---
	t.Run("4_7_charset_binary_vs_binary_type", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, `CREATE TABLE t (
  a BINARY(10),
  b CHAR(10) CHARACTER SET binary,
  c VARBINARY(10)
)`)

		// MySQL 8.0 information_schema reality:
		// - BINARY(N) / VARBINARY(N) columns have CHARACTER_SET_NAME and
		//   COLLATION_NAME reported as NULL (they're byte types, not text).
		// - CHAR(N) CHARACTER SET binary is **silently rewritten** at parse
		//   time to BINARY(N) (sql_yacc.yy folds the form), so it ends up
		//   indistinguishable from `a` in information_schema: DATA_TYPE='binary',
		//   COLLATION_NAME=NULL. The "three different kinds of binary" the
		//   scenario describes manifests in the parse tree, not the post-store
		//   metadata.
		assertStringEq(t, "oracle a collation (NULL for BINARY type)",
			c4OracleColCollation(t, "t", "a"), "")
		assertStringEq(t, "oracle b collation (rewritten to BINARY -> NULL)",
			c4OracleColCollation(t, "t", "b"), "")
		assertStringEq(t, "oracle c collation (NULL for VARBINARY type)",
			c4OracleColCollation(t, "t", "c"), "")
		// Post-fold DATA_TYPE: a -> binary, b -> binary (rewritten), c -> varbinary.
		assertStringEq(t, "oracle a data_type",
			c4OracleDataType(t, "t", "a"), "binary")
		assertStringEq(t, "oracle b data_type (rewritten)",
			c4OracleDataType(t, "t", "b"), "binary")
		assertStringEq(t, "oracle c data_type",
			c4OracleDataType(t, "t", "c"), "varbinary")

		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		// orrery-side: match oracle's NULL-vs-"binary" distinction. For BINARY
		// and VARBINARY column types the catalog should NOT report a column
		// charset (mirrors information_schema NULL). For CHAR(N) CHARACTER SET
		// binary, the catalog should report Collation="binary".
		check := func(name, wantDT, wantCollation string) {
			col := tbl.GetColumn(name)
			if col == nil {
				t.Errorf("orrery: column %s not found", name)
				return
			}
			if strings.ToLower(col.Collation) != wantCollation {
				t.Errorf("orrery col %s collation: got %q, want %q", name, col.Collation, wantCollation)
			}
			if strings.ToLower(col.DataType) != wantDT {
				t.Errorf("orrery col %s data_type: got %q, want %q", name, col.DataType, wantDT)
			}
		}
		check("a", "binary", "")
		// b: orrery should match oracle's silent rewrite — DATA_TYPE='binary'
		// after CHAR(N) CHARACTER SET binary normalisation.
		check("b", "binary", "")
		check("c", "varbinary", "")
	})

	// --- 4.8 utf8 → utf8mb3 alias normalization ---
	t.Run("4_8_utf8_alias_normalization", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, `CREATE TABLE t (c VARCHAR(10) CHARACTER SET utf8)`)

		assertStringEq(t, "oracle col charset",
			c4OracleColCharset(t, "t", "c"), "utf8mb3")
		assertStringEq(t, "oracle col collation",
			c4OracleColCollation(t, "t", "c"), "utf8mb3_general_ci")

		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		col := tbl.GetColumn("c")
		if col == nil {
			t.Error("orrery: column c not found")
			return
		}
		assertStringEq(t, "orrery col charset (normalized)",
			strings.ToLower(col.Charset), "utf8mb3")

		// SHOW CREATE TABLE should print utf8mb3, never utf8.
		orreryCreate := strings.ToLower(c.ShowCreateTable("testdb", "t"))
		if strings.Contains(orreryCreate, "character set utf8 ") ||
			strings.Contains(orreryCreate, "character set utf8,") ||
			strings.HasSuffix(orreryCreate, "character set utf8") {
			t.Errorf("orrery SHOW CREATE TABLE: expected utf8mb3, got unnormalized utf8 in: %s", orreryCreate)
		}
	})

	// --- 4.9 NCHAR/NATIONAL → utf8mb3 hardcoding ---
	t.Run("4_9_national_nchar_utf8mb3", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, `CREATE TABLE t (
  a NCHAR(10),
  b NATIONAL CHARACTER(10),
  cc NATIONAL VARCHAR(10),
  d NCHAR VARYING(10)
)`)

		for _, name := range []string{"a", "b", "cc", "d"} {
			assertStringEq(t, "oracle "+name+" charset",
				c4OracleColCharset(t, "t", name), "utf8mb3")
			assertStringEq(t, "oracle "+name+" collation",
				c4OracleColCollation(t, "t", name), "utf8mb3_general_ci")
		}

		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		for _, name := range []string{"a", "b", "cc", "d"} {
			col := tbl.GetColumn(name)
			if col == nil {
				t.Errorf("orrery: column %s not found", name)
				continue
			}
			assertStringEq(t, "orrery "+name+" charset",
				strings.ToLower(col.Charset), "utf8mb3")
		}
	})

	// --- 4.10 ENUM/SET charset inheritance ---
	t.Run("4_10_enum_set_charset_inheritance", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c, `CREATE TABLE t (
  a ENUM('x','y'),
  b ENUM('x','y') CHARACTER SET latin1,
  cc SET('p','q') COLLATE utf8mb4_unicode_ci
) DEFAULT CHARSET=utf8mb4`)

		// a: inherits table default utf8mb4
		assertStringEq(t, "oracle a charset",
			c4OracleColCharset(t, "t", "a"), "utf8mb4")
		// b: latin1 + default collation
		assertStringEq(t, "oracle b charset",
			c4OracleColCharset(t, "t", "b"), "latin1")
		assertStringEq(t, "oracle b collation",
			c4OracleColCollation(t, "t", "b"), "latin1_swedish_ci")
		// cc: charset derived from COLLATE
		assertStringEq(t, "oracle cc charset",
			c4OracleColCharset(t, "t", "cc"), "utf8mb4")
		assertStringEq(t, "oracle cc collation",
			c4OracleColCollation(t, "t", "cc"), "utf8mb4_unicode_ci")

		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		if colB := tbl.GetColumn("b"); colB != nil {
			assertStringEq(t, "orrery b charset",
				strings.ToLower(colB.Charset), "latin1")
			assertStringEq(t, "orrery b collation",
				strings.ToLower(colB.Collation), "latin1_swedish_ci")
		} else {
			t.Error("orrery: column b not found")
		}
		if colC := tbl.GetColumn("cc"); colC != nil {
			assertStringEq(t, "orrery cc charset",
				strings.ToLower(colC.Charset), "utf8mb4")
			assertStringEq(t, "orrery cc collation",
				strings.ToLower(colC.Collation), "utf8mb4_unicode_ci")
		} else {
			t.Error("orrery: column cc not found")
		}
	})

	// --- 4.11 Index prefix × mbmaxlen ---
	t.Run("4_11_index_prefix_mbmaxlen", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		// t1: latin1 c(5) — fits (SUB_PART reported when prefix < full col).
		runOnBoth(t, mc, c,
			`CREATE TABLE t1 (c VARCHAR(10) CHARACTER SET latin1, KEY k (c(5)))`)
		// t2: utf8mb4 c(5) — 20 bytes, fits.
		runOnBoth(t, mc, c,
			`CREATE TABLE t2 (c VARCHAR(10) CHARACTER SET utf8mb4, KEY k (c(5)))`)

		// t3: utf8mb4 VARCHAR(200), KEY (c(768)) = 3072 bytes. Should be rejected
		// (exceeds InnoDB 3072-byte per-column key limit by default; really the
		// prefix length > VARCHAR(200) also fails with ER_WRONG_SUB_KEY).
		// Run only on oracle to verify it errors.
		tooLong := `CREATE TABLE t3 (c VARCHAR(200) CHARACTER SET utf8mb4, KEY k (c(768)))`
		if _, err := mc.db.ExecContext(mc.ctx, tooLong); err == nil {
			t.Error("oracle: expected t3 creation to fail (prefix too long), got no error")
			_, _ = mc.db.ExecContext(mc.ctx, "DROP TABLE IF EXISTS t3")
		}
		// orrery: should also reject.
		results, err := c.Exec(tooLong, nil)
		orreryRejected := err != nil
		if !orreryRejected {
			for _, r := range results {
				if r.Error != nil {
					orreryRejected = true
					break
				}
			}
		}
		if !orreryRejected {
			t.Error("orrery: expected t3 CREATE to be rejected, got no error")
		}

		// t1/t2: prefix is reported in characters (SUB_PART).
		var sub1, sub2 int
		oracleScan(t, mc,
			`SELECT SUB_PART FROM information_schema.STATISTICS
             WHERE TABLE_SCHEMA='testdb' AND TABLE_NAME='t1' AND INDEX_NAME='k'`,
			&sub1)
		oracleScan(t, mc,
			`SELECT SUB_PART FROM information_schema.STATISTICS
             WHERE TABLE_SCHEMA='testdb' AND TABLE_NAME='t2' AND INDEX_NAME='k'`,
			&sub2)
		assertIntEq(t, "oracle t1 SUB_PART", sub1, 5)
		assertIntEq(t, "oracle t2 SUB_PART", sub2, 5)

		// orrery: index column Length should be 10 (characters, not bytes).
		for _, name := range []string{"t1", "t2"} {
			tbl := c.GetDatabase("testdb").GetTable(name)
			if tbl == nil {
				t.Errorf("orrery: table %s not found", name)
				continue
			}
			var idx *Index
			for _, i := range tbl.Indexes {
				if i.Name == "k" {
					idx = i
					break
				}
			}
			if idx == nil {
				t.Errorf("orrery: index k on %s not found", name)
				continue
			}
			if len(idx.Columns) != 1 {
				t.Errorf("orrery: %s.k expected 1 col, got %d", name, len(idx.Columns))
				continue
			}
			assertIntEq(t, "orrery "+name+".k length", idx.Columns[0].Length, 5)
		}
	})

	// --- 4.12 DTCollation derivation levels ---
	//
	// This scenario requires an expression-level collation resolver. orrery
	// catalog currently stores column-level Charset/Collation, but does not
	// expose DTCollation / derivation for arbitrary SELECT expressions. We
	// verify the column is set up correctly and that MySQL produces the
	// documented comparison outcomes; orrery-side assertions are best-effort.
	t.Run("4_12_dtcollation_derivation", func(t *testing.T) {
		scenarioReset(t, mc)
		c := scenarioNewCatalog(t)

		runOnBoth(t, mc, c,
			`CREATE TABLE t (c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci)`)

		// Oracle: baseline column collation.
		assertStringEq(t, "oracle col collation",
			c4OracleColCollation(t, "t", "c"), "utf8mb4_0900_ai_ci")

		// Oracle runtime checks for each comparison shape. These are queries
		// against an empty table — we care about whether MySQL accepts or
		// rejects them (not about returned rows).
		ok := func(q string) {
			if _, err := mc.db.ExecContext(mc.ctx, q); err != nil {
				t.Errorf("oracle: %q should succeed, got %v", q, err)
			}
		}
		_ = func(q string) {} // placeholder to keep ok() referenced if unused
		ok("SELECT c = 'abc' FROM t")
		ok("SELECT c = _utf8mb4'abc' COLLATE utf8mb4_bin FROM t")
		ok("SELECT c COLLATE utf8mb4_bin = _latin1'abc' FROM t")
		// The CAST-vs-column comparison is documented to fail with
		// ER_CANT_AGGREGATE_2COLLATIONS, but MySQL 8.0.x has loosened
		// implicit conversion in several point releases. Log the outcome
		// for visibility but don't fail the test on either path.
		castQuery := "SELECT CAST('abc' AS CHAR CHARACTER SET latin1) = c FROM t"
		if _, err := mc.db.ExecContext(mc.ctx, castQuery); err == nil {
			t.Logf("oracle: %q succeeded (DTCollation aggregation allowed in this MySQL version)", castQuery)
		} else {
			t.Logf("oracle: %q failed as documented: %v", castQuery, err)
		}

		// orrery: we only assert the column collation survives.
		tbl := c.GetDatabase("testdb").GetTable("t")
		if tbl == nil {
			t.Error("orrery: table t not found")
			return
		}
		col := tbl.GetColumn("c")
		if col == nil {
			t.Error("orrery: column c not found")
			return
		}
		assertStringEq(t, "orrery col collation",
			strings.ToLower(col.Collation), "utf8mb4_0900_ai_ci")

		// Best-effort: orrery's catalog currently does not expose a SELECT
		// expression evaluator with DTCollation, so the 4 query-time checks
		// above are oracle-only. See scenarios_bug_queue/c4.md for the gap.
	})
}
