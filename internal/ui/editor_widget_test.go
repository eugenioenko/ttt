package ui

import (
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func newTestEditor() *EditorPaneWidget {
	buf := &buffer.Buffer{Lines: []string{"Hello", "World", "Test"}}
	cur := &cursor.Cursor{Line: 0, Col: 0}
	vp := &view.Viewport{TopLine: 0, LeftCol: 0, Width: 20, Height: 10}
	return NewEditorPaneWidget(buf, cur, vp)
}

func TestEditorRender(t *testing.T) {
	e := newTestEditor()
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})

	grid := makeGrid(20, 10)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 10})
	e.Render(surface)

	g := e.GutterWidth()
	if grid[0][g].Ch != 'H' {
		t.Fatalf("expected 'H' at (%d,0), got '%c'", g, grid[0][g].Ch)
	}
	if grid[1][g].Ch != 'W' {
		t.Fatalf("expected 'W' at (%d,1), got '%c'", g, grid[1][g].Ch)
	}
	// Line past buffer should be empty
	if grid[3][g].Ch != ' ' {
		t.Fatalf("expected ' ' at (%d,3), got '%c'", g, grid[3][g].Ch)
	}
}

func TestEditorArrowKeys(t *testing.T) {
	e := newTestEditor()
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})

	e.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", 0))
	if e.Cursor.Line != 1 {
		t.Fatalf("expected line 1, got %d", e.Cursor.Line)
	}

	e.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", 0))
	if e.Cursor.Col != 1 {
		t.Fatalf("expected col 1, got %d", e.Cursor.Col)
	}

	e.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", 0))
	if e.Cursor.Line != 0 {
		t.Fatalf("expected line 0, got %d", e.Cursor.Line)
	}
}

func TestEditorTypeCharacter(t *testing.T) {
	e := newTestEditor()
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})

	e.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "X", 0))
	if e.Buf.Lines[0] != "XHello" {
		t.Fatalf("expected 'XHello', got '%s'", e.Buf.Lines[0])
	}
	if e.Cursor.Col != 1 {
		t.Fatalf("expected col 1, got %d", e.Cursor.Col)
	}
}

func TestEditorCursorPosition(t *testing.T) {
	e := newTestEditor()
	e.SetRect(Rect{X: 5, Y: 3, W: 20, H: 10})
	e.Cursor.Line = 1
	e.Cursor.Col = 2

	grid := makeGrid(25, 13)
	surface := NewRenderSurface(grid, Rect{X: 5, Y: 3, W: 20, H: 10})
	e.Render(surface)

	expectedX := 5 + e.GutterWidth() + 2
	if e.CursorX != expectedX {
		t.Fatalf("expected CursorX %d, got %d", expectedX, e.CursorX)
	}
	if e.CursorY != 4 {
		t.Fatalf("expected CursorY 4, got %d", e.CursorY)
	}
}

func TestEditorFocusable(t *testing.T) {
	e := newTestEditor()
	if !e.Focusable() {
		t.Fatal("editor should be focusable")
	}
}

func TestEditorHomeEnd(t *testing.T) {
	e := newTestEditor()
	e.Cursor.Col = 3

	e.HandleEvent(tcell.NewEventKey(tcell.KeyHome, "", 0))
	if e.Cursor.Col != 0 {
		t.Fatalf("Home: expected col 0, got %d", e.Cursor.Col)
	}

	e.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", 0))
	if e.Cursor.Col != 5 {
		t.Fatalf("End: expected col 5, got %d", e.Cursor.Col)
	}
}

func TestEditorBackspace(t *testing.T) {
	e := newTestEditor()
	e.Cursor.Col = 2

	e.HandleEvent(tcell.NewEventKey(tcell.KeyBackspace2, "", 0))
	if e.Buf.Lines[0] != "Hllo" {
		t.Fatalf("expected 'Hllo', got '%s'", e.Buf.Lines[0])
	}
}

func TestEditorEnter(t *testing.T) {
	e := newTestEditor()
	e.Cursor.Col = 3

	e.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	if e.Buf.Lines[0] != "Hel" {
		t.Fatalf("expected 'Hel', got '%s'", e.Buf.Lines[0])
	}
	if e.Buf.Lines[1] != "lo" {
		t.Fatalf("expected 'lo', got '%s'", e.Buf.Lines[1])
	}
	if e.Cursor.Line != 1 || e.Cursor.Col != 0 {
		t.Fatalf("expected cursor at (1,0), got (%d,%d)", e.Cursor.Line, e.Cursor.Col)
	}
}

func TestEditorSelectionHighlight(t *testing.T) {
	e := newTestEditor()
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})
	e.Selection = &selection.Selection{}

	// Select "ell" in "Hello" via Shift+Right from col 1
	e.Cursor.Col = 1
	e.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModShift))
	e.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModShift))
	e.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModShift))

	if !e.Selection.Active {
		t.Fatal("selection should be active")
	}

	grid := makeGrid(20, 10)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 10})
	e.Render(surface)

	// Cols 1,2,3 (offset by gutter) should have BgStyle=StyleSelection
	g := e.GutterWidth()
	for col := 1; col <= 3; col++ {
		if grid[0][g+col].BgStyle != term.StyleSelection {
			t.Errorf("col %d: expected BgStyle=StyleSelection, got %d", col, grid[0][g+col].BgStyle)
		}
	}
	// Col 0 and col 4 should NOT be selected
	if grid[0][g+0].BgStyle == term.StyleSelection {
		t.Error("col 0 should not be selected")
	}
	if grid[0][g+4].BgStyle == term.StyleSelection {
		t.Error("col 4 should not be selected")
	}
}

