package analysis

import (
	"os"
	"testing"

	"github.com/Dxlxz/orrery-sql/googlesql/internal/testcontainer"
)

func TestMain(m *testing.M) {
	code := m.Run()
	testcontainer.RemoveHarness()
	os.Exit(code)
}
