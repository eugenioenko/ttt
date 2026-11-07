package ui

import (
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/selection"
)

func assertAddedCursors(t *testing.T, e *EditorPaneWidget, want ...[2]int) {
	t.Helper()
	got := [][2]int{{e.Cursor.Line, e.Cursor.Col}}
	if e.Multi != nil {
		got = nil
		for _, c := range e.Multi.Cursors {
			got = append(got, [2]int{c.Line, c.Col})
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cursors = %v, want %v", got, want)
	}
}

func TestAddCursorVertical(t *testing.T) {
	for _, tt := range []struct {
		name         string
		lines        []string
		line, col    int
		delta, width int
		wrap         bool
		want         [][2]int
	}{
		{"below", []string{"abcde", "x", "abcde"}, 0, 4, 1, 20, false, [][2]int{{0, 4}, {1, 1}, {2, 4}}},
		{"above", []string{"abcde", "x", "abcde"}, 2, 4, -1, 20, false, [][2]int{{0, 4}, {1, 1}, {2, 4}}},
		{"empty lines", []string{"abcde", "", "", "abcde"}, 0, 4, 1, 20, false, [][2]int{{0, 4}, {1, 0}, {2, 0}, {3, 4}}},
		{"tabs and wide runes", []string{"abcde", "\tab", "가나다", "abcde"}, 0, 5, 1, 20, false, [][2]int{{0, 5}, {1, 2}, {2, 2}, {3, 5}}},
		{"no wrap", []string{"aaaaabbbbbccccc", "ddd"}, 0, 2, 1, 5, false, [][2]int{{0, 2}, {1, 2}}},
		{"wrapped below", []string{"aaaaabbbbbccccc", "d", "eeeee"}, 0, 2, 1, 5, true, [][2]int{{0, 2}, {0, 7}, {0, 12}, {1, 1}, {2, 2}}},
		{"wrapped above", []string{"aaaaabbbbbccccc", "d", "eeeee"}, 2, 2, -1, 5, true, [][2]int{{0, 2}, {0, 7}, {0, 12}, {1, 1}, {2, 2}}},
		{"from wrapped segment", []string{"aaaaabbbbbccccc", "ddd"}, 0, 7, 1, 5, true, [][2]int{{0, 7}, {0, 12}, {1, 2}}},
		{"wrapped wide runes", []string{"abcde", "가나다"}, 0, 4, 1, 5, true, [][2]int{{0, 4}, {1, 1}, {1, 3}}},
		{"single line", []string{"abc"}, 0, 2, 1, 20, false, [][2]int{{0, 2}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEditorWithLines(tt.lines...)
			e.WordWrap, e.TabSize, e.Viewport.Width = tt.wrap, 4, tt.width
			e.Cursor.Line, e.Cursor.Col = tt.line, tt.col
			for range len(tt.want) + 2 {
				e.addCursorVertical(tt.delta)
			}
			assertAddedCursors(t, e, tt.want...)
			assertCursor(t, e, tt.line, tt.col)
			if !reflect.DeepEqual(e.Buf.Lines, tt.lines) {
				t.Fatal("adding cursors changed the buffer")
			}
		})
	}
}

func TestAddCursorVerticalOppositeDirectionAndUndo(t *testing.T) {
	e := newEditorWithLines("abc", "abc", "abc", "abc")
	e.Cursor.Line, e.Cursor.Col = 1, 2
	e.addCursorVertical(1)
	e.addCursorVertical(-1)
	e.addCursorVertical(1)
	assertAddedCursors(t, e, [2]int{0, 2}, [2]int{1, 2}, [2]int{2, 2}, [2]int{3, 2})
	e.UndoLastCursor()
	assertAddedCursors(t, e, [2]int{0, 2}, [2]int{1, 2}, [2]int{2, 2})
	e.UndoLastCursor()
	assertAddedCursors(t, e, [2]int{1, 2}, [2]int{2, 2})
	e.UndoLastCursor()
	assertAddedCursors(t, e, [2]int{1, 2})
	if e.Multi != nil {
		t.Fatal("undoing all additions should leave a single cursor")
	}
}

func TestAddCursorVerticalSkipsFoldedLines(t *testing.T) {
	e := newEditorWithLines("abc", "hidden", "hidden", "def")
	e.Folds = fold.NewState()
	e.Folds.SetRanges([]fold.Range{{StartLine: 0, EndLine: 2}})
	e.Folds.Toggle(0)
	e.Cursor.Col = 2
	e.addCursorVertical(1)
	assertAddedCursors(t, e, [2]int{0, 2}, [2]int{3, 2})
	if !e.Folds.IsCollapsed(0) {
		t.Fatal("adding a cursor expanded the fold")
	}
}

func TestAddCursorVerticalSkipsPhantoms(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		e := newEditorWithLines("abc", "def")
		e.WordWrap = wrap
		e.setPhantoms(map[int]int{1: 2})
		e.Cursor.Col = 2
		e.addCursorVertical(1)
		assertAddedCursors(t, e, [2]int{0, 2}, [2]int{1, 2})
	}
}

func TestAddCursorVerticalPreservesSelectionAndScrolls(t *testing.T) {
	e := newEditorWithLines("abc", "abc", "abc", "abc")
	e.Viewport.Height = 2
	e.Cursor.Col = 2
	e.Selection = &selection.Selection{Active: true, Anchor: selection.Position{Line: 0, Col: 1}}
	want := *e.Selection
	for range 3 {
		e.addCursorVertical(1)
	}
	if *e.Selection != want || e.Multi.PrimaryCursor().Sel != want {
		t.Fatal("adding cursors changed the original selection")
	}
	for _, c := range e.Multi.Cursors[1:] {
		if c.Sel.Active {
			t.Fatal("new cursor should not select text")
		}
	}
	if e.Viewport.TopLine != 2 {
		t.Fatalf("top line = %d, want 2 to show the newest cursor", e.Viewport.TopLine)
	}
}
