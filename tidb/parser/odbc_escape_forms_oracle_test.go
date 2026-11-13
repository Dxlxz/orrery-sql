package parser

import "testing"

// TestODBCEscapeFormsTiDBOracle asserts orrery agrees with a live TiDB v8.5.5 on
// odbcEscapeCases (container test). The verdict comes from PREPARE, which parses
// the statement without running it.
func TestODBCEscapeFormsTiDBOracle(t *testing.T) {
	tc := startTiDB(t)
	assertTiDBVersion(t, tc)
	for i, c := range odbcEscapeCases {
		t.Run(c.sql, func(t *testing.T) {
			previous := "<start of cases>"
			if i > 0 {
				previous = odbcEscapeCases[i-1].sql
			}
			accepts, _, err := tc.syntaxVerdict(t, c.sql, previous)
			if err != nil {
				t.Fatalf("no verdict: %v", err)
			}
			if accepts != c.accept {
				t.Fatalf("TiDB %s accepts=%v, want %v for %q", tidbCorpusVersion, accepts, c.accept, c.sql)
			}
			orreryAccepts, perr := parsesAsOneStatement(c.sql)
			if orreryAccepts != c.accept {
				t.Errorf("DIVERGENCE orrery accepts=%v, TiDB %s accepts=%v for %q: %v",
					orreryAccepts, tidbCorpusVersion, c.accept, c.sql, perr)
			}
		})
	}
}
