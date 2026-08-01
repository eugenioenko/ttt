package ui

import "testing"

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
