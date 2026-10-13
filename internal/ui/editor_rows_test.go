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
	e.setPhantoms(map[int]int{2: 2, 10: 1})

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

func TestEditorRowsScrollbarDragWithWrapMapsRows(t *testing.T) {
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = fmt.Sprintf("%d%s", i, strings.Repeat("x", 25))
	}
	e := newRowsEditor(lines, 12, 5)
	e.WordWrap = true
	renderRows(e)
	sb := e.scrollbar
	e.HandleEvent(tcell.NewEventMouse(sb.X, sb.Y+sb.Height-1, tcell.Button1, tcell.ModNone))
	e.HandleEvent(tcell.NewEventMouse(sb.X, sb.Y+sb.Height-1, tcell.ButtonNone, tcell.ModNone))
	rows := renderRows(e)
	if e.Viewport.TopLine >= len(lines) || rows[0] == "" {
		t.Fatalf("scrollbar drag set TopLine %d of %d lines, rows %q", e.Viewport.TopLine, len(lines), rows)
	}
}

func TestEditorRowsExternalTopLineResetsRowOffset(t *testing.T) {
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = fmt.Sprintf("%d%s", i, strings.Repeat("x", 25))
	}
	e := newRowsEditor(lines, 12, 5)
	e.WordWrap = true
	renderRows(e)
	e.scrollDown(1)
	if rows := renderRows(e); strings.HasPrefix(rows[0], "0") {
		t.Fatalf("scrollDown(1) did not move inside line 0: %q", rows)
	}
	e.Viewport.TopLine = 5
	if rows := renderRows(e); !strings.HasPrefix(rows[0], "5") {
		t.Fatalf("after setting TopLine = 5 the first row is %q, want the start of line 5", rows[0])
	}
}

func TestEditorRowsCursorInsideCollapsedFoldShowsOnFoldLine(t *testing.T) {
	lines := []string{"a0", "b1", "  b2", "  b3", "c4"}
	e := newRowsEditor(lines, 20, 5)
	e.Folds = fold.NewState()
	e.Folds.SetRanges(fold.ComputeIndentRanges(lines))
	e.Folds.Toggle(1)
	e.Cursor.Line = 3
	renderRows(e)
	if e.CursorY != 1 {
		t.Fatalf("CursorY = %d for a cursor inside the fold at line 1, want 1", e.CursorY)
	}
}

func TestTopOffsetPastShrunkenTopLineKeepsCursorOnItsRow(t *testing.T) {
	e := newRowsEditor([]string{strings.Repeat("abcdefghij", 4), "second", "third"}, 14, 6)
	e.WordWrap = true
	renderRows(e)
	l := e.layout()
	if l.blockRows(0) < 3 {
		t.Fatalf("top line wraps to %d rows, want at least 3", l.blockRows(0))
	}
	e.setTopRow(l, 2)
	e.Cursor.Line = 1
	renderRows(e)

	e.SetRect(Rect{X: 0, Y: 0, W: 60, H: 6})
	renderRows(e)
	for y, row := range e.rowMap {
		if row.bufLine == 1 && !row.isPhantom() {
			if e.CursorY != y {
				t.Fatalf("cursor drawn at row %d, line 1 is at row %d", e.CursorY, y)
			}
			return
		}
	}
	t.Fatal("line 1 not on screen")
}

func TestTextPosAtMapsTabsFoldsAndWraps(t *testing.T) {
	type probe struct {
		x, y      int
		line, col int
		ok        bool
	}
	check := func(t *testing.T, e *EditorPaneWidget, probes []probe) {
		t.Helper()
		renderRows(e)
		g := e.GutterWidth()
		for _, p := range probes {
			line, col, ok := e.TextPosAt(g+p.x, p.y)
			if ok != p.ok || (ok && (line != p.line || col != p.col)) {
				t.Fatalf("TextPosAt(%d,%d) = (%d,%d,%v), want (%d,%d,%v)", p.x, p.y, line, col, ok, p.line, p.col, p.ok)
			}
		}
	}

	t.Run("tabs", func(t *testing.T) {
		e := newRowsEditor([]string{"\tx := 1"}, 30, 4)
		check(t, e, []probe{
			{x: 0, y: 0, line: 0, col: 0, ok: true},
			{x: 3, y: 0, line: 0, col: 0, ok: true},
			{x: 4, y: 0, line: 0, col: 1, ok: true},
			{x: 9, y: 0, line: 0, col: 6, ok: true},
			{x: 10, y: 0},
			{x: 0, y: 1},
			{x: -1, y: 0},
		})
	})

	t.Run("folds", func(t *testing.T) {
		lines := []string{"a0", "  a1", "  a2", "b3"}
		e := newRowsEditor(lines, 20, 4)
		e.Folds = fold.NewState()
		e.Folds.SetRanges(fold.ComputeIndentRanges(lines))
		e.Folds.Toggle(0)
		check(t, e, []probe{{x: 1, y: 1, line: 3, col: 1, ok: true}})
	})

	t.Run("wrap", func(t *testing.T) {
		e := newRowsEditor([]string{"short", "aaaaaaaaaabbbbbbbbbbcc", "end"}, 12, 6)
		e.WordWrap = true
		check(t, e, []probe{
			{x: 3, y: 2, line: 1, col: 12, ok: true},
			{x: 1, y: 3, line: 1, col: 19, ok: true},
			{x: 4, y: 3},
			{x: 0, y: 4, line: 2, col: 0, ok: true},
		})
	})
}
