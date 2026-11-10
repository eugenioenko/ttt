package ui

import (
	"math"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

const MaxEditorGroups = 4

type EditorLayoutWidget struct {
	BaseWidget
	Group       *EditorGroupWidget
	First       *EditorLayoutWidget
	Second      *EditorLayoutWidget
	Split       *ContentSplitWidget
	Borders     *term.BorderSet
	invalidated func()
	dividerGaps []Rect
}

func NewEditorLayoutWidget(group *EditorGroupWidget, borders *term.BorderSet) *EditorLayoutWidget {
	return &EditorLayoutWidget{Group: group, Borders: borders}
}

func (e *EditorLayoutWidget) child() Widget {
	if e.Group != nil {
		return e.Group
	}
	return e.Split
}

func (e *EditorLayoutWidget) WidgetChildren() []Widget { return []Widget{e.child()} }
func (e *EditorLayoutWidget) Focusable() bool          { return false }

func (e *EditorLayoutWidget) Groups() []*EditorGroupWidget {
	if e.Group != nil {
		return []*EditorGroupWidget{e.Group}
	}
	return append(e.First.Groups(), e.Second.Groups()...)
}

func (e *EditorLayoutWidget) leaf(group *EditorGroupWidget) *EditorLayoutWidget {
	if e.Group == group {
		return e
	}
	if e.Group != nil {
		return nil
	}
	if found := e.First.leaf(group); found != nil {
		return found
	}
	return e.Second.leaf(group)
}

func (e *EditorLayoutWidget) SplitGroup(group, next *EditorGroupWidget, position SplitPosition) bool {
	leaf := e.leaf(group)
	if leaf == nil || len(e.Groups()) >= MaxEditorGroups {
		return false
	}
	e.InvalidatePointerInteraction()
	leaf.First = NewEditorLayoutWidget(group, e.Borders)
	leaf.Second = NewEditorLayoutWidget(next, e.Borders)
	leaf.Group = nil
	s := NewContentSplitWidget()
	s.Position, s.FullHeightRight, s.ShowBottom = position, true, true
	s.HideBottomDivider = true
	s.Borders, s.MinRightW, s.MinTopH, s.MinBottomH = e.Borders, 20, 6, 6
	s.Top, s.Bottom = leaf.First, leaf.Second
	s.RightW, s.BottomH, s.BottomRatio = 0, 0, 0.5
	s.OnResize = func(size int) {
		total := s.GetRect().H
		if s.Position == SplitRight {
			total = s.GetRect().W
		}
		s.BottomRatio = float64(size) / float64(max(total, 1))
		s.RightW, s.BottomH = 0, 0
	}
	leaf.Split = s
	e.SetPointerCaptureInvalidated(e.invalidated)
	return true
}

func (e *EditorLayoutWidget) remove(group *EditorGroupWidget) bool {
	if e.Group != nil {
		return e.Group == group
	}
	if e.First.remove(group) {
		*e = *e.Second
	} else if e.Second.remove(group) {
		*e = *e.First
	}
	return false
}

func (e *EditorLayoutWidget) RemoveGroup(group *EditorGroupWidget) bool {
	if len(e.Groups()) == 1 || e.leaf(group) == nil {
		return false
	}
	e.InvalidatePointerInteraction()
	invalidated := e.invalidated
	e.remove(group)
	e.SetPointerCaptureInvalidated(invalidated)
	return true
}

func (e *EditorLayoutWidget) Equalize() {
	var equalize func(*EditorLayoutWidget)
	equalize = func(node *EditorLayoutWidget) {
		if node.Split != nil {
			first, second := node.First.axisUnits(node.Split.Position), node.Second.axisUnits(node.Split.Position)
			node.Split.BottomRatio, node.Split.RightW, node.Split.BottomH = float64(second)/float64(first+second), 0, 0
			equalize(node.First)
			equalize(node.Second)
		}
	}
	equalize(e)
}

func (e *EditorLayoutWidget) Neighbor(group *EditorGroupWidget, dx, dy int) *EditorGroupWidget {
	r := group.GetRect()
	best := math.MaxInt
	var target *EditorGroupWidget
	for _, other := range e.Groups() {
		if other == group {
			continue
		}
		o := other.GetRect()
		x, y := (o.X*2+o.W)-(r.X*2+r.W), (o.Y*2+o.H)-(r.Y*2+r.H)
		if (dx != 0 && (o.Y >= r.Y+r.H || r.Y >= o.Y+o.H)) || (dy != 0 && (o.X >= r.X+r.W || r.X >= o.X+o.W)) {
			continue
		}
		if dx*x+dy*y <= 0 {
			continue
		}
		primary, secondary := max(x, -x), max(y, -y)
		if dy != 0 {
			primary, secondary = secondary, primary
		}
		if score := primary + secondary*4; score < best {
			best, target = score, other
		}
	}
	return target
}

func (e *EditorLayoutWidget) Render(surface Surface) {
	child := e.child()
	child.SetRect(e.GetRect())
	child.Render(surface)
	e.renderDividerGaps(surface)
	e.renderJunctions(surface)
}

func (e *EditorLayoutWidget) HandleEvent(ev tcell.Event) EventResult {
	return e.child().HandleEvent(ev)
}
func (e *EditorLayoutWidget) PointerShapeAt(x, y int) string {
	return widgets.PointerShapeAt(e.child(), x, y)
}
func (e *EditorLayoutWidget) OwnsPointerCapture() bool { return widgets.OwnsPointerCapture(e.child()) }
func (e *EditorLayoutWidget) CancelPointerCapture() bool {
	return widgets.CancelPointerCapture(e.child())
}
func (e *EditorLayoutWidget) InvalidatePointerInteraction() bool {
	return widgets.InvalidatePointerInteraction(e.child())
}
func (e *EditorLayoutWidget) SetPointerCaptureInvalidated(fn func()) {
	e.invalidated = fn
	if e.Split != nil {
		e.Split.DividerVisible = e.dividerVisibleAt
	}
	widgets.SetPointerCaptureInvalidated(e.child(), fn)
}

func (e *EditorLayoutWidget) BorderJunctions() borderJunctions {
	junctions := borderJunctions{BottomX: [4]int{-1, -1, -1, -1}, RightY: [4]int{-1, -1, -1, -1}}
	r, index, rightIndex := e.GetRect(), 0, 0
	for _, g := range e.Groups() {
		gr := g.GetRect()
		if gr.X+gr.W == r.X+r.W && gr.Y > r.Y && rightIndex < len(junctions.RightY) && !e.Split.HideBottomDivider {
			junctions.RightY[rightIndex] = gr.Y - 1
			rightIndex++
		}
		if gr.Y+gr.H == r.Y+r.H && gr.X > r.X && index < len(junctions.BottomX) {
			junctions.BottomX[index] = gr.X - 1
			index++
		}
	}
	return junctions
}

func (e *EditorLayoutWidget) axisUnits(axis SplitPosition) int {
	if e.Group != nil {
		return 1
	}
	first, second := e.First.axisUnits(axis), e.Second.axisUnits(axis)
	if e.Split.Position == axis {
		return first + second
	}
	return max(first, second)
}

func (e *EditorLayoutWidget) renderJunctions(surface Surface) {
	if e.Split == nil {
		return
	}
	b := term.SingleBorderSet()
	if e.Borders != nil {
		b = *e.Borders
	}
	if b.Cross == 0 {
		b.Cross = b.LeftTee
	}
	r, joins := e.GetRect(), map[int]int{}
	if e.Split.Position == SplitRight {
		tabJoins := map[int]uint8{}
		for _, group := range e.Groups() {
			if side := e.dividerSide(group); side != 0 {
				tabs := group.TabBar.GetRect()
				tabJoins[tabs.Y+tabs.H-1] |= side
			}
		}
		x := e.Split.DividerScreenX()
		for y, sides := range tabJoins {
			glyph := b.TopTee
			if e.dividerVisibleAt(x, y-1) {
				glyph = b.Cross
				if sides == 1 {
					glyph = b.RightTee
				} else if sides == 2 {
					glyph = b.LeftTee
				}
			} else if sides == 1 {
				glyph = b.TopRight
			} else if sides == 2 {
				glyph = b.TopLeft
			}
			surface.SetCell(x-r.X, y-r.Y, term.Cell{Ch: glyph, Style: term.StyleBorder})
		}
	}
	if e.Split.HideBottomDivider {
		return
	}
	for _, group := range e.First.Groups() {
		gr := group.GetRect()
		if e.Split.Position == SplitRight {
			if gr.X+gr.W == e.First.GetRect().X+e.First.GetRect().W && gr.Y > r.Y {
				joins[gr.Y-1] |= 1
			}
		} else if gr.Y+gr.H == e.First.GetRect().Y+e.First.GetRect().H && gr.X > r.X {
			joins[gr.X-1] |= 1
		}
	}
	for _, group := range e.Second.Groups() {
		gr := group.GetRect()
		if e.Split.Position == SplitRight {
			if gr.X == e.Second.GetRect().X && gr.Y > r.Y {
				joins[gr.Y-1] |= 2
			}
		} else if gr.Y == e.Second.GetRect().Y && gr.X > r.X {
			joins[gr.X-1] |= 2
		}
	}
	for coordinate, sides := range joins {
		glyph := b.Cross
		x, y := e.Split.DividerScreenX()-r.X, coordinate-r.Y
		if e.Split.Position == SplitRight {
			if sides == 1 {
				glyph = b.RightTee
			} else if sides == 2 {
				glyph = b.LeftTee
			}
		} else {
			x, y = coordinate-r.X, e.Split.DividerScreenY()-r.Y
			if sides == 1 {
				glyph = b.BottomTee
			} else if sides == 2 {
				glyph = b.TopTee
			}
		}
		surface.SetCell(x, y, term.Cell{Ch: glyph, Style: term.StyleBorder})
	}
}

func (e *EditorLayoutWidget) renderDividerGaps(surface Surface) {
	e.dividerGaps = e.dividerGaps[:0]
	if e.Split == nil || e.Split.Position != SplitRight {
		return
	}
	r, dividerX := e.GetRect(), e.Split.DividerScreenX()
	tabSides := make([]uint8, max(r.H, 0))
	for _, group := range e.Groups() {
		side := e.dividerSide(group)
		if side == 0 {
			continue
		}
		tabs := group.TabBar.GetRect()
		for y := max(tabs.Y, r.Y); y < min(tabs.Y+tabs.H, r.Y+r.H); y++ {
			tabSides[y-r.Y] |= side
		}
	}
	for y, sides := range tabSides {
		if sides == 3 {
			e.dividerGaps = append(e.dividerGaps, Rect{X: dividerX, Y: y + r.Y, W: 1, H: 1})
			surface.SetCell(dividerX-r.X, y, term.Cell{Ch: ' ', Style: term.StyleDefault})
		}
	}
}

func (e *EditorLayoutWidget) dividerSide(group *EditorGroupWidget) uint8 {
	r, x := group.GetRect(), e.Split.DividerScreenX()
	if r.X+r.W == x {
		return 1
	}
	if r.X == x+1 {
		return 2
	}
	return 0
}

func (e *EditorLayoutWidget) dividerVisibleAt(x, y int) bool {
	for _, gap := range e.dividerGaps {
		if gap.Contains(x, y) {
			return false
		}
	}
	return true
}
