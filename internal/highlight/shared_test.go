package highlight

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

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

func TestSharedTokensConcurrentSides(t *testing.T) {
	lines := make([]string, 0, 600)
	for i := range 100 {
		lines = append(lines, "func f() {", "\t/* note", "\t*/", fmt.Sprintf("\tx := %d", i), "}", "")
	}
	a, b := New("main.go"), New("main.go")
	a.ShareTokens(nil)
	b.ShareTokens(a)
	var wg sync.WaitGroup
	for _, h := range []*Highlighter{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.HighlightLineAt(lines, len(lines)-1)
		}()
	}
	wg.Wait()
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
