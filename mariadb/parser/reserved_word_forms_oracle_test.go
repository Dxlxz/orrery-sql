package parser

import (
	"sort"
	"strings"
	"testing"
)

// TestReservedWordFormsOracle asserts orrery agrees with a live MariaDB 11.8.8 on reservedWordCases
// (container test). The verdict comes from PREPARE, which parses the statement without running it.
func TestReservedWordFormsOracle(t *testing.T) {
	o := startMariaDB(t)
	for _, c := range reservedWordCases {
		t.Run(c.sql, func(t *testing.T) {
			if got := o.prepareAccepts(t, c.sql); got != c.accept {
				t.Fatalf("MariaDB accepts=%v, want %v for %q", got, c.accept, c.sql)
			}
			orrery, err := parsesAsOneStatement(c.sql)
			if orrery != c.accept {
				t.Errorf("DIVERGENCE orrery accepts=%v, MariaDB accepts=%v for %q: %v", orrery, c.accept, c.sql, err)
			}
		})
	}
}

// TestReservedWordSweepOracle holds every registered keyword to a live MariaDB 11.8.8 in each name
// position, which is what keeps keywordCategories MariaDB's and not MySQL's. A word added to the
// lexer with the wrong category fails here.
func TestReservedWordSweepOracle(t *testing.T) {
	o := startMariaDB(t)
	words := make([]string, 0, len(keywords))
	for word := range keywords {
		words = append(words, word)
	}
	sort.Strings(words)
	for _, position := range reservedWordPositions {
		for _, word := range words {
			sql := strings.ReplaceAll(position, "%s", word)
			server := o.prepareAccepts(t, sql)
			orrery, err := parsesAsOneStatement(sql)
			if reservedWordSweepGap[sql] {
				if server == orrery {
					t.Errorf("%q now agrees: drop it from reservedWordSweepGap", sql)
				}
				continue
			}
			if server != orrery {
				t.Errorf("DIVERGENCE orrery accepts=%v, MariaDB accepts=%v for %q: %v", orrery, server, sql, err)
			}
		}
	}
}
