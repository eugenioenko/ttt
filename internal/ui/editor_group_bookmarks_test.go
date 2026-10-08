package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func openBookmarkFiles(t *testing.T) (g *EditorGroupWidget, a, b string) {
	t.Helper()
	tmp := t.TempDir()
	a = filepath.Join(tmp, "a.txt")
	b = filepath.Join(tmp, "b.txt")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("one\ntwo\nthree"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	g = NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(a)
	g.OpenFile(a)
	g.Editor.SetBookmark(1, '*', term.StyleDefault)
	g.OpenFile(b)
	return g, a, b
}

func TestBookmarksFollowInactiveTabRename(t *testing.T) {
	g, a, _ := openBookmarkFiles(t)
	renamed := filepath.Join(filepath.Dir(a), "renamed.txt")

	g.RenamePath(a, renamed)
	g.OpenFile(renamed)

	if _, ok := g.Editor.GetBookmark(1); !ok || g.ActiveFilePath() != renamed {
		t.Fatalf("bookmark lost after renaming an inactive tab (active %q)", g.ActiveFilePath())
	}
}

func TestBookmarksFollowActiveTabRename(t *testing.T) {
	g, a, b := openBookmarkFiles(t)
	g.OpenFile(a)
	renamed := filepath.Join(filepath.Dir(a), "renamed.txt")

	g.RenamePath(a, renamed)
	g.OpenFile(b)
	g.OpenFile(renamed)

	if _, ok := g.Editor.GetBookmark(1); !ok {
		t.Fatal("bookmark lost after renaming the active tab and switching away")
	}
}

func TestBookmarksFollowSaveAs(t *testing.T) {
	g, a, b := openBookmarkFiles(t)
	g.OpenFile(a)
	saved := filepath.Join(filepath.Dir(a), "saved.txt")

	if !g.SaveAs(saved) {
		t.Fatal("SaveAs failed")
	}
	g.OpenFile(b)
	g.OpenFile(saved)

	if _, ok := g.Editor.GetBookmark(1); !ok {
		t.Fatal("bookmark lost after Save As and switching away")
	}
}

func TestBookmarksFollowRenameOfReplacedPreviewTab(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.txt")
	b := filepath.Join(tmp, "b.txt")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("one\ntwo"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(a)
	g.Editor.SetBookmark(1, '*', term.StyleDefault)
	g.OpenFile(b)
	renamed := filepath.Join(tmp, "renamed.txt")

	g.RenamePath(a, renamed)
	g.OpenFile(renamed)

	if _, ok := g.Editor.GetBookmark(1); !ok {
		t.Fatal("bookmark lost after renaming a file whose preview tab was replaced")
	}
}

func TestBookmarksFollowFolderRename(t *testing.T) {
	g, a, b := openBookmarkFiles(t)
	dir := filepath.Dir(a)
	moved := dir + "-moved"

	g.RenamePath(dir, moved)
	g.OpenFile(filepath.Join(moved, filepath.Base(b)))
	g.OpenFile(filepath.Join(moved, filepath.Base(a)))

	if _, ok := g.Editor.GetBookmark(1); !ok {
		t.Fatal("bookmark lost after renaming the parent folder")
	}
}
