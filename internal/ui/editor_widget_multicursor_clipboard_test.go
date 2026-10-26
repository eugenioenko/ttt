package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func TestPastePiecesTrailingNewline(t *testing.T) {
	for _, tt := range []struct {
		text string
		want []string
	}{
		{"apple\n", []string{"apple\n", "apple\n"}},
		{"one\ntwo\n", []string{"one", "two"}},
		{"one\n\n", []string{"one", ""}},
	} {
		t.Run(tt.text, func(t *testing.T) {
			if got := pastePieces(tt.text, 2); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("pastePieces() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMultiCursorPasteWholeCopiedLine(t *testing.T) {
	clipboard.DisableSystem()
	g := newRenameGroup(t, filepath.Join(t.TempDir(), "lines.txt"), "apple\nfoo\nfoo\n")
	g.Copy()
	g.Editor.Cursor.Line, g.Editor.Cursor.Col = 1, 0
	g.Editor.SelectAllOccurrences()
	g.Paste()
	if got := strings.Join(g.Editor.Buf.Lines, "\n"); got != "apple\napple\n\napple\n" {
		t.Fatalf("pasted text = %q", got)
	}
}

func TestMultiCursorClipboardOverlappingOccurrences(t *testing.T) {
	clipboard.DisableSystem()
	for _, operation := range []string{"copy", "cut", "paste", "multiline paste"} {
		t.Run(operation, func(t *testing.T) {
			g := newRenameGroup(t, filepath.Join(t.TempDir(), "overlap.txt"), "aaaaZ\n")
			e := g.Editor
			e.Selection.Start(0, 0)
			e.Cursor.Col = 2
			e.SelectAllOccurrences()
			want := "aaaaZ"
			switch operation {
			case "copy":
				g.Copy()
				if got := clipboard.Get(); got != "aaaa" {
					t.Errorf("copied text = %q, want aaaa", got)
				}
			case "cut":
				g.Cut()
				want = "Z"
				if got := clipboard.Get(); got != "aaaa" {
					t.Errorf("cut text = %q, want aaaa", got)
				}
			case "paste":
				g.PasteText("X")
				want = "XZ"
			case "multiline paste":
				g.PasteText("X\n")
				want = "X\nZ"
			}
			if got := strings.Join(e.Buf.Lines, "\n"); got != want {
				t.Fatalf("buffer = %q, want %q", got, want)
			}
			if operation != "copy" {
				e.Undo.Undo(e.Buf)
				if got := strings.Join(e.Buf.Lines, "\n"); got != "aaaaZ" {
					t.Fatalf("undo = %q, want original text", got)
				}
				e.Undo.Redo(e.Buf)
				if got := strings.Join(e.Buf.Lines, "\n"); got != want {
					t.Fatalf("redo = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestMultiCursorClipboardMergedPieceMapping(t *testing.T) {
	e := newMultiCursorEditor([]string{"aaaaZ", "foo!"}, [][2]int{{0, 2}, {0, 3}, {0, 4}, {1, 3}})
	e.Undo = &undo.UndoStack{}
	for i, anchor := range []selection.Position{{Line: 0, Col: 0}, {Line: 0, Col: 1}, {Line: 0, Col: 2}, {Line: 1, Col: 0}} {
		e.Multi.Cursors[i].Sel = selection.Selection{Active: true, Anchor: anchor}
	}
	if got := e.multiCopyPieces(); !reflect.DeepEqual(got, []string{"aaaa", "foo"}) {
		t.Fatalf("copied pieces = %q, want [aaaa foo]", got)
	}
	e.multiPasteText("one\ntwo")
	want := []string{"oneZ", "two!"}
	if !reflect.DeepEqual(e.Buf.Lines, want) {
		t.Fatalf("buffer = %q, want %q", e.Buf.Lines, want)
	}
	if e.Multi.Cursors[0].Col != 3 || e.Multi.Cursors[1].Col != 3 {
		t.Fatalf("paste cursor positions = %+v", e.Multi.Cursors)
	}
	e.Undo.Undo(e.Buf)
	if !reflect.DeepEqual(e.Buf.Lines, []string{"aaaaZ", "foo!"}) {
		t.Fatalf("undo = %q, want original text", e.Buf.Lines)
	}
}

func TestMultiCursorPasteRefreshesFoldsAndViewport(t *testing.T) {
	e := newMultiCursorEditor([]string{"foo", "foo"}, [][2]int{{0, 0}, {1, 0}})
	e.Undo = &undo.UndoStack{}
	e.Folds = fold.NewState()
	e.Folds.SetRanges(fold.ComputeIndentRanges(e.Buf.Lines))
	e.Viewport.Height = 5
	e.multiPasteText(strings.Repeat("head\n    child\n", 15) + "end")
	if e.Cursor.Line < e.Viewport.TopLine || e.Cursor.Line >= e.Viewport.TopLine+e.Viewport.Height {
		t.Errorf("cursor line %d outside viewport %+v", e.Cursor.Line, e.Viewport)
	}
	if got := e.Folds.FoldAt(0); got == nil || got.EndLine != 1 {
		t.Errorf("first pasted fold = %+v, want lines 0–1", got)
	}
	if got := e.Folds.FoldAt(2); got == nil || got.EndLine != 3 {
		t.Errorf("second pasted fold = %+v, want lines 2–3", got)
	}
}

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
