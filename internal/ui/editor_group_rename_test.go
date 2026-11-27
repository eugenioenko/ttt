package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

// newRenameGroup returns a group with one tab open on a real file at path.
func newRenameGroup(t *testing.T, path, contents string) *EditorGroupWidget {
	t.Helper()
	requireNoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.SyntaxHighlight = true
	requireNoError(t, g.OpenFile(path))
	return g
}

func TestRenamePathUpdatesOpenTab(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "old.txt")
	g := newRenameGroup(t, old, "hello\nworld")

	newPath := filepath.Join(tmp, "new.txt")
	if !g.RenamePath(old, newPath) {
		t.Fatal("expected RenamePath to report a change")
	}
	if got := g.ActiveFilePath(); got != newPath {
		t.Fatalf("tab path: got %q, want %q", got, newPath)
	}
}

func TestRenamePathUpdatesFileViewerTabAndContent(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "image.bin")
	if err := os.WriteFile(old, []byte{0, 1, 2}, 0644); err != nil {
		t.Fatal(err)
	}
	viewer := NewBinaryFileWidget(old)
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.openContentFileTab(old, viewer)

	newPath := filepath.Join(tmp, "renamed.bin")
	if err := os.Rename(old, newPath); err != nil {
		t.Fatal(err)
	}
	if !g.RenamePath(old, newPath) {
		t.Fatal("expected RenamePath to report a content-tab change")
	}
	if got := g.tabs[g.active].IconPath; got != newPath {
		t.Fatalf("content-tab icon path: got %q, want %q", got, newPath)
	}
	if got := g.tabs[g.active].FilePath; got != newPath {
		t.Fatalf("content-tab path: got %q, want %q", got, newPath)
	}
	if got := viewer.FilePath; got != newPath || viewer.Err != "" {
		t.Fatalf("viewer path/error = %q/%q, want %q/empty", got, viewer.Err, newPath)
	}
	if got := filepath.Base(g.TabBar.Tabs[g.active].Name); got != filepath.Base(newPath) {
		t.Fatalf("visible tab name: got %q, want %q", got, filepath.Base(newPath))
	}
}

// The reported bug: after a rename the tab kept the old path, so saving wrote
// the buffer back to the name it had been renamed away from, leaving a ghost
// duplicate on disk alongside the renamed file.
func TestRenamePathUpdatesInlineDiffFilePath(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "file.txt")
	g := newRenameGroup(t, old, "package main")
	if !g.EnableInlineDiff(old, []string{"package main"}) {
		t.Fatal("could not enable inline diff")
	}

	newPath := filepath.Join(tmp, "file.go")
	if err := os.Rename(old, newPath); err != nil {
		t.Fatal(err)
	}
	if !g.RenamePath(old, newPath) {
		t.Fatal("expected RenamePath to report a tab change")
	}
	if got := g.tabs[g.active].Diff.FilePath; got != newPath {
		t.Fatalf("inline diff path: got %q, want %q", got, newPath)
	}
}

func TestRenamePathUpdatesImageViewerContent(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "image.png")
	writeTestPNG(t, old, 4, 4)
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	requireNoError(t, g.OpenFile(old))

	newPath := filepath.Join(tmp, "renamed.png")
	if err := os.Rename(old, newPath); err != nil {
		t.Fatal(err)
	}
	if !g.RenamePath(old, newPath) {
		t.Fatal("expected RenamePath to report an image-tab change")
	}
	viewer, ok := g.tabs[g.active].Content.(*ImageViewWidget)
	if !ok {
		t.Fatalf("content = %T, want image viewer", g.tabs[g.active].Content)
	}
	if viewer.FilePath != newPath || viewer.Err != "" {
		t.Fatalf("image viewer path/error = %q/%q, want %q/empty", viewer.FilePath, viewer.Err, newPath)
	}
}

func TestRenamePathUpdatesDiffTabFileIdentity(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "old.go")
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.OpenDiff(old, diff.FileDiff{}, []string{"old"}, []string{"new"}, false)

	newPath := filepath.Join(tmp, "renamed.go")
	if !g.RenamePath(old, newPath) {
		t.Fatal("expected RenamePath to report a diff-tab change")
	}
	if got := g.tabs[g.active].FilePath; got != newPath+" (diff)" {
		t.Fatalf("diff tab identity: got %q, want %q", got, newPath+" (diff)")
	}
	if got := g.tabs[g.active].IconPath; got != newPath {
		t.Fatalf("diff tab icon path: got %q, want %q", got, newPath)
	}
	if got := g.ActiveDiffWidget().FilePath; got != newPath {
		t.Fatalf("diff viewer path: got %q, want %q", got, newPath)
	}
}

