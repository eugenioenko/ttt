package ui

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func newFoldedEditor(foldLine int, lines ...string) *EditorPaneWidget {
	e := newEditorWithLines(lines...)
	e.Undo = &undo.UndoStack{}
	e.Folds = fold.NewState()
	e.Folds.SetRanges(fold.ComputeIndentRanges(e.Buf.Lines))
	e.Folds.Toggle(foldLine)
	e.Cursor.Line = foldLine
	return e
}

var foldSample = []string{
	"func outer() {",
	"\tif true {",
	"\t\tfoo()",
	"\t}",
	"\tbar()",
	"}",
}

func TestMoveLineDownCarriesFoldedBody(t *testing.T) {
	e := newFoldedEditor(1, foldSample...)

	e.MoveLineDown()

	want := []string{"func outer() {", "\t}", "\tif true {", "\t\tfoo()", "\tbar()", "}"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("lines = %q, want %q", e.Buf.Lines, want)
	}
	if e.Cursor.Line != 2 || !e.Folds.IsCollapsed(2) {
		t.Fatalf("cursor line %d, collapsed at 2: %v; want the fold to follow the header", e.Cursor.Line, e.Folds.IsCollapsed(2))
	}
}

func TestMoveLineUpStepsOverFoldedNeighbor(t *testing.T) {
	e := newFoldedEditor(1, foldSample...)
	e.Cursor.Line = 3

	e.MoveLineUp()

	want := []string{"func outer() {", "\t}", "\tif true {", "\t\tfoo()", "\tbar()", "}"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("lines = %q, want %q", e.Buf.Lines, want)
	}
	if e.Cursor.Line != 1 || !e.Folds.IsCollapsed(2) {
		t.Fatalf("cursor line %d, collapsed at 2: %v", e.Cursor.Line, e.Folds.IsCollapsed(2))
	}
}

func TestUndoMoveRestoresFoldPosition(t *testing.T) {
	e := newFoldedEditor(1, foldSample...)

	e.MoveLineDown()
	e.Undo.Undo(e.Buf)
	e.Folds.SetRanges(fold.ComputeIndentRanges(e.Buf.Lines))

	if !slices.Equal(e.Buf.Lines, foldSample) {
		t.Fatalf("lines = %q, want %q", e.Buf.Lines, foldSample)
	}
	if !e.Folds.IsCollapsed(1) || e.Folds.IsCollapsed(2) {
		t.Fatalf("collapsed at 1: %v, at 2: %v; want the fold back on the if-block", e.Folds.IsCollapsed(1), e.Folds.IsCollapsed(2))
	}
}

func TestMoveLineWithoutFoldsSwapsOneLine(t *testing.T) {
	e := newEditorWithLines("a", "b", "c")
	e.Undo = &undo.UndoStack{}
	e.Cursor.Line = 1

	e.MoveLineDown()

	if want := []string{"a", "c", "b"}; !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("lines = %q, want %q", e.Buf.Lines, want)
	}
	if e.Cursor.Line != 2 {
		t.Fatalf("cursor line = %d, want 2", e.Cursor.Line)
	}
}
