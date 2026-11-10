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
	OnChanged   func()
	invalidated func()
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
	s.OnResizeEnd = func() {
		if e.OnChanged != nil {
			e.OnChanged()
		}
	}
	leaf.Split = s
	e.SetPointerCaptureInvalidated(e.invalidated)
	if e.OnChanged != nil {
		e.OnChanged()
	}
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
	invalidated, changed := e.invalidated, e.OnChanged
	e.remove(group)
	e.OnChanged = changed
	e.SetPointerCaptureInvalidated(invalidated)
	if changed != nil {
		changed()
	}
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
	if e.OnChanged != nil {
		e.OnChanged()
	}
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
	widgets.SetPointerCaptureInvalidated(e.child(), fn)
}

func (e *EditorLayoutWidget) BorderJunctions() borderJunctions {
	junctions := borderJunctions{BottomX: [4]int{-1, -1, -1, -1}, RightY: -1}
	r, index := e.GetRect(), 0
	for _, g := range e.Groups() {
		gr := g.GetRect()
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
