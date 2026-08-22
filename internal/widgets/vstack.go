package widgets

import (
	"github.com/gdamore/tcell/v3"
)

type VStackWidget struct {
	BaseWidget
	Children []Widget
	Align    string `json:"align,omitempty"`
	Gap      int    `json:"gap,omitempty"`
	// MeasureGrow measures grow children by content height instead of collapsing to 0 (scroll views).
	MeasureGrow bool `json:"measureGrow,omitempty"`
}

func (v *VStackWidget) WidgetChildren() []Widget { return v.Children }

func NewVStackWidget(children ...Widget) *VStackWidget {
	return &VStackWidget{Children: children}
}

func (v *VStackWidget) Height() int {
	total := 0
	for _, child := range v.Children {
		ch := child.Height()
		if ch == 0 {
			return 0
		}
		total += ch
	}
	if len(v.Children) > 1 {
		total += (len(v.Children) - 1) * v.Gap
	}
	return total + v.BoxOverheadH()
}
func (v *VStackWidget) Width() int { return 0 }

// measureChild: fixed height, then HeightForWidth, then (MeasureGrow) content height.
func (v *VStackWidget) measureChild(child Widget, w int) int {
	ch := child.Height()
	if ch == 0 {
		if hfw, ok := child.(HeightForWidther); ok {
			ch = hfw.HeightForWidth(w)
		}
	}
	if ch == 0 && v.MeasureGrow {
		if cs, ok := child.(ContentHeighter); ok {
			ch = cs.ContentHeight()
		}
	}
	return ch
}

func (v *VStackWidget) HeightForWidth(w int) int {
	total := 0
	for _, child := range v.Children {
		ch := v.measureChild(child, w)
		if ch == 0 {
			if !v.MeasureGrow {
				return 0
			}
			// Skip unmeasurable children instead of collapsing the whole stack.
			continue
		}
		total += ch
	}
	if len(v.Children) > 1 {
		total += (len(v.Children) - 1) * v.Gap
	}
	return total + v.BoxOverheadH()
}

func (v *VStackWidget) ScrollSize() (int, int) {
	r := v.GetRect()
	w := r.W
	if w <= 0 {
		w = 80
	}
	return w, v.HeightForWidth(w)
}

func (v *VStackWidget) Render(surface Surface) {
	inner := v.RenderBox(surface)
	w, h := inner.Size()
	if len(v.Children) == 0 || w <= 0 || h <= 0 {
		v.InvalidatePointerInteraction()
		return
	}

	gapTotal := 0
	if len(v.Children) > 1 {
		gapTotal = (len(v.Children) - 1) * v.Gap
	}

	fixedTotal := 0
	growCount := 0
	for _, child := range v.Children {
		ch := v.measureChild(child, w)
		if ch > 0 {
			fixedTotal += ch
		} else {
			growCount++
		}
	}

	growH := 0
	growRemainder := 0
	if growCount > 0 {
		remaining := h - fixedTotal - gapTotal
		if remaining > 0 {
			growH = remaining / growCount
			growRemainder = remaining % growCount
		}
	}

	r := v.GetRect()
	ox := v.Box.MarginLeft + v.Box.PaddingLeft
	oy := v.Box.MarginTop + v.Box.PaddingTop
	if v.Box.BorderLeft {
		ox++
	}
	if v.Box.BorderTop {
		oy++
	}

	totalUsed := fixedTotal + growH*growCount + growRemainder + gapTotal
	y := 0
	if growCount == 0 {
		switch v.Align {
		case "center":
			y = (h - totalUsed) / 2
		case "bottom":
			y = h - totalUsed
		}
	}

	growIndex := 0
	for i, child := range v.Children {
		ch := v.measureChild(child, w)
		if ch == 0 {
			ch = growH
			if growIndex < growRemainder {
				ch++
			}
			growIndex++
		}
		if ch <= 0 || y >= h {
			InvalidatePointerInteraction(child)
			continue
		}
		if y+ch > h {
			ch = h - y
		}
		sub := inner.Sub(Rect{X: 0, Y: y, W: w, H: ch})
		child.SetRect(Rect{X: r.X + ox, Y: r.Y + oy + y, W: w, H: ch})
		child.Render(sub)
		y += ch
		if i < len(v.Children)-1 {
			y += v.Gap
		}
	}
}

func (v *VStackWidget) HandleEvent(ev tcell.Event) EventResult {
	for _, child := range v.Children {
		if result := child.HandleEvent(ev); result != EventIgnored {
			return result
		}
	}
	return EventIgnored
}

func (v *VStackWidget) CancelPointerCapture() bool {
	canceled := false
	for _, child := range v.Children {
		canceled = CancelPointerCapture(child) || canceled
	}
	return canceled
}

func (v *VStackWidget) InvalidatePointerInteraction() bool {
	invalidated := false
	for _, child := range v.Children {
		invalidated = InvalidatePointerInteraction(child) || invalidated
	}
	return invalidated
}

func (v *VStackWidget) SetPointerCaptureInvalidated(invalidated func()) {
	for _, child := range v.Children {
		SetPointerCaptureInvalidated(child, invalidated)
	}
}

func (v *VStackWidget) OwnsPointerCapture() bool {
	for _, child := range v.Children {
		if owner, ok := child.(PointerCaptureOwner); ok && owner.OwnsPointerCapture() {
			return true
		}
	}
	return false
}
