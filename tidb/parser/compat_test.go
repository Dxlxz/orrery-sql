package parser

import (
	"path/filepath"
	"testing"

	"github.com/Dxlxz/orrery-sql/enginetest"
	"github.com/Dxlxz/orrery-sql/enginetest/compat"
	"github.com/Dxlxz/orrery-sql/tidb/internal/testcontainer"
)

// TestCompatibility holds orrery to testdata/compat.tsv: a statement for each
// alternative of TiDB's parser.y and its near-misses, with orrery's verdict and
// TiDB's. It reads the file and starts no server. OMNI_RECORD=1 judges the
// rows that have no verdict yet on the pinned TiDB.
func TestCompatibility(t *testing.T) {
	compat.Check(t, compat.Suite{
		Path:    filepath.Join("testdata", "compat.tsv"),
		Pins:    filepath.Join("..", "testdata", "pins.tsv"),
		Grammar: filepath.Join("..", "testdata", "parser.y"),
		Tokens:  filepath.Join("..", "testdata", "tokens.tsv"),
		Orrery:    orreryVerdict,
		Source:  compat.SourceEngine,
		Judge:   compatJudge,
	})
}

func orreryVerdict(sql string) string {
	if _, err := Parse(sql); err != nil {
		return compat.Reject
	}
	return compat.Accept
}

// compatJudge asks the package's TiDB server, through PREPARE, whether it
// accepts each statement's syntax. It fails t unless the server is the pinned
// image and release.
func compatJudge(t testing.TB, pin enginetest.Pin) func(string) (string, error) {
	t.Helper()
	if pin.Image != tidbImage || pin.Tag != tidbCorpusVersion {
		t.Fatalf("pins.tsv pins %s at %s, but this package starts %s at %s", pin.Image, pin.Tag, tidbImage, tidbCorpusVersion)
	}
	tc := startTiDB(t)
	assertTiDBVersion(t, tc)
	if err := testcontainer.CheckDigest(tc.ctx, tc.container, pin.Digest); err != nil {
		t.Fatalf("%v", err)
	}
	previous := "<start of compat.tsv>"
	return func(sql string) (string, error) {
		accepts, _, err := tc.syntaxVerdict(t, sql, previous)
		previous = sql
		if err != nil {
			return "", err
		}
		if accepts {
			return compat.Accept, nil
		}
		return compat.Reject, nil
	}
}
