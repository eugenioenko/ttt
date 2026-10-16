package widgets

import "github.com/eugenioenko/ttt/internal/term"

type virtualSurface struct {
	w, h  int
	cells [][]term.Cell
}

func newVirtualSurface(w, h int) *virtualSurface {
	cells := make([][]term.Cell, h)
	for y := range h {
		row := make([]term.Cell, w)
		for x := range w {
			row[x] = term.Cell{Ch: ' '}
		}
		cells[y] = row
	}
	return &virtualSurface{w: w, h: h, cells: cells}
}

func (v *virtualSurface) Size() (int, int)   { return v.w, v.h }
func (v *virtualSurface) Origin() (int, int) { return 0, 0 }

func (v *virtualSurface) SetCell(x, y int, c term.Cell) {
	if x >= 0 && x < v.w && y >= 0 && y < v.h {
		v.cells[y][x] = c
	}
}

func (v *virtualSurface) DrawText(x, y int, text string, maxW int, style term.Style) int {
	return drawTextWidthAware(v, x, y, text, maxW, v.w, style)
}

func (v *virtualSurface) DrawBorder(x, y, w, h int, b term.BorderSet, style term.Style) {
	if w < 2 || h < 2 {
		return
	}
	v.SetCell(x, y, term.Cell{Ch: b.TopLeft, Style: style})
	v.SetCell(x+w-1, y, term.Cell{Ch: b.TopRight, Style: style})
	v.SetCell(x, y+h-1, term.Cell{Ch: b.BottomLeft, Style: style})
	v.SetCell(x+w-1, y+h-1, term.Cell{Ch: b.BottomRight, Style: style})
	for i := x + 1; i < x+w-1; i++ {
		v.SetCell(i, y, term.Cell{Ch: b.Horizontal, Style: style})
		v.SetCell(i, y+h-1, term.Cell{Ch: b.Horizontal, Style: style})
	}
	for i := y + 1; i < y+h-1; i++ {
		v.SetCell(x, i, term.Cell{Ch: b.Vertical, Style: style})
		v.SetCell(x+w-1, i, term.Cell{Ch: b.Vertical, Style: style})
	}
}

func (v *virtualSurface) ClearRect(x, y, w, h int, style term.Style) {
	x, y, w, h, ok := ClampRectToBounds(x, y, w, h, 0, 0, v.w, v.h)
	if !ok {
		return
	}
	for dy := range h {
		for dx := range w {
			v.SetCell(x+dx, y+dy, term.Cell{Ch: ' ', Style: style})
		}
	}
}

func (v *virtualSurface) Fill(c term.Cell) {
	for y := range v.h {
		for x := range v.w {
			v.cells[y][x] = c
		}
	}
}

func (v *virtualSurface) Sub(r Rect) Surface {
	return &subVirtualSurface{parent: v, offX: r.X, offY: r.Y, w: r.W, h: r.H}
}

type subVirtualSurface struct {
	parent *virtualSurface
	offX   int
	offY   int
	w      int
	h      int
}

func (s *subVirtualSurface) Size() (int, int)   { return s.w, s.h }
func (s *subVirtualSurface) Origin() (int, int) { return s.offX, s.offY }

func (s *subVirtualSurface) SetCell(x, y int, c term.Cell) {
	if x >= 0 && x < s.w && y >= 0 && y < s.h {
		s.parent.SetCell(s.offX+x, s.offY+y, c)
	}
}

func (s *subVirtualSurface) DrawText(x, y int, text string, maxW int, style term.Style) int {
	return drawTextWidthAware(s, x, y, text, maxW, s.w, style)
}

func (s *subVirtualSurface) DrawBorder(x, y, w, h int, b term.BorderSet, style term.Style) {
	if w < 2 || h < 2 {
		return
	}
	s.SetCell(x, y, term.Cell{Ch: b.TopLeft, Style: style})
	s.SetCell(x+w-1, y, term.Cell{Ch: b.TopRight, Style: style})
	s.SetCell(x, y+h-1, term.Cell{Ch: b.BottomLeft, Style: style})
	s.SetCell(x+w-1, y+h-1, term.Cell{Ch: b.BottomRight, Style: style})
	for i := x + 1; i < x+w-1; i++ {
		s.SetCell(i, y, term.Cell{Ch: b.Horizontal, Style: style})
		s.SetCell(i, y+h-1, term.Cell{Ch: b.Horizontal, Style: style})
	}
	for i := y + 1; i < y+h-1; i++ {
		s.SetCell(x, i, term.Cell{Ch: b.Vertical, Style: style})
		s.SetCell(x+w-1, i, term.Cell{Ch: b.Vertical, Style: style})
	}
}

func (s *subVirtualSurface) ClearRect(x, y, w, h int, style term.Style) {
	minX, maxX := subSurfaceBounds(s.offX, s.w, s.parent.w)
	minY, maxY := subSurfaceBounds(s.offY, s.h, s.parent.h)
	x, y, w, h, ok := ClampRectToBounds(x, y, w, h, minX, minY, maxX, maxY)
	if !ok {
		return
	}
	for dy := range h {
		for dx := range w {
			s.SetCell(x+dx, y+dy, term.Cell{Ch: ' ', Style: style})
		}
	}
}

// ClampRectToBounds returns the part of the requested rectangle inside the bounds.
func ClampRectToBounds(x, y, w, h, minX, minY, maxX, maxY int) (int, int, int, int, bool) {
	if w <= 0 || h <= 0 || minX >= maxX || minY >= maxY {
		return 0, 0, 0, 0, false
	}
	maxInt := int(^uint(0) >> 1)
	right := x + w
	if x > maxInt-w {
		right = maxInt
	}
	bottom := y + h
	if y > maxInt-h {
		bottom = maxInt
	}
	x = max(x, minX)
	y = max(y, minY)
	right = min(right, maxX)
	bottom = min(bottom, maxY)
	if x >= right || y >= bottom {
		return 0, 0, 0, 0, false
	}
	return x, y, right - x, bottom - y, true
}

func subSurfaceBounds(offset, size, parentSize int) (int, int) {
	if size <= 0 || parentSize <= 0 || offset >= parentSize {
		return 0, 0
	}
	if offset >= 0 {
		max := parentSize - offset
		if max > size {
			max = size
		}
		return 0, max
	}
	if offset <= -size {
		return 0, 0
	}
	min := -offset
	visible := size - min
	if parentSize >= visible {
		return min, size
	}
	return min, min + parentSize
}

func (s *subVirtualSurface) Fill(c term.Cell) {
	for y := range s.h {
		for x := range s.w {
			s.SetCell(x, y, c)
		}
	}
}

func (s *subVirtualSurface) Sub(r Rect) Surface {
	return &subVirtualSurface{
		parent: s.parent,
		offX:   s.offX + r.X,
		offY:   s.offY + r.Y,
		w:      r.W,
		h:      r.H,
	}
}
