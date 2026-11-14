package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func sharedTestGroups(t *testing.T) (*EditorGroupWidget, *EditorGroupWidget) {
	t.Helper()
	left := newRenameGroup(t, filepath.Join(t.TempDir(), "shared.go"), "package main\n\nvar message = \"hello\"\n")
	right := NewEditorGroupWidget(nil, 4, true, "relative")
	right.SyntaxHighlight = true
	groups := []*EditorGroupWidget{left, right}
	for _, g := range groups {
		g.RelatedGroups = func() []*EditorGroupWidget { return groups }
	}
	if !left.CloneActiveTabTo(right) {
		t.Fatal("failed to split document")
	}
	t.Cleanup(func() { left.CloseAllTabs(); right.CloseAllTabs() })
	return left, right
}

func TestSharedDocumentIndependentViewsAndUndo(t *testing.T) {
	left, right := sharedTestGroups(t)
	if left.Editor.Cursor == right.Editor.Cursor || left.Editor.Viewport == right.Editor.Viewport || left.Editor.Selection == right.Editor.Selection || left.Editor.Highlighter == right.Editor.Highlighter {
		t.Fatal("split shared per-view state")
	}
	if left.ActiveBuffer() != right.ActiveBuffer() || left.Editor.Undo != right.Editor.Undo {
		t.Fatal("split copied the document instead of sharing it")
	}
	left.GoToLineCol(3, 5)
	left.Editor.Selection.Active = true
	left.Editor.Selection.Anchor.Line, left.Editor.Selection.Anchor.Col = 2, 1
	left.activeTab().rememberView()
	right.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	right.SyncDocumentViews()
	if left.Editor.Cursor.Line != 3 || left.Editor.Cursor.Col != 4 || left.ActiveBuffer().Lines[3] != "var message = \"hello\"" {
		t.Fatalf("other view lost its position after a newline above it: %+v", left.Editor.Cursor)
	}
	if left.Editor.Selection.Anchor.Line != 3 || left.Editor.Selection.Anchor.Col != 1 {
		t.Fatal("the other view's selection did not follow the inserted line")
	}
	right.Undo()
	right.SyncDocumentViews()
	if left.Editor.Cursor.Line != 2 || left.ActiveBuffer().Dirty {
		t.Fatalf("shared undo did not restore the document: %+v dirty %v", left.Editor.Cursor, left.ActiveBuffer().Dirty)
	}
	if left.Editor.Selection.Anchor.Line != 2 || left.Editor.Selection.Anchor.Col != 1 {
		t.Fatal("shared undo did not restore the other view's selection")
	}
	top := right.Editor.Viewport.TopLine
	left.Editor.Viewport.TopLine = top + 1
	if right.Editor.Viewport.TopLine != top {
		t.Fatal("scrolling one view scrolled the other")
	}
}

func TestSharedDocumentCloseKeepsLanguageAndSaveAsUpdatesBoth(t *testing.T) {
	left, right := sharedTestGroups(t)
	closed := 0
	for _, group := range []*EditorGroupWidget{left, right} {
		group.OnFileClose = func(string, string) { closed++ }
	}
	path := filepath.Join(t.TempDir(), "renamed.go")
	if !right.SaveAs(path) || left.ActiveFilePath() != path || left.ActiveBuffer() != right.ActiveBuffer() {
		t.Fatal("Save As did not update both views")
	}
	left.CloseTab()
	if closed != 0 || right.Editor.Highlighter == nil {
		t.Fatal("closing one view closed the shared document")
	}
	right.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", 0))
	right.Editor.FlushOnChange()
	right.CloseTab()
	if closed != 1 {
		t.Fatalf("document closes = %d, want one", closed)
	}
}

func TestSharedDocumentSaveAsGuardsOtherGroup(t *testing.T) {
	left, right := sharedTestGroups(t)
	path := filepath.Join(t.TempDir(), "destination.txt")
	if err := os.WriteFile(path, []byte("destination\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := right.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	right.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", 0))
	if left.SaveAs(path) {
		t.Fatal("Save As replaced an unsaved document in another group")
	}
	right.Undo()
	if !left.SaveAs(path) {
		t.Fatal("Save As could not replace a clean destination")
	}
	if right.TabCount() != 1 || right.ActiveBuffer() != left.ActiveBuffer() || right.Editor.Undo != left.Editor.Undo {
		t.Fatal("Save As left duplicate or independent destination views")
	}
}

func TestClonedDocumentDoesNotInheritPlaceholderPin(t *testing.T) {
	source := newRenameGroup(t, filepath.Join(t.TempDir(), "file.txt"), "text")
	target := NewEditorGroupWidget(nil, 4, true, "relative")
	target.TogglePinTab()
	if !source.CloneActiveTabTo(target) || target.TabCount() != 1 || target.IsActiveTabPinned() {
		t.Fatal("cloned document inherited the replaced placeholder's pin")
	}
}

func TestSharedDocumentReloadAndRenameKeepOneLifecycle(t *testing.T) {
	left, right := sharedTestGroups(t)
	oldPath := left.ActiveFilePath()
	if err := os.WriteFile(oldPath, []byte("package updated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !left.ReloadFile(oldPath) || right.ActiveBuffer().Lines[0] != "package updated" {
		t.Fatal("reloading one view did not update the shared document")
	}
	var closed, opened []string
	for _, group := range []*EditorGroupWidget{left, right} {
		group.OnFileClose = func(path, _ string) { closed = append(closed, path) }
		group.OnFileOpen = func(path, _ string, _ []string) { opened = append(opened, path) }
	}
	newPath := filepath.Join(filepath.Dir(oldPath), "renamed.go")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	if !left.RenamePath(oldPath, newPath) || right.RenamePath(oldPath, newPath) {
		t.Fatal("rename was not applied once across related views")
	}
	if right.ActiveFilePath() != newPath || left.ActiveBuffer() != right.ActiveBuffer() {
		t.Fatal("renaming one view detached its peer")
	}
	if len(closed) != 1 || closed[0] != oldPath || len(opened) != 1 || opened[0] != newPath {
		t.Fatalf("document lifecycle: closed %v, opened %v", closed, opened)
	}
}

func TestCloningKeepsAnEditedThenUndonePlaceholder(t *testing.T) {
	source := newRenameGroup(t, filepath.Join(t.TempDir(), "file.txt"), "text")
	target := NewEditorGroupWidget(nil, 4, true, "relative")
	buf := target.ActiveBuffer()
	target.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", 0))
	target.Undo()
	if buf.Dirty || buf.Lines[0] != "" {
		t.Fatal("undo did not return the placeholder to empty")
	}
	if !source.CloneActiveTabTo(target) || target.TabCount() != 2 || target.tabs[0].Buf != buf {
		t.Fatal("cloning discarded a placeholder the user had edited")
	}
}
