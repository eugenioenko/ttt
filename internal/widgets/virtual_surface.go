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
	for dy := range h {
		for dx := range w {
			s.SetCell(x+dx, y+dy, term.Cell{Ch: ' ', Style: style})
		}
	}
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
