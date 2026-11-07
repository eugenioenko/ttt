package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/gdamore/tcell/v3"
)

func newWrapEditor(width int, lines ...string) *EditorPaneWidget {
	e := newEditorWithLines(lines...)
	e.WordWrap = true
	e.TabSize = 4
	e.Viewport.Width = width
	return e
}

func pressWrapKey(e *EditorPaneWidget, key tcell.Key, mods tcell.ModMask) {
	e.HandleEvent(tcell.NewEventKey(key, "", mods))
}

func assertCursor(t *testing.T, e *EditorPaneWidget, line, col int) {
	t.Helper()
	if e.Cursor.Line != line || e.Cursor.Col != col {
		t.Fatalf("cursor = (%d,%d), want (%d,%d)", e.Cursor.Line, e.Cursor.Col, line, col)
	}
}

func TestWrapDownMovesThroughWrappedRows(t *testing.T) {
	e := newWrapEditor(5, "aaaaabbbbbccccc", "ddd")
	e.Cursor.Col = 2

	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 0, 7)
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 0, 12)
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 2)
}

func TestWrapUpMovesThroughWrappedRows(t *testing.T) {
	e := newWrapEditor(5, "ddd", "aaaaabbbbbccccc")
	e.Cursor.Line, e.Cursor.Col = 1, 12

	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 1, 7)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 1, 2)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 2)
}

func TestWrapUpOnFirstRowAndDownOnLastRowStay(t *testing.T) {
	e := newWrapEditor(5, "aaaaabbb")
	e.Cursor.Col = 3

	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 3)

	e.Cursor.Col = 6
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 0, 6)
}

func TestWrapGoalColumnSurvivesShortRows(t *testing.T) {
	e := newWrapEditor(5, "abcdefgh", "xy", "0123456789")
	e.Cursor.Col = 4

	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 0, 8)
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 2)
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 2, 4)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 1, 2)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 8)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 4)
}

func TestWrapGoalColumnResetsAfterHorizontalMove(t *testing.T) {
	e := newWrapEditor(5, "abcde", "xy", "0123456789")
	e.Cursor.Col = 4

	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 2)
	pressWrapKey(e, tcell.KeyLeft, 0)
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 2, 1)
}

func TestWrapGoalColumnResetsAfterEdit(t *testing.T) {
	e := newWrapEditor(5, "abcde", "xy", "0123456789")
	e.Cursor.Col = 4

	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 2)
	e.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "z", 0))
	pressWrapKey(e, tcell.KeyBackspace2, 0)
	assertCursor(t, e, 1, 2)
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 2, 2)
}

func TestWrapDownPastShortMiddleRowStaysOnThatRow(t *testing.T) {
	e := newWrapEditor(5, "abcde", "가나다")
	e.Cursor.Col = 4

	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 1)
	if row, _ := e.layout().rowOf(1, 1); row != 1 {
		t.Fatalf("cursor drawn on row %d, want the first row of line 1", row)
	}
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 3)
}

func TestWrapGoalInsideFullwidthRune(t *testing.T) {
	e := newWrapEditor(10, "abc", "가나다")
	e.Cursor.Col = 3

	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 1)
}

func TestWrapGoalCountsTabWidth(t *testing.T) {
	e := newWrapEditor(20, "abcdef", "\tab")
	e.Cursor.Col = 5

	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 2)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 5)
}

func TestWrapShiftDownSelectsOneScreenRow(t *testing.T) {
	e := newWrapEditor(5, "aaaaabbbbbccccc")
	e.Selection = &selection.Selection{}
	e.Cursor.Col = 2

	pressWrapKey(e, tcell.KeyDown, tcell.ModShift)
	assertCursor(t, e, 0, 7)
	if got := e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col); got != "aaabb" {
		t.Fatalf("selection = %q, want %q", got, "aaabb")
	}
	pressWrapKey(e, tcell.KeyUp, tcell.ModShift)
	assertCursor(t, e, 0, 2)
	if e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col) != "" {
		t.Fatal("Shift+Up back to the anchor should leave an empty selection")
	}
}

