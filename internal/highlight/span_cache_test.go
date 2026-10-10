package highlight

import (
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func freshSpans(lines []string, idx int) []Span {
	return New("main.go").HighlightLineAt(lines, idx)
}

func TestHighlightLineAtCacheFollowsEdits(t *testing.T) {
	h := New("main.go")
	lines := []string{"package main", "", "func a() {", "\tx := 1", "}"}
	check := func(step string) {
		t.Helper()
		for i := range lines {
			got := h.HighlightLineAt(lines, i)
			if want := freshSpans(lines, i); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: line %d spans %v, want %v", step, i, got, want)
			}
		}
	}
	check("initial")
	check("cached")

	lines = append([]string(nil), lines...)
	lines[3] = "\tx := \"one\""
	h.ClearCache()
	check("edited line")

	lines = append([]string{"/*"}, lines...)
	h.ClearCache()
	check("comment opened above")
	if styleAt(h.HighlightLineAt(lines, 4), 1) != term.StyleSyntaxComment {
		t.Fatalf("line under an open block comment is not a comment")
	}

	lines = lines[1:]
	h.ClearCache()
	check("undo")
	if styleAt(h.HighlightLineAt(lines, 2), 0) == term.StyleSyntaxComment {
		t.Fatalf("comment state survived removing the opener")
	}
}

func TestHighlightLineAtCacheSharesUnchangedLines(t *testing.T) {
	h := New("main.go")
	lines := []string{"package main", "func a() { return }"}
	first := h.HighlightLineAt(lines, 1)
	again := h.HighlightLineAt(lines, 1)
	if len(first) == 0 || &first[0] != &again[0] {
		t.Fatalf("an unchanged line converted its tokens again")
	}
}
