package ui

import (
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func TestMultiCursorCopyPiecesUsesSelectionsAndLines(t *testing.T) {
	e := newMultiCursorEditor(
		[]string{"apple", "banana", "cherry"},
		[][2]int{{0, 2}, {1, 3}},
	)

	// Select "ap" from the first line.
	e.Multi.Cursors[0].Sel = selection.Selection{
		Active: true,
		Anchor: selection.Position{Line: 0, Col: 0},
	}

	// The second cursor has no selection, so Copy should use its whole line.
	got := e.multiCopyPieces()
	want := []string{"ap", "banana"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("multiCopyPieces() = %#v, want %#v", got, want)
	}
}

func TestPastePiecesDistributesMatchingLines(t *testing.T) {
	got := pastePieces("one\ntwo", 2)
	want := []string{"one", "two"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pastePieces() = %#v, want %#v", got, want)
	}
}

func TestPastePiecesRepeatsTextWhenLineCountDoesNotMatch(t *testing.T) {
	got := pastePieces("hello\nworld", 3)
	want := []string{"hello\nworld", "hello\nworld", "hello\nworld"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pastePieces() = %#v, want %#v", got, want)
	}
}

func TestMultiCursorPasteTextAtEachCursor(t *testing.T) {
	e := newMultiCursorEditor(
		[]string{"abc", "xyz"},
		[][2]int{{0, 1}, {1, 1}},
	)
	e.Undo = &undo.UndoStack{}

	e.multiPasteText("Q\nR")

	if e.Buf.Lines[0] != "aQbc" || e.Buf.Lines[1] != "xRyz" {
		t.Fatalf("unexpected buffer after multi-cursor paste: %#v", e.Buf.Lines)
	}
}

func TestMultiCursorCutLinesAndUndo(t *testing.T) {
	e := newMultiCursorEditor(
		[]string{"apple", "banana", "cherry"},
		[][2]int{{0, 0}, {2, 0}},
	)
	e.Undo = &undo.UndoStack{}

	e.multiCut()

	if !reflect.DeepEqual(e.Buf.Lines, []string{"banana"}) {
		t.Fatalf("buffer after multi-cursor cut = %#v, want [banana]", e.Buf.Lines)
	}

	e.Undo.Undo(e.Buf)

	if !reflect.DeepEqual(e.Buf.Lines, []string{"apple", "banana", "cherry"}) {
		t.Fatalf("buffer after undo = %#v, want original lines", e.Buf.Lines)
	}
}

func TestMultiCursorPasteUndoRestoresOriginalText(t *testing.T) {
	e := newMultiCursorEditor(
		[]string{"abc", "xyz"},
		[][2]int{{0, 1}, {1, 1}},
	)
	e.Undo = &undo.UndoStack{}

	e.multiPasteText("Q\nR")

	e.Undo.Undo(e.Buf)

	if !reflect.DeepEqual(e.Buf.Lines, []string{"abc", "xyz"}) {
		t.Fatalf("buffer after undo = %#v, want original lines", e.Buf.Lines)
	}
}

func TestMultiCursorCutMixedSelections(t *testing.T) {
	e := newMultiCursorEditor(
		[]string{"apple", "banana"},
		[][2]int{{0, 2}, {1, 3}},
	)
	e.Undo = &undo.UndoStack{}

	// Select "ap" on the first cursor; the second has no selection.
	e.Multi.Cursors[0].Sel = selection.Selection{
		Active: true,
		Anchor: selection.Position{Line: 0, Col: 0},
	}

	copied := e.multiCopyPieces()
	wantCopied := []string{"ap", "banana"}
	if !reflect.DeepEqual(copied, wantCopied) {
		t.Fatalf("copied pieces = %#v, want %#v", copied, wantCopied)
	}

	e.multiCut()

	wantLines := []string{"ple"}
	if !reflect.DeepEqual(e.Buf.Lines, wantLines) {
		t.Fatalf("buffer after mixed-selection cut = %#v, want %#v", e.Buf.Lines, wantLines)
	}

	e.Undo.Undo(e.Buf)

	wantOriginal := []string{"apple", "banana"}
	if !reflect.DeepEqual(e.Buf.Lines, wantOriginal) {
		t.Fatalf("buffer after undo = %#v, want %#v", e.Buf.Lines, wantOriginal)
	}
}

func TestAdjustSelectionAnchorAfterLineDelete(t *testing.T) {
	cs := multicursor.CursorState{}
	cs.Sel.Start(3, 2)

	adjustSelectionAnchorAfterLineDelete(&cs, 1)

	if cs.Sel.Anchor.Line != 2 {
		t.Fatalf("anchor line = %d, want 2", cs.Sel.Anchor.Line)
	}

	if cs.Sel.Anchor.Col != 2 {
		t.Fatalf("anchor col = %d, want 2", cs.Sel.Anchor.Col)
	}

	if !cs.Sel.Active {
		t.Fatal("selection should remain active")
	}
}

func TestMultiCursorPasteSameLineTracksPositions(t *testing.T) {
	e := newMultiCursorEditor(
		[]string{"abcdef"},
		[][2]int{{0, 1}, {0, 4}},
	)
	e.Undo = &undo.UndoStack{}

	e.multiPasteText("X")

	if got, want := e.Buf.Lines[0], "aXbcdXef"; got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}

	if got, want := e.Multi.Cursors[0].Col, 2; got != want {
		t.Errorf("first cursor col = %d, want %d", got, want)
	}

	if got, want := e.Multi.Cursors[1].Col, 6; got != want {
		t.Errorf("second cursor col = %d, want %d", got, want)
	}

	e.Undo.Undo(e.Buf)

	if got, want := e.Buf.Lines[0], "abcdef"; got != want {
		t.Errorf("buffer after undo = %q, want %q", got, want)
	}
}

