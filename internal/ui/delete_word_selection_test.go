package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

// With an active selection, Ctrl+Backspace deletes the selection rather than a
// word from the cursor (matching VS Code).
func TestDeleteWordLeftDeletesSelectionFirst(t *testing.T) {
	e := newEditorWithLines("foobar")
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 3) // anchor at col 3
	e.Cursor.Line, e.Cursor.Col = 0, 6

	e.DeleteWordLeft()

	if e.Buf.Lines[0] != "foo" {
		t.Fatalf("expected selection %q deleted leaving %q, got %q", "bar", "foo", e.Buf.Lines[0])
	}
	if e.Selection.Active {
		t.Fatal("expected selection cleared after delete")
	}
}

// With an active selection, Ctrl+Delete deletes the selection rather than a word
// to the right of the cursor.
func TestDeleteWordRightDeletesSelectionFirst(t *testing.T) {
	e := newEditorWithLines("foobar")
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0) // anchor at col 0
	e.Cursor.Line, e.Cursor.Col = 0, 3

	e.DeleteWordRight()

	if e.Buf.Lines[0] != "bar" {
		t.Fatalf("expected selection %q deleted leaving %q, got %q", "foo", "bar", e.Buf.Lines[0])
	}
	if e.Selection.Active {
		t.Fatal("expected selection cleared after delete")
	}
}

func TestUndoDeleteWordRightKeepsCaretAtStart(t *testing.T) {
	e := newEditorWithLines("foo bar foo")
	e.Cursor.Col = 4
	e.Undo = &undo.UndoStack{}
	e.DeleteWordRight()
	if got := e.Buf.Lines[0]; got != "foo  foo" {
		t.Fatalf("after deleting word: %q, want foo  foo", got)
	}
	pos := e.Undo.Undo(e.Buf)
	if got := e.Buf.Lines[0]; got != "foo bar foo" {
		t.Errorf("after undo: %q, want foo bar foo", got)
	}
	if pos == nil || pos.Line != 0 || pos.Col != 4 {
		t.Errorf("undo caret = %+v, want 0:4", pos)
	}
}