func TestEditorSyntaxSearchAndSelectionLayering(t *testing.T) {
	e := NewEditorPaneWidget(
		&buffer.Buffer{Lines: []string{"func main() {}"}},
		&cursor.Cursor{Line: 0, Col: 0},
		&view.Viewport{TopLine: 0, LeftCol: 0, Width: 20, Height: 10},
	)
	e.Highlighter = highlight.New("main.go")
	e.Selection = &selection.Selection{Active: true, Anchor: selection.Position{Line: 0, Col: 0}}
	e.Cursor.Line = 0
	e.Cursor.Col = 4
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 4})

	grid := makeGrid(20, 4)
	e.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 4}))
	cell := grid[0][e.GutterWidth()]
	if cell.Style != term.StyleSyntaxKeyword || cell.BgStyle != term.StyleSelection {
		t.Fatalf("selected syntax cell = %+v, want keyword foreground with selection background", cell)
	}

	e.Selection.Active = false
	e.SearchMatches = []FindMatch{{Line: 0, Col: 0, Len: 4}}
	e.SearchActive = 0
	e.buildSearchIndex()
	grid = makeGrid(20, 4)
	e.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 4}))
	cell = grid[0][e.GutterWidth()]
	if cell.Style != term.StyleSearchActive || cell.BgStyle != 0 {
		t.Fatalf("searched syntax cell = %+v, want active search style with no layered background", cell)
	}
}

func TestEditorLineNumbers(t *testing.T) {
	e := newTestEditor()
	e.LineNumbers = true
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})

	grid := makeGrid(20, 10)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 10})
	e.Render(surface)

	g := e.GutterWidth()
	// First col should be gutter padding
	if grid[0][0].Ch != ' ' {
		t.Fatalf("expected left pad space at (0,0), got '%c'", grid[0][0].Ch)
	}
	// Line number "1" is right-aligned before the right padding
	numCol := g - 3 // 1 left pad + digits, number ends at gutterW - 2 (right pad)
	if grid[0][numCol].Ch != '1' {
		t.Fatalf("expected '1' at (0,%d), got '%c'", numCol, grid[0][numCol].Ch)
	}
	// Text starts after gutter
	if grid[0][g].Ch != 'H' {
		t.Fatalf("expected 'H' at (0,%d), got '%c'", g, grid[0][g].Ch)
	}
	// Line 2
	if grid[1][numCol].Ch != '2' {
		t.Fatalf("expected '2' at (1,%d), got '%c'", numCol, grid[1][numCol].Ch)
	}
	// Gutter style on non-active line
	if grid[1][numCol].Style != term.StyleLineNumber {
		t.Errorf("expected StyleLineNumber for gutter, got %d", grid[1][numCol].Style)
	}
	// Active line gutter gets StyleActiveLine
	if grid[0][numCol].Style != term.StyleActiveLine {
		t.Errorf("expected StyleActiveLine for active line gutter, got %d", grid[0][numCol].Style)
	}
}

func TestEditorLineNumbersCursorOffset(t *testing.T) {
	e := newTestEditor()
	e.LineNumbers = true
	e.SetRect(Rect{X: 5, Y: 3, W: 20, H: 10})
	e.Cursor.Line = 0
	e.Cursor.Col = 2

	grid := makeGrid(25, 13)
	surface := NewRenderSurface(grid, Rect{X: 5, Y: 3, W: 20, H: 10})
	e.Render(surface)

	expectedX := 5 + e.GutterWidth() + 2
	if e.CursorX != expectedX {
		t.Fatalf("expected CursorX %d, got %d", expectedX, e.CursorX)
	}
}

func TestHorizontalScrollClampNoOverflow(t *testing.T) {
	e := newTestEditor()
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})
	e.Viewport.Width = 20

	ev := tcell.NewEventMouse(10, 5, tcell.WheelRight, tcell.ModNone)
	e.HandleEvent(ev)

	if e.Viewport.LeftCol != 0 {
		t.Errorf("expected LeftCol 0 when content fits, got %d", e.Viewport.LeftCol)
	}
}

func TestHorizontalScrollClampShiftWheel(t *testing.T) {
	e := newTestEditor()
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})
	e.Viewport.Width = 20

	ev := tcell.NewEventMouse(10, 5, tcell.WheelDown, tcell.ModShift)
	e.HandleEvent(ev)

	if e.Viewport.LeftCol != 0 {
		t.Errorf("expected LeftCol 0 when content fits with shift+wheel, got %d", e.Viewport.LeftCol)
	}
}

func TestHorizontalScrollAllowedWhenOverflow(t *testing.T) {
	buf := &buffer.Buffer{Lines: []string{"a]very long line that definitely overflows the viewport width of twenty chars"}}
	cur := &cursor.Cursor{Line: 0, Col: 0}
	vp := &view.Viewport{TopLine: 0, LeftCol: 0, Width: 20, Height: 10}
	e := NewEditorPaneWidget(buf, cur, vp)
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})

	ev := tcell.NewEventMouse(10, 5, tcell.WheelRight, tcell.ModNone)
	e.HandleEvent(ev)

	if e.Viewport.LeftCol == 0 {
		t.Error("expected LeftCol > 0 when content overflows")
	}
}

// ensure term import is used
var _ = term.Cell{}

func TestTransformSelectionStaleAnchorBeyondLine(t *testing.T) {
	// Regression (chaos crash): anchor column beyond the line length must not panic.
	e := newTestEditor()
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 34) // anchor past end of "Hello" (5 runes)
	e.Cursor.Line = 1
	e.Cursor.Col = 2

	e.UpperCase() // must not panic

	if e.Buf.Lines[1][:2] != "WO" {
		t.Errorf("expected transformed text, got %q", e.Buf.Lines[1])
	}
}