func TestMultiCursorMultilinePasteTracksPositions(t *testing.T) {
	e := newMultiCursorEditor(
		[]string{"abcdef"},
		[][2]int{{0, 1}, {0, 4}},
	)
	e.Undo = &undo.UndoStack{}

	e.multiPasteText("X\nY\nZ")

	want := []string{
		"aX",
		"Y",
		"ZbcdX",
		"Y",
		"Zef",
	}

	if !reflect.DeepEqual(e.Buf.Lines, want) {
		t.Fatalf("buffer = %#v, want %#v", e.Buf.Lines, want)
	}

	if got := e.Multi.Cursors[0]; got.Line != 2 || got.Col != 1 {
		t.Errorf("first cursor = (%d,%d), want (2,1)", got.Line, got.Col)
	}

	if got := e.Multi.Cursors[1]; got.Line != 4 || got.Col != 1 {
		t.Errorf("second cursor = (%d,%d), want (4,1)", got.Line, got.Col)
	}

	e.Undo.Undo(e.Buf)

	if !reflect.DeepEqual(e.Buf.Lines, []string{"abcdef"}) {
		t.Errorf("undo buffer = %#v, want original", e.Buf.Lines)
	}
}

func TestMultiCursorCutSameLineDoesNotDeleteNextLine(t *testing.T) {
	e := newMultiCursorEditor(
		[]string{"apple", "banana", "cherry"},
		[][2]int{{0, 1}, {0, 3}},
	)
	e.Undo = &undo.UndoStack{}

	e.multiCut()

	want := []string{"banana", "cherry"}
	if !reflect.DeepEqual(e.Buf.Lines, want) {
		t.Fatalf("buffer after cut = %#v, want %#v", e.Buf.Lines, want)
	}

	e.Undo.Undo(e.Buf)

	original := []string{"apple", "banana", "cherry"}
	if !reflect.DeepEqual(e.Buf.Lines, original) {
		t.Errorf("buffer after undo = %#v, want %#v", e.Buf.Lines, original)
	}
}
