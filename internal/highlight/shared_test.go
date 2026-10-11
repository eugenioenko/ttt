package highlight

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestSharedTokensDoNotLeakAcrossSides(t *testing.T) {
	old := []string{"package main", "", "func a() {", "\tx := 1", "\ty := 2", "}", "", "func b() {}"}
	updated := []string{"package main", "", "/* removed", "func a() {", "\tx := 1", "\ty := 2", "}", "*/", "", "func b() {}"}

	oldHL, newHL := New("main.go"), New("main.go")
	oldHL.ShareTokens(nil)
	newHL.ShareTokens(oldHL)

	for i := range old {
		oldHL.HighlightLineAt(old, i)
	}
	for i := range updated {
		got := newHL.HighlightLineAt(updated, i)
		if want := freshSpans(updated, i); !reflect.DeepEqual(got, want) {
			t.Fatalf("new side line %d spans %v, want %v", i, got, want)
		}
	}
	if styleAt(newHL.HighlightLineAt(updated, 4), 1) != term.StyleSyntaxComment {
		t.Fatalf("line inside the comment opened on the new side is not a comment")
	}
	for i := range old {
		got := oldHL.HighlightLineAt(old, i)
		if want := freshSpans(old, i); !reflect.DeepEqual(got, want) {
			t.Fatalf("old side line %d spans %v, want %v", i, got, want)
		}
	}
	if styleAt(oldHL.HighlightLineAt(old, 3), 1) == term.StyleSyntaxComment {
		t.Fatalf("the new side's block comment leaked into the old side")
	}
}

func TestSharedTokensFollowEdits(t *testing.T) {
	h, peer := New("main.go"), New("main.go")
	peer.ShareTokens(nil)
	h.ShareTokens(peer)
	lines := []string{"package main", "", "func a() {", "\tx := 1", "}"}
	check := func(step string) {
		t.Helper()
		for i := range lines {
			if got, want := h.HighlightLineAt(lines, i), freshSpans(lines, i); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: line %d spans %v, want %v", step, i, got, want)
			}
		}
	}
	check("initial")
	lines = append([]string{"/*"}, lines...)
	h.ClearCache()
	check("comment opened above")
	lines = lines[1:]
	h.ClearCache()
	check("comment removed")
}

func TestSharedTokensReuseTheOtherSidesWork(t *testing.T) {
	lines := make([]string, 0, 600)
	for i := range 100 {
		lines = append(lines, "func f() {", "\t/* note", "\t*/", fmt.Sprintf("\tx := %d", i), "}", "")
	}
	a, b := New("main.go"), New("main.go")
	a.ShareTokens(nil)
	b.ShareTokens(a)
	a.HighlightLineAt(lines, len(lines)-1)
	cached := len(a.doc.(*sharedDoc).cache.m)
	b.HighlightLineAt(lines, len(lines)-1)
	if got := len(b.doc.(*sharedDoc).cache.m); got != cached {
		t.Fatalf("the second side added %d cache entries for identical lines, want 0", got-cached)
	}
	for _, i := range []int{2, 3, len(lines) - 3} {
		want := freshSpans(lines, i)
		if got := a.HighlightLineAt(lines, i); !reflect.DeepEqual(got, want) {
			t.Fatalf("side a line %d spans %v, want %v", i, got, want)
		}
		if got := b.HighlightLineAt(lines, i); !reflect.DeepEqual(got, want) {
			t.Fatalf("side b line %d spans %v, want %v", i, got, want)
		}
	}
}

func TestSidesShareSingleLineHighlighting(t *testing.T) {
	a, b := New("main.go"), New("main.go")
	b.ShareSingleLines(a)
	if !b.SharesSingleLinesWith(a) {
		t.Fatal("sides do not share the single-line cache")
	}
	line := "x := 1 // note"
	if got, want := b.HighlightLine(line), a.HighlightLine(line); !reflect.DeepEqual(got, want) {
		t.Fatalf("shared single-line spans %v, want %v", got, want)
	}
	other := New("main.py")
	other.ShareSingleLines(a)
	if other.SharesSingleLinesWith(a) {
		t.Fatal("highlighters for different grammars shared a single-line cache")
	}
}

func TestSharedTokensRetryATimeStoppedState(t *testing.T) {
	lines := commentedFile(50)
	h := New("main.go")
	h.ShareTokens(nil)
	doc := h.doc.(*sharedDoc)
	doc.options.TimeLimit = time.Nanosecond
	h.HighlightLineAt(lines, 30)

	doc.options.TimeLimit = time.Minute
	if got := styleAt(h.HighlightLineAt(lines, 30), 1); got != term.StyleSyntaxComment {
		t.Fatalf("line inside the comment has style %v after the time limit was lifted, want a comment", got)
	}
}

func TestSharedTokensKeepATimeStoppedStateAfterRetries(t *testing.T) {
	lines := commentedFile(50)
	h := New("main.go")
	h.ShareTokens(nil)
	doc := h.doc.(*sharedDoc)
	doc.options.TimeLimit = time.Nanosecond
	h.HighlightLineAt(lines, 30)
	for range maxTimeLimitRetries {
		doc.StateAt(31)
	}
	if got := doc.MaterializedLines(); got < 31 {
		t.Fatalf("materialized lines = %d after the retries ran out, want at least 31", got)
	}
}
