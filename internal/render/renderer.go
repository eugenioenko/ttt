package render

import "github.com/eugenioenko/ttt/internal/term"

// Renderer handles diff-based rendering to the terminal screen.
type Renderer struct {
	prev [][]term.Cell
	curr [][]term.Cell
}

// SetCurrent sets the current buffer to render.
func (r *Renderer) SetCurrent(cells [][]term.Cell) {
	r.curr = cells
}

// Render diffs curr vs prev and emits minimal updates to the screen.
func (r *Renderer) Render(screen term.Screen) {
	for y, row := range r.curr {
		for x, cell := range row {
			if r.prev == nil || y >= len(r.prev) || x >= len(r.prev[y]) || r.prev[y][x] != cell {
				screen.SetCell(x, y, cell)
			}
		}
	}
	screen.Show()
	// No copy needed: callers always pass a freshly allocated grid to
	// SetCurrent each frame, so retaining the reference is safe and avoids
	// a full-grid alloc+copy on every render.
	r.prev = r.curr
}

// Clear resets the renderer's buffers.
func (r *Renderer) Clear() {
	r.prev = nil
	r.curr = nil
}
