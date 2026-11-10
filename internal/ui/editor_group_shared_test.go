package ui

import (
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
	left.activeTab().rememberView()
	right.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	right.SyncDocumentViews()
	if left.Editor.Cursor.Line != 3 || left.Editor.Cursor.Col != 4 || left.ActiveBuffer().Lines[3] != "var message = \"hello\"" {
		t.Fatalf("other view lost its position after a newline above it: %+v", left.Editor.Cursor)
	}
	right.Undo()
	right.SyncDocumentViews()
	if left.Editor.Cursor.Line != 2 || left.ActiveBuffer().Dirty {
		t.Fatalf("shared undo did not restore the document: %+v dirty %v", left.Editor.Cursor, left.ActiveBuffer().Dirty)
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
