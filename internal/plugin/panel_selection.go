package plugin

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type cellPos struct{ x, y int }

func (a cellPos) before(b cellPos) bool {
	return a.y < b.y || (a.y == b.y && a.x < b.x)
}

// panelSelection selects the text a panel last drew, like a terminal does:
// a drag that leaves the cell it pressed on, in reading order.
type panelSelection struct {
	cells    [][]term.Cell
	pressed  bool
	blocked  bool
	dragging bool
	active   bool
	anchor   cellPos
	end      cellPos
}

func (s *panelSelection) snapshot(surface widgets.Surface) {
	reader, ok := surface.(widgets.CellReader)
	if !ok {
		s.cells = nil
		return
	}
	w, h := surface.Size()
	if len(s.cells) != h || (h > 0 && len(s.cells[0]) != w) {
		s.cells = make([][]term.Cell, h)
		for y := range s.cells {
			s.cells[y] = make([]term.Cell, w)
		}
	}
	for y := range h {
		for x := range w {
			s.cells[y][x], _ = reader.CellAt(x, y)
		}
	}
}

func (s *panelSelection) bounds() (cellPos, cellPos) {
	if s.end.before(s.anchor) {
		return s.end, s.anchor
	}
	return s.anchor, s.end
}

func (s *panelSelection) each(fn func(y, x0, x1 int)) {
	if !s.active || len(s.cells) == 0 {
		return
	}
	start, end := s.bounds()
	w := len(s.cells[0])
	for y := max(start.y, 0); y <= end.y && y < len(s.cells); y++ {
		x0, x1 := 0, w-1
		if y == start.y {
			x0 = start.x
		}
		if y == end.y {
			x1 = end.x
		}
		fn(y, max(x0, 0), min(x1, w-1))
	}
}

func (s *panelSelection) highlight(surface widgets.Surface) {
	s.each(func(y, x0, x1 int) {
		row := s.cells[y]
		for x := x0; x <= x1; x++ {
			c := row[x]
			c.BgStyle = term.StylePluginSelection
			surface.SetCell(x, y, c)
			if textwidth.Rune(c.Ch) == 2 {
				x++
			}
		}
	})
}

func (s *panelSelection) text() string {
	var lines []string
	s.each(func(y, x0, x1 int) {
		var b strings.Builder
		row := s.cells[y]
		for x := x0; x <= x1; x++ {
			ch := row[x].Ch
			if ch == 0 {
				ch = ' '
			}
			b.WriteRune(ch)
			if textwidth.Rune(ch) == 2 {
				x++
			}
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	})
	return strings.Join(lines, "\n")
}

func (s *panelSelection) selectAll() {
	if len(s.cells) == 0 || len(s.cells[0]) == 0 {
		return
	}
	s.anchor = cellPos{}
	s.end = cellPos{x: len(s.cells[0]) - 1, y: len(s.cells) - 1}
	s.active = true
}

// release forgets a press whose release went elsewhere, such as to a menu
// the press opened, so the next press is not taken for a drag.
func (s *panelSelection) release() bool {
	was := s.pressed
	s.pressed, s.blocked, s.dragging = false, false, false
	return was
}

func (s *panelSelection) clear() bool {
	was := s.active
	s.active = false
	return was
}

// track reports whether the selection consumed a mouse event. A press is left
// to the panel's widgets; the drag that follows it is not, unless a widget
// selects text of its own there or captured the pointer.
func (s *panelSelection) track(ev *tcell.EventMouse, r widgets.Rect, ownsDrag bool) (widgets.EventResult, bool) {
	mx, my := ev.Position()
	p := cellPos{x: min(max(mx-r.X, 0), max(r.W-1, 0)), y: min(max(my-r.Y, 0), max(r.H-1, 0))}
	btn := ev.Buttons()
	switch {
	case btn&tcell.Button1 != 0:
		if s.dragging {
			s.end = p
			return widgets.EventCaptured, true
		}
		if !s.pressed {
			s.pressed = true
			s.blocked = ownsDrag
			s.active = false
			s.anchor = p
			return widgets.EventIgnored, false
		}
		if !s.blocked && p != s.anchor {
			s.dragging, s.active = true, true
			s.end = p
			return widgets.EventCaptured, true
		}
	case btn == tcell.ButtonNone:
		was := s.dragging
		s.pressed, s.blocked, s.dragging = false, false, false
		if was {
			return widgets.EventConsumed, true
		}
	}
	return widgets.EventIgnored, false
}

// ownsDragAt reports whether the widget under the pointer selects text itself:
// inputs and markdown keep their drags.
func ownsDragAt(w widgets.Widget, x, y int) bool {
	if w == nil {
		return false
	}
	r := w.GetRect()
	if x < r.X || x >= r.X+r.W || y < r.Y || y >= r.Y+r.H {
		return false
	}
	switch w.(type) {
	case *widgets.MarkdownWidget, *widgets.InputWidget:
		return true
	}
	if c, ok := w.(widgets.ContainerWidget); ok {
		for _, child := range c.WidgetChildren() {
			if ownsDragAt(child, x, y) {
				return true
			}
		}
	}
	return false
}

func isCopyKey(ev *tcell.EventKey) bool {
	r := term.KeyRune(ev)
	return ev.Key() == tcell.KeyCtrlC ||
		(ev.Key() == tcell.KeyRune && (r == 'c' || r == 'C') && ev.Modifiers()&tcell.ModCtrl != 0)
}
