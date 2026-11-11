package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewTabsGatesTabReplacement(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.txt")
	b := filepath.Join(tmp, "b.txt")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("one\ntwo\nthree"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	if !g.PreviewTabs {
		t.Fatal("preview tabs must default to enabled")
	}
	g.OpenFile(a)
	g.OpenFile(b) // replaces the preview tab
	if len(g.tabs) != 1 {
		t.Fatalf("with preview enabled: tabs = %d, want 1", len(g.tabs))
	}
	g.PreviewTabs = false
	g.OpenFile(a) // must NOT replace the open tab anymore
	if len(g.tabs) != 2 {
		t.Fatalf("with preview disabled: tabs = %d, want 2", len(g.tabs))
	}
	g.PreviewTabs = true
	g.OpenFile(filepath.Join(tmp, "c.txt")) // the active tab is not a preview: c appends, nothing is replaced
	if len(g.tabs) != 3 {
		t.Fatalf("after re-enabling: tabs = %d, want 3", len(g.tabs))
	}
}
