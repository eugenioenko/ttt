package widgets

import "github.com/eugenioenko/ttt/internal/term"

// PointerShaper is authoritative for its whole subtree: PointerShapeAt does not
// descend past it. An empty result means the widget has no opinion, so the
// caller falls back to whatever lies beneath.
type PointerShaper interface {
	PointerShapeAt(mx, my int) string
}

func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func OwnsPointerCapture(w Widget) bool {
	owner, ok := w.(PointerCaptureOwner)
	return ok && owner.OwnsPointerCapture()
}

func PointerShapeAt(w Widget, mx, my int) string {
	if w == nil {
		return ""
	}
	if shaper, ok := w.(PointerShaper); ok {
		return shaper.PointerShapeAt(mx, my)
	}
	container, ok := w.(ContainerWidget)
	if !ok {
		return ""
	}
	children := container.WidgetChildren()
	for _, child := range children {
		if OwnsPointerCapture(child) {
			return PointerShapeAt(child, mx, my)
		}
	}
	// Later children paint over earlier ones.
	for i := len(children) - 1; i >= 0; i-- {
		if children[i].GetRect().Contains(mx, my) {
			if shape := PointerShapeAt(children[i], mx, my); shape != "" {
				return shape
			}
		}
	}
	return ""
}

func (b *ButtonWidget) PointerShapeAt(mx, my int) string {
	if !b.Disabled && b.GetRect().Contains(mx, my) {
		return term.PointerHand
	}
	return ""
}

func (inp *InputWidget) PointerShapeAt(mx, my int) string {
	for _, hit := range inp.ActionHits {
		if hit.Contains(mx, my) {
			return term.PointerHand
		}
	}
	if inp.GetRect().Contains(mx, my) {
		return term.PointerText
	}
	return ""
}

func (t *TreeWidget) PointerShapeAt(mx, my int) string {
	if t.scrollbar.isDragging() {
		return term.PointerDefault
	}
	if !t.rect.Contains(mx, my) || t.scrollbar.interaction.geometry.hitTrack.Contains(mx, my) {
		return ""
	}
	idx := t.scrollTop + (my - t.contentY)
	if idx < 0 || idx >= len(t.flatList) || t.flatList[idx].Heading {
		return ""
	}
	return term.PointerHand
}

func (t *TabsWidget) PointerShapeAt(mx, my int) string {
	if t.drag.Dragging() {
		return term.PointerGrabbing
	}
	r := t.GetRect()
	if !r.Contains(mx, my) {
		return ""
	}
	lx := mx - r.X - t.Box.MarginLeft - t.Box.PaddingLeft
	inSpan := func(span [2]int) bool { return lx >= span[0] && lx < span[1] }
	for _, span := range t.actionSpans {
		if inSpan(span) {
			return term.PointerHand
		}
	}
	if t.Config.OnOverflow != nil && t.overSpan[1] > 0 && inSpan(t.overSpan) {
		return term.PointerHand
	}
	for _, span := range t.tabSpans {
		if inSpan(span) {
			return term.PointerHand
		}
	}
	return ""
}