func TestWrapPageDownMovesByScreenRows(t *testing.T) {
	e := newWrapEditor(5, "aaaaabbbbbcccccdddddeeeee", "fff")
	e.Viewport.Height = 3
	e.Cursor.Col = 1

	pressWrapKey(e, tcell.KeyPgDn, 0)
	assertCursor(t, e, 0, 16)
	pressWrapKey(e, tcell.KeyPgDn, 0)
	assertCursor(t, e, 1, 1)
	pressWrapKey(e, tcell.KeyPgUp, 0)
	assertCursor(t, e, 0, 11)
	pressWrapKey(e, tcell.KeyPgUp, 0)
	assertCursor(t, e, 0, 1)
}

func TestWrapPageDownClampsToLastRow(t *testing.T) {
	e := newWrapEditor(5, "aaaaabb")
	e.Viewport.Height = 10
	e.Cursor.Col = 3

	pressWrapKey(e, tcell.KeyPgDn, 0)
	assertCursor(t, e, 0, 7)
	pressWrapKey(e, tcell.KeyPgUp, 0)
	assertCursor(t, e, 0, 3)
}

func TestWrapMoveSkipsPhantomRows(t *testing.T) {
	e := newWrapEditor(5, "aaaaabb", "ccc")
	e.setPhantoms(map[int]int{0: 1, 1: 2})
	e.Cursor.Col = 6

	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 1)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 6)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 1)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 1)
}

func TestWrapPageUpSkipsLeadingPhantomRows(t *testing.T) {
	e := newWrapEditor(5, "aaa", "bbb")
	e.setPhantoms(map[int]int{0: 3})
	e.Viewport.Height = 10
	e.Cursor.Line, e.Cursor.Col = 1, 2

	pressWrapKey(e, tcell.KeyPgUp, 0)
	assertCursor(t, e, 0, 2)
}

func TestWrapMultiCursorMovesByScreenRow(t *testing.T) {
	e := newWrapEditor(5, "aaaaabbbbb", "cccccddddd")
	e.Cursor.Col = 1
	e.Multi = &multicursor.MultiCursor{Cursors: []multicursor.CursorState{
		{Line: 0, Col: 1},
		{Line: 1, Col: 3},
	}}

	pressWrapKey(e, tcell.KeyDown, 0)
	got := e.Multi.Cursors
	if len(got) != 2 || got[0].Line != 0 || got[0].Col != 6 || got[1].Line != 1 || got[1].Col != 8 {
		t.Fatalf("cursors after Down = %+v, want (0,6) and (1,8)", got)
	}
	pressWrapKey(e, tcell.KeyUp, 0)
	got = e.Multi.Cursors
	if len(got) != 2 || got[0].Line != 0 || got[0].Col != 1 || got[1].Line != 1 || got[1].Col != 3 {
		t.Fatalf("cursors after Up = %+v, want (0,1) and (1,3)", got)
	}
}

func TestNoWrapDownMovesByBufferLine(t *testing.T) {
	e := newEditorWithLines("aaaaabbbbbccccc", "ddd")
	e.Viewport.Width = 5
	e.Cursor.Col = 2

	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 2)
}

func TestColAtX(t *testing.T) {
	l := wrapLayout([]string{"abcdefghij", "가나다", "a\tb"}, 5, 4)
	tests := []struct {
		line, startCol, x, want int
	}{
		{0, 0, 0, 0},
		{0, 0, 4, 4},
		{0, 0, 9, 4},
		{0, 5, 2, 7},
		{0, 5, 9, 10},
		{1, 0, 1, 0},
		{1, 0, 3, 1},
		{1, 0, 4, 1},
		{1, 2, 0, 2},
		{1, 2, 4, 3},
		{2, 0, 1, 1},
		{2, 0, 3, 1},
		{2, 0, 4, 2},
	}
	for _, tt := range tests {
		r := editorRow{bufLine: tt.line, startCol: tt.startCol, phantom: -1}
		if got := l.colAtX(r, tt.x); got != tt.want {
			t.Errorf("colAtX(line %d, start %d, x %d) = %d, want %d", tt.line, tt.startCol, tt.x, got, tt.want)
		}
	}
}