func TestRenamePathSaveDoesNotRecreateOldFile(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "old.txt")
	g := newRenameGroup(t, old, "hello")

	newPath := filepath.Join(tmp, "new.txt")
	if err := os.Rename(old, newPath); err != nil {
		t.Fatalf("rename: %v", err)
	}
	g.RenamePath(old, newPath)

	g.tabs[g.active].Buf.Lines = []string{"edited"}
	g.tabs[g.active].Buf.Dirty = true
	if !g.Save() {
		t.Fatal("expected save to succeed")
	}

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("save recreated the old path %s as a ghost duplicate", old)
	}
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read %s: %v", newPath, err)
	}
	if string(data) != "edited\n" && string(data) != "edited" {
		t.Fatalf("new path contents: got %q, want edited", string(data))
	}
}

func TestRenamePathPreservesBufferAndUndo(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "old.txt")
	g := newRenameGroup(t, old, "hello\nworld")

	buf := g.tabs[g.active].Buf
	undo := g.tabs[g.active].Undo

	g.RenamePath(old, filepath.Join(tmp, "new.txt"))

	if g.tabs[g.active].Buf != buf {
		t.Error("buffer was replaced; a rename must not reload the document")
	}
	if g.tabs[g.active].Undo != undo {
		t.Error("undo stack was replaced; history must survive a rename")
	}
}

func TestRenamePathRedetectsLanguageOnExtensionChange(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "script.txt")
	g := newRenameGroup(t, old, "package main")

	newPath := filepath.Join(tmp, "script.go")
	g.RenamePath(old, newPath)

	h := g.tabs[g.active].Highlighter
	if h == nil {
		t.Fatal("expected a highlighter after rename")
	}
	if h.Language() != "Go" {
		t.Fatalf("language after rename to .go: got %q, want Go", h.Language())
	}
}

func TestRenamePathFolderMovesNestedTabs(t *testing.T) {
	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, "src")
	nested := filepath.Join(oldDir, "pkg", "a.txt")
	g := newRenameGroup(t, nested, "a")

	newDir := filepath.Join(tmp, "lib")
	if !g.RenamePath(oldDir, newDir) {
		t.Fatal("expected folder rename to move nested tabs")
	}

	want := filepath.Join(newDir, "pkg", "a.txt")
	if got := g.ActiveFilePath(); got != want {
		t.Fatalf("nested tab path: got %q, want %q", got, want)
	}
}

func TestRenamePathLeavesUnrelatedTabsAlone(t *testing.T) {
	tmp := t.TempDir()
	keep := filepath.Join(tmp, "keep.txt")
	g := newRenameGroup(t, keep, "keep")

	// A sibling whose name merely shares a prefix must not be dragged along.
	if g.RenamePath(filepath.Join(tmp, "keep"), filepath.Join(tmp, "moved")) {
		t.Fatal("prefix-only match should not rename keep.txt")
	}
	if got := g.ActiveFilePath(); got != keep {
		t.Fatalf("unrelated tab changed: got %q, want %q", got, keep)
	}
}

func TestRenamePathSkipsVirtualTabs(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	untitled := g.ActiveFilePath()

	if g.RenamePath(untitled, "renamed") {
		t.Fatal("virtual tabs have no path on disk and must not be renamed")
	}
	if got := g.ActiveFilePath(); got != untitled {
		t.Fatalf("virtual tab path changed: got %q, want %q", got, untitled)
	}
}

func TestRenamePathNotifiesLSPOfMove(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "old.go")
	g := newRenameGroup(t, old, "package main")

	var closed, opened string
	g.OnFileClose = func(path, _ string) { closed = path }
	g.OnFileOpen = func(path, _ string, _ []string) { opened = path }

	newPath := filepath.Join(tmp, "new.go")
	g.RenamePath(old, newPath)

	if closed != old {
		t.Errorf("expected didClose for %q, got %q", old, closed)
	}
	if opened != newPath {
		t.Errorf("expected didOpen for %q, got %q", newPath, opened)
	}
}

func TestRenamePathIgnoresNoOp(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a.txt")
	g := newRenameGroup(t, path, "a")

	if g.RenamePath(path, path) {
		t.Error("renaming a path to itself is not a change")
	}
	if g.RenamePath("", path) || g.RenamePath(path, "") {
		t.Error("empty paths must be ignored")
	}
}
