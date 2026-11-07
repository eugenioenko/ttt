package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
)

func TestOpenFileTargetsPreservesFailedContentTab(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.png")
	text := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(broken, []byte("\x89PNG\r\n\x1a\ninvalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(text, []byte("one\ntwo\nthree"), 0600); err != nil {
		t.Fatal(err)
	}
	g := ui.NewEditorGroupWidget(nil, 4, false, "extended")
	targets := []FileTarget{{Path: broken}, {Path: text, Line: 3, Col: 2}}
	opened, err := openFileTargets(g, targets)
	if err == nil || !strings.Contains(err.Error(), broken) {
		t.Fatalf("openFileTargets error = %v, want failed path %s", err, broken)
	}
	if !slices.Equal(opened, targets[1:]) {
		t.Errorf("opened targets = %v, want %v", opened, targets[1:])
	}
	if g.TabCount() != 2 {
		t.Fatalf("tab count = %d, want broken image and text file", g.TabCount())
	}
	for i, want := range []string{broken, text} {
		if path, _ := g.TabInfo(i); path != want {
			t.Errorf("tab %d path = %s, want %s", i, path, want)
		}
	}
	if cur := g.Editor.Cursor; cur.Line != 2 || cur.Col != 1 {
		t.Errorf("cursor = %v, want line 2 col 1", cur)
	}
	if !g.SwitchToTabByPath(broken) {
		t.Fatal("cannot switch back to broken image")
	}
	content := g.TabContent(g.ActiveTabIndex())
	image, ok := content.(*ui.ImageViewWidget)
	if !ok || image.Err == "" {
		t.Fatalf("content = %#v, want image error", content)
	}
}

func TestOpenFileTargetsDoesNotCommitUnrelatedTabOnFailure(t *testing.T) {
	dir := t.TempDir()
	preview := filepath.Join(dir, "preview.txt")
	next := filepath.Join(dir, "next.txt")
	g := ui.NewEditorGroupWidget(nil, 4, false, "extended")
	if err := g.OpenFile(preview); err != nil {
		t.Fatal(err)
	}
	opened, err := openFileTargets(g, []FileTarget{{Path: dir}})
	if err == nil || len(opened) != 0 {
		t.Fatalf("openFileTargets = %v, %v, want failed directory", opened, err)
	}
	if err := g.OpenFile(next); err != nil {
		t.Fatal(err)
	}
	if paths := g.OpenFilePaths(); !slices.Equal(paths, []string{next}) {
		t.Errorf("open paths = %v, want preview replaced by %s", paths, next)
	}
}
