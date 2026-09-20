package render

import "github.com/eugenioenko/ttt/internal/term"

// Renderer handles diff-based rendering to the terminal screen.
type Renderer struct {
	prev  [][]term.Cell
	curr  [][]term.Cell
	spare [][]term.Cell
}

// NextFrame returns a cleared w x h grid to draw the next frame into. Frames
// alternate between two grids: one is the previous frame, kept to diff against,
// and the other is drawn into, so a grid is only reallocated when the size changes.
func (r *Renderer) NextFrame(w, h int) [][]term.Cell {
	grid := r.spare
	if len(grid) != h || (h > 0 && len(grid[0]) != w) {
		grid = make([][]term.Cell, h)
		for y := range grid {
			grid[y] = make([]term.Cell, w)
		}
		r.spare = grid
	} else {
		for y := range grid {
			clear(grid[y])
		}
	}
	r.curr = grid
	return grid
}

// Render diffs the grid from NextFrame against the previous frame and emits minimal
// updates to the screen.
func (r *Renderer) Render(screen term.Screen) {
	for y, row := range r.curr {
		for x, cell := range row {
			if r.prev == nil || y >= len(r.prev) || x >= len(r.prev[y]) || r.prev[y][x] != cell {
				screen.SetCell(x, y, cell)
			}
		}
	}
	screen.Show()
	r.spare, r.prev = r.prev, r.curr
}

// Clear resets the renderer's buffers.
func (r *Renderer) Clear() {
	r.prev = nil
	r.curr = nil
	r.spare = nil
}
