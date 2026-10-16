package ui

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/selection"
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

func TestInsertLineAboveAndBelowUndoCursor(t *testing.T) {
	above := newEditorWithLines("A", "B", "C", "D", "E")
	above.Undo = &undo.UndoStack{}
	above.Cursor.Line = 2
	above.Cursor.Col = 1
	above.InsertLineAbove()
	above.Cursor.Line = 4

	pos := above.Undo.Undo(above.Buf)
	if pos == nil || pos.Line != 2 || pos.Col != 0 {
		t.Fatalf("undo after insert above = %+v, want {2 0}", pos)
	}
	pos = above.Undo.Redo(above.Buf)
	if pos == nil || pos.Line != 2 || pos.Col != 0 {
		t.Fatalf("redo after insert above = %+v, want {2 0}", pos)
	}

	below := newEditorWithLines("A", "B", "C", "D", "E")
	below.Undo = &undo.UndoStack{}
	below.Cursor.Line = 2
	below.InsertLineBelow()
	below.Cursor.Line = 0

	pos = below.Undo.Undo(below.Buf)
	if pos == nil || pos.Line != 2 || pos.Col != 0 {
		t.Fatalf("undo after insert below = %+v, want {2 0}", pos)
	}
	pos = below.Undo.Redo(below.Buf)
	if pos == nil || pos.Line != 3 || pos.Col != 0 {
		t.Fatalf("redo after insert below = %+v, want {3 0}", pos)
	}
}

func TestMoveLineUndoRedoCursor(t *testing.T) {
	down := newEditorWithLines("A", "B", "C", "D", "E")
	down.Undo = &undo.UndoStack{}
	down.Cursor.Line = 2
	down.MoveLineDown()
	down.Cursor.Line = 0

	pos := down.Undo.Undo(down.Buf)
	if pos == nil || pos.Line != 2 || pos.Col != 0 {
		t.Fatalf("undo after move down = %+v, want {2 0}", pos)
	}
	pos = down.Undo.Redo(down.Buf)
	if pos == nil || pos.Line != 3 || pos.Col != 0 {
		t.Fatalf("redo after move down = %+v, want {3 0}", pos)
	}

	up := newEditorWithLines("A", "B", "C", "D", "E")
	up.Undo = &undo.UndoStack{}
	up.Cursor.Line = 2
	up.MoveLineUp()
	up.Cursor.Line = 4

	pos = up.Undo.Undo(up.Buf)
	if pos == nil || pos.Line != 2 || pos.Col != 0 {
		t.Fatalf("undo after move up = %+v, want {2 0}", pos)
	}
	pos = up.Undo.Redo(up.Buf)
	if pos == nil || pos.Line != 1 || pos.Col != 0 {
		t.Fatalf("redo after move up = %+v, want {1 0}", pos)
	}
}

func TestMoveLineBlockUndoRedoCursor(t *testing.T) {
	e := newEditorWithLines("A", "B", "C", "D", "E")
	e.Undo = &undo.UndoStack{}
	e.Selection = &selection.Selection{}
	e.Selection.Start(1, 0)
	e.Cursor.Line = 2
	e.Cursor.Col = 1
	e.MoveLineDown()
	e.Cursor.Line = 0

	pos := e.Undo.Undo(e.Buf)
	if pos == nil || pos.Line != 1 || pos.Col != 0 {
		t.Fatalf("undo cursor = %+v, want {1 0}", pos)
	}
	pos = e.Undo.Redo(e.Buf)
	if pos == nil || pos.Line != 2 || pos.Col != 0 {
		t.Fatalf("redo cursor = %+v, want {2 0}", pos)
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
