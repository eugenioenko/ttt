package render

import (
	"github.com/eugenioenko/ttt/internal/term"
	"testing"
	"unsafe"
)

func makeCells(rows ...string) [][]term.Cell {
	out := make([][]term.Cell, len(rows))
	for y, row := range rows {
		out[y] = make([]term.Cell, len(row))
		for x, ch := range row {
			out[y][x] = term.Cell{Ch: ch}
		}
	}
	return out
}

func TestRenderer_RenderDiff(t *testing.T) {
	r := &Renderer{}
	screen := term.NewMockScreen(5, 2)
	r.SetCurrent(makeCells("abcde", "fghij"))
	r.Render(screen)
	// All cells should be set
	for y, row := range []string{"abcde", "fghij"} {
		for x, ch := range row {
			c, ok := screen.Cells[[2]int{x, y}]
			if !ok || c.Ch != ch {
				t.Errorf("expected cell (%d,%d) to be %c", x, y, ch)
			}
		}
	}
	// Change one cell
	r.SetCurrent(makeCells("abxde", "fghij"))
	r.Render(screen)
	c, ok := screen.Cells[[2]int{2, 0}]
	if !ok || c.Ch != 'x' {
		t.Errorf("expected cell (2,0) to be 'x'")
	}
}

func TestRenderer_RenderNoCopy(t *testing.T) {
	r := &Renderer{}
	screen := term.NewMockScreen(5, 2)
	cells := makeCells("abcde", "fghij")
	r.SetCurrent(cells)
	r.Render(screen)

	for y := range cells {
		if unsafe.SliceData(r.prev[y]) != unsafe.SliceData(cells[y]) {
			t.Errorf("row %d: r.prev does not share cells' backing array; Render is copying instead of retaining the reference", y)
		}
	}
}

func TestRenderer_Clear(t *testing.T) {
	r := &Renderer{}
	r.SetCurrent(makeCells("abc"))
	r.Clear()
	if r.prev != nil || r.curr != nil {
		t.Error("expected buffers to be nil after Clear")
	}
}
