package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

func testMenuBar() *MenuBarWidget {
	return NewMenuBarWidget([]MenuItem{{Name: "File"}, {Name: "Edit"}, {Name: "Options"}})
}

// A hidden menu bar is never rendered, but its shortcuts still open dropdowns —
// which need an anchor. Spans must therefore be derivable without a draw.
func TestItemAnchorXBeforeRender(t *testing.T) {
	m := testMenuBar()

	anchor := m.ItemAnchorX(2)

	rendered := testMenuBar()
	rendered.SetRect(Rect{X: 0, Y: 0, W: 80, H: 1})
	rendered.Render(NewRenderSurface(makeGrid(80, 1), Rect{X: 0, Y: 0, W: 80, H: 1}))

	if want := rendered.ItemAnchorX(2); anchor != want {
		t.Errorf("anchor before render = %d, want %d (same as after render)", anchor, want)
	}
}

// The anchor must point at the item's own label, otherwise dropdowns open under
// the wrong menu.
func TestItemAnchorXMatchesRenderedLabel(t *testing.T) {
	m := testMenuBar()
	cells := makeGrid(80, 1)
	m.SetRect(Rect{X: 0, Y: 0, W: 80, H: 1})
	m.Render(NewRenderSurface(cells, Rect{X: 0, Y: 0, W: 80, H: 1}))

	// Items are drawn as " Name ", so the label starts one cell past the anchor.
	for i, item := range m.Items {
		start := m.ItemAnchorX(i) + 1
		label := make([]rune, 0, len(item.Name))
		for _, cell := range cells[0][start : start+len([]rune(item.Name))] {
			label = append(label, cell.Ch)
		}
		if got := string(label); got != item.Name {
			t.Errorf("item %d: anchor points at %q, want %q", i, got, item.Name)
		}
	}
}

func TestItemAnchorXOutOfRange(t *testing.T) {
	m := testMenuBar()
	if got := m.ItemAnchorX(-1); got != 0 {
		t.Errorf("negative index anchor = %d, want 0", got)
	}
	if got := m.ItemAnchorX(99); got != 0 {
		t.Errorf("out-of-range index anchor = %d, want 0", got)
	}
}

func TestMenuBarWideLabelsRenderAndClick(t *testing.T) {
	m := NewMenuBarWidget([]MenuItem{{Name: "文件"}, {Name: "编辑"}, {Name: "Help"}})
	r := Rect{X: 3, Y: 2, W: 40, H: 1}
	m.SetRect(r)
	anchor := m.ItemAnchorX(1)
	cells := makeGrid(50, 4)
	m.Render(NewRenderSurface(cells, r))
	if anchor != m.ItemAnchorX(1) {
		t.Fatal("render changed the menu anchor")
	}
	for i, item := range m.Items {
		x := r.X + m.ItemAnchorX(i) + 1
		for _, ch := range item.Name {
			if got := cells[r.Y][x].Ch; got != ch {
				t.Errorf("item %d column %d = %q, want %q", i, x, got, ch)
			}
			x += textwidth.Rune(ch)
		}
	}
	selected := -1
	m.OnSelect = func(index int) { selected = index }
	for i, span := range m.ItemSpans() {
		for x := span.Start; x < span.End; x++ {
			m.HandleEvent(tcell.NewEventMouse(r.X+x, r.Y, tcell.Button1, 0))
			if selected != i {
				t.Fatalf("click at column %d selected %d, want %d", x, selected, i)
			}
			m.HandleEvent(tcell.NewEventMouse(r.X+x, r.Y, tcell.ButtonNone, 0))
		}
	}
}

func TestMenuBarClipsWideLabelAtEdge(t *testing.T) {
	m := NewMenuBarWidget([]MenuItem{{Name: "文件"}})
	cells := makeGrid(3, 1)
	r := Rect{W: 3, H: 1}
	m.SetRect(r)
	m.Render(NewRenderSurface(cells, r))
	if got := cells[0][2].Ch; got != ' ' {
		t.Fatalf("last column = %q, want a space instead of a clipped fullwidth rune", got)
	}
}
