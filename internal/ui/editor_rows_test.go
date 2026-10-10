package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

func newRowsEditor(lines []string, w, h int) *EditorPaneWidget {
	buf := &buffer.Buffer{Lines: lines}
	e := NewEditorPaneWidget(buf, &cursor.Cursor{}, &view.Viewport{Width: w, Height: h})
	e.Selection = &selection.Selection{}
	e.SetRect(Rect{X: 0, Y: 0, W: w, H: h})
	return e
}

func renderRows(e *EditorPaneWidget) []string {
	r := e.GetRect()
	grid := makeGrid(r.W, r.H)
	e.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: r.W, H: r.H}))
	gutterW := e.GutterWidth()
	rows := make([]string, r.H)
	for y := 0; y < r.H; y++ {
		var sb strings.Builder
		for x := gutterW; x < gutterW+e.Viewport.Width && x < r.W; x++ {
			sb.WriteRune(grid[y][x].Ch)
		}
		rows[y] = strings.TrimRight(sb.String(), " ")
	}
	return rows
}

func clickAt(e *EditorPaneWidget, x, y int) {
	e.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	e.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
}

func numberedLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	return lines
}

func TestEditorRowsPlain(t *testing.T) {
	e := newRowsEditor(numberedLines(20), 20, 5)
	renderRows(e)

	e.scrollDown(3)
	rows := renderRows(e)
	if rows[0] != "line 3" || rows[4] != "line 7" {
		t.Fatalf("rows after scrollDown(3) = %q", rows)
	}

	e.Cursor.Line = 5
	renderRows(e)
	if e.CursorY != 2 {
		t.Fatalf("CursorY = %d, want 2", e.CursorY)
	}

	clickAt(e, e.GutterWidth()+2, 1)
	if e.Cursor.Line != 4 || e.Cursor.Col != 2 {
		t.Fatalf("click = (%d,%d), want (4,2)", e.Cursor.Line, e.Cursor.Col)
	}

	e.scrollDown(100)
	if e.Viewport.TopLine != 19 {
		t.Fatalf("TopLine after scrollDown(100) = %d, want 19", e.Viewport.TopLine)
	}
	rows = renderRows(e)
	if rows[0] != "line 19" || rows[1] != "" {
		t.Fatalf("rows at end = %q", rows)
	}

	e.scrollUp(100)
	if e.Viewport.TopLine != 0 {
		t.Fatalf("TopLine after scrollUp(100) = %d, want 0", e.Viewport.TopLine)
	}

	e.Cursor.Line = 10
	e.scrollViewport()
	if e.Viewport.TopLine != 6 {
		t.Fatalf("TopLine after reveal = %d, want 6", e.Viewport.TopLine)
	}
	renderRows(e)
	if e.CursorY != 4 {
		t.Fatalf("CursorY = %d, want 4", e.CursorY)
	}
}

func TestEditorRowsFolds(t *testing.T) {
	lines := []string{"a0", "  a1", "  a2", "b3", "  b4", "c5"}
	e := newRowsEditor(lines, 20, 5)
	e.Folds = fold.NewState()
	e.Folds.SetRanges(fold.ComputeIndentRanges(lines))
	e.Folds.Toggle(0)

	rows := renderRows(e)
	want := []string{"a0 ⋯", "b3", "  b4", "c5", ""}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("folded rows = %q, want %q", rows, want)
		}
	}

	clickAt(e, e.GutterWidth()+1, 1)
	if e.Cursor.Line != 3 || e.Cursor.Col != 1 {
		t.Fatalf("click = (%d,%d), want (3,1)", e.Cursor.Line, e.Cursor.Col)
	}

	e.Cursor.Line = 5
	renderRows(e)
	if e.CursorY != 3 {
		t.Fatalf("CursorY = %d, want 3", e.CursorY)
	}

	e.Cursor.Line, e.Cursor.Col = 0, 0
	e.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", 0))
	if e.Cursor.Line != 3 {
		t.Fatalf("down over fold = %d, want 3", e.Cursor.Line)
	}

	e.scrollDown(1)
	if e.Viewport.TopLine != 3 {
		t.Fatalf("TopLine after scrollDown(1) = %d, want 3", e.Viewport.TopLine)
	}
	e.scrollUp(1)
	if e.Viewport.TopLine != 0 {
		t.Fatalf("TopLine after scrollUp(1) = %d, want 0", e.Viewport.TopLine)
	}
}

