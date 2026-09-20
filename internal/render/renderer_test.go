package render

import (
	"github.com/eugenioenko/ttt/internal/term"
	"testing"
	"unsafe"
)

func nextFrame(r *Renderer, rows ...string) [][]term.Cell {
	out := r.NextFrame(len(rows[0]), len(rows))
	for y, row := range rows {
		for x, ch := range row {
			out[y][x] = term.Cell{Ch: ch}
		}
	}
	return out
}

func TestRenderer_RenderDiff(t *testing.T) {
	r := &Renderer{}
	screen := term.NewMockScreen(5, 2)
	nextFrame(r, "abcde", "fghij")
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
	nextFrame(r, "abxde", "fghij")
	r.Render(screen)
	c, ok := screen.Cells[[2]int{2, 0}]
	if !ok || c.Ch != 'x' {
		t.Errorf("expected cell (2,0) to be 'x'")
	}
}

func TestRenderer_RenderNoCopy(t *testing.T) {
	r := &Renderer{}
	screen := term.NewMockScreen(5, 2)
	cells := nextFrame(r, "abcde", "fghij")
	r.Render(screen)

	for y := range cells {
		if unsafe.SliceData(r.prev[y]) != unsafe.SliceData(cells[y]) {
			t.Errorf("row %d: r.prev does not share cells' backing array; Render is copying instead of retaining the reference", y)
		}
	}
}

func TestRenderer_NextFrameAlternatesTwoGrids(t *testing.T) {
	r := &Renderer{}
	screen := term.NewMockScreen(3, 1)

	first := nextFrame(r, "abc")
	r.Render(screen)
	second := nextFrame(r, "abd")
	if unsafe.SliceData(second[0]) == unsafe.SliceData(first[0]) {
		t.Fatal("NextFrame handed out the grid held as the previous frame")
	}
	r.Render(screen)

	third := r.NextFrame(3, 1)
	if unsafe.SliceData(third[0]) != unsafe.SliceData(first[0]) {
		t.Error("third frame should reuse the first frame's grid")
	}
	if third[0][0] != (term.Cell{}) || third[0][2] != (term.Cell{}) {
		t.Error("reused grid was not cleared")
	}
}

func TestRenderer_NextFrameReallocatesOnResize(t *testing.T) {
	r := &Renderer{}
	screen := term.NewMockScreen(6, 2)
	nextFrame(r, "abc")
	r.Render(screen)
	nextFrame(r, "abc")
	r.Render(screen)

	grid := r.NextFrame(6, 2)
	if len(grid) != 2 || len(grid[0]) != 6 {
		t.Fatalf("grid is %dx%d, want 6x2", len(grid[0]), len(grid))
	}
}

func TestRenderer_Clear(t *testing.T) {
	r := &Renderer{}
	nextFrame(r, "abc")
	r.Clear()
	if r.prev != nil || r.curr != nil || r.spare != nil {
		t.Error("expected buffers to be nil after Clear")
	}
}
