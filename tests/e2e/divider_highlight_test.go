package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

func (h *testHarness) frame() [][]term.Cell {
	h.t.Helper()
	cells := h.renderer.NextFrame(h.app.Root.Width, h.app.Root.Height)
	h.app.Root.Render(cells)
	return cells
}

func (h *testHarness) mouseAt(x, y int, btn tcell.ButtonMask) {
	h.t.Helper()
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, btn, tcell.ModNone))
	h.flushOnChange()
	h.redraw()
}

func (h *testHarness) setHighlightDividers(enabled bool) {
	h.t.Helper()
	s := *h.app.Settings
	s.Appearance.HighlightDividers = enabled
	h.app.ApplySettings(s)
	h.redraw()
}

type cellPos struct{ x, y int }

func hoverCells(cells [][]term.Cell) []cellPos {
	var found []cellPos
	for y, row := range cells {
		for x, c := range row {
			if c.Style == term.StyleBorderHover {
				found = append(found, cellPos{x, y})
			}
		}
	}
	return found
}

func assertHoverColumn(t *testing.T, cells [][]term.Cell, x, fromY, toY int) {
	t.Helper()
	got := hoverCells(cells)
	if len(got) == 0 {
		t.Fatal("no divider cells were highlighted")
	}
	for _, p := range got {
		if p.x != x || p.y < fromY || p.y > toY {
			t.Fatalf("highlighted cell %v is off the divider column x=%d rows %d..%d", p, x, fromY, toY)
		}
	}
	for y := fromY + 1; y < toY; y++ {
		if c := cells[y][x]; c.Ch != 0 && c.Ch != ' ' && c.Style != term.StyleBorderHover {
			t.Fatalf("divider cell (%d,%d) %q kept style %v", x, y, c.Ch, c.Style)
		}
	}
}

func assertHoverRow(t *testing.T, cells [][]term.Cell, y, fromX, toX int) {
	t.Helper()
	got := hoverCells(cells)
	if len(got) == 0 {
		t.Fatal("no divider cells were highlighted")
	}
	for _, p := range got {
		if p.y != y || p.x < fromX || p.x > toX {
			t.Fatalf("highlighted cell %v is off the divider row y=%d cols %d..%d", p, y, fromX, toX)
		}
	}
}

func assertNoHover(t *testing.T, cells [][]term.Cell) {
	t.Helper()
	if got := hoverCells(cells); len(got) != 0 {
		t.Fatalf("%d cells highlighted, first at %v", len(got), got[0])
	}
}

func TestSidebarDividerHighlightIsOptIn(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	sp := h.app.SplitPanel
	divX := sp.DividerScreenX()
	r := sp.GetRect()

	h.mouseMove(divX, r.Y+10)
	assertNoHover(t, h.frame())

	h.setHighlightDividers(true)
	assertHoverColumn(t, h.frame(), divX, r.Y, r.Y+r.H-1)

	h.mouseMove(divX+10, r.Y+10)
	assertNoHover(t, h.frame())

	h.mouseMove(divX, r.Y+10)
	h.setHighlightDividers(false)
	assertNoHover(t, h.frame())
}

func TestDraggedPanelDividerStaysHighlightedUntilRelease(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.setHighlightDividers(true)
	h.app.ShowBottomPanel()
	h.app.BottomPanel.SetActivePanel("output")
	h.redraw()
	cs := h.app.ContentSplit
	r := cs.GetRect()
	y := cs.DividerScreenY()

	h.mouseMove(60, y)
	assertHoverRow(t, h.frame(), y, r.X, r.X+r.W-1)

	h.mouseAt(60, y, tcell.Button1)
	h.mouseAt(5, y-3, tcell.Button1)
	if !cs.Dragging() {
		t.Fatal("drag was canceled mid-drag")
	}
	if cs.OverDivider(5, y-3) {
		t.Fatal("the pointer must be off the divider line")
	}
	assertHoverRow(t, h.frame(), y-3, r.X, r.X+r.W-1)

	h.mouseAt(5, y-3, tcell.ButtonNone)
	assertNoHover(t, h.frame())
}

func TestEditorSplitDividersHighlightOnlyBorderCells(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	h.setHighlightDividers(true)
	h.exec("editor.split")
	split := h.app.EditorSplit.Split
	divX := split.DividerScreenX()
	r := split.GetRect()

	h.mouseMove(divX, r.Y+r.H/2)
	cells := h.frame()
	assertHoverColumn(t, cells, divX, r.Y, r.Y+r.H-1)

	h.exec("editor.splitDown")
	down := h.app.EditorSplit.Second.Split
	if down == nil || down.Position != ui.SplitBottom {
		t.Fatal("split down did not nest a bottom split in the right group")
	}
	y := down.DividerScreenY()
	dr := down.GetRect()
	h.mouseMove(dr.X+dr.W-5, y)
	cells = h.frame()
	assertHoverRow(t, cells, y, dr.X, dr.X+dr.W-1)
	tab := h.app.EditorGroup.TabBar.GetRect()
	for x := tab.X; x < tab.X+tab.W; x++ {
		if c := cells[tab.Y+1][x]; c.Style == term.StyleBorderHover {
			t.Fatalf("tab label row cell %d was restyled", x)
		}
	}
}

func TestDividerHighlightYieldsToOverlaysAndOtherCaptures(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.setHighlightDividers(true)
	sp := h.app.SplitPanel
	divX := sp.DividerScreenX()
	r := sp.GetRect()

	h.exec("command.palette")
	h.mouseMove(divX, r.Y+r.H-3)
	assertNoHover(t, h.frame())
	h.pressKey(tcell.KeyEscape, 0)

	editor := h.app.EditorGroup.Editor.GetRect()
	h.mouseAt(editor.X+editor.W/2, editor.Y+2, tcell.Button1)
	h.mouseAt(divX, editor.Y+2, tcell.Button1)
	assertNoHover(t, h.frame())
	h.mouseAt(divX, editor.Y+2, tcell.ButtonNone)
	assertHoverColumn(t, h.frame(), divX, r.Y, r.Y+r.H-1)
}