func TestEditorRowsWordWrap(t *testing.T) {
	lines := []string{"short", "aaaaaaaaaabbbbbbbbbbcc", "end"}
	e := newRowsEditor(lines, 12, 6)
	e.WordWrap = true

	rows := renderRows(e)
	if e.Viewport.Width != 9 {
		t.Fatalf("wrap width = %d, want 9", e.Viewport.Width)
	}
	want := []string{"short", "aaaaaaaaa", "abbbbbbbb", "bbcc", "end", ""}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("wrapped rows = %q, want %q", rows, want)
		}
	}

	clickAt(e, e.GutterWidth()+3, 2)
	if e.Cursor.Line != 1 || e.Cursor.Col != 12 {
		t.Fatalf("click = (%d,%d), want (1,12)", e.Cursor.Line, e.Cursor.Col)
	}

	e.Cursor.Line, e.Cursor.Col = 1, 20
	renderRows(e)
	if e.CursorY != 3 || e.CursorX != e.GutterWidth()+2 {
		t.Fatalf("cursor screen = (%d,%d), want (%d,3)", e.CursorX, e.CursorY, e.GutterWidth()+2)
	}

	e.Cursor.Line, e.Cursor.Col = 2, 0
	renderRows(e)
	if e.CursorY != 4 {
		t.Fatalf("CursorY = %d, want 4", e.CursorY)
	}

	e.scrollDown(2)
	rows = renderRows(e)
	if e.Viewport.TopLine != 1 || rows[0] != "abbbbbbbb" {
		t.Fatalf("after scrollDown(2) top=%d rows=%q", e.Viewport.TopLine, rows)
	}

	e.scrollUp(1)
	rows = renderRows(e)
	if rows[0] != "aaaaaaaaa" {
		t.Fatalf("after scrollUp(1) rows=%q", rows)
	}
}

func TestEditorRowsPhantoms(t *testing.T) {
	e := newRowsEditor(numberedLines(10), 20, 5)
	e.phantoms = map[int]int{2: 2, 10: 1}

	rows := renderRows(e)
	want := []string{"line 0", "line 1", "", "", "line 2"}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("rows = %q, want %q", rows, want)
		}
	}
	if !e.rowAt(2).isPhantom() || e.rowAt(4).isPhantom() {
		t.Fatalf("phantom flags wrong: %+v %+v", e.rowAt(2), e.rowAt(4))
	}

	e.Cursor.Line = 1
	e.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", 0))
	renderRows(e)
	if e.Cursor.Line != 2 || e.CursorY != 4 {
		t.Fatalf("cursor after down = line %d y %d, want line 2 y 4", e.Cursor.Line, e.CursorY)
	}

	clickAt(e, e.GutterWidth()+1, 3)
	if e.Cursor.Line != 2 {
		t.Fatalf("click on phantom = line %d, want 2", e.Cursor.Line)
	}

	e.scrollDown(2)
	rows = renderRows(e)
	if e.Viewport.TopLine != 2 || rows[0] != "" || rows[2] != "line 2" {
		t.Fatalf("after scrollDown(2) top=%d rows=%q", e.Viewport.TopLine, rows)
	}

	e.scrollDown(100)
	rows = renderRows(e)
	if e.Viewport.TopLine != 9 || rows[0] != "" || !e.rowAt(0).isPhantom() {
		t.Fatalf("after scrollDown(100) top=%d rows=%q", e.Viewport.TopLine, rows)
	}

	e.Viewport.TopLine = 0
	e.Cursor.Line = 9
	e.scrollViewport()
	renderRows(e)
	if e.CursorY != 4 {
		t.Fatalf("CursorY after reveal = %d, want 4", e.CursorY)
	}
	if got := e.layout().total(); got != 13 {
		t.Fatalf("total rows = %d, want 13", got)
	}
}
