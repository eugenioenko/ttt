package widgets

import (
	"github.com/gdamore/tcell/v3"
)

type HStackWidget struct {
	BaseWidget
	Children    []Widget
	Align       string `json:"align,omitempty"`
	Gap         int    `json:"gap,omitempty"`
	FixedHeight int
}

func (h *HStackWidget) WidgetChildren() []Widget { return h.Children }

func NewHStackWidget(children ...Widget) *HStackWidget {
	return &HStackWidget{Children: children}
}

func (s *HStackWidget) Height() int {
	if s.FixedHeight > 0 {
		return s.FixedHeight + s.BoxOverheadH()
	}
	return 0
}
func (s *HStackWidget) Width() int { return 0 }

func (s *HStackWidget) Render(surface Surface) {
	inner := s.RenderBox(surface)
	w, h := inner.Size()
	if len(s.Children) == 0 || w <= 0 || h <= 0 {
		return
	}

	gapTotal := 0
	if len(s.Children) > 1 {
		gapTotal = (len(s.Children) - 1) * s.Gap
	}

	fixedTotal := 0
	growCount := 0
	for _, child := range s.Children {
		cw := child.Width()
		if cw > 0 {
			fixedTotal += cw
		} else {
			growCount++
		}
	}

	growW := 0
	growRemainder := 0
	if growCount > 0 {
		remaining := w - fixedTotal - gapTotal
		if remaining > 0 {
			growW = remaining / growCount
			growRemainder = remaining % growCount
		}
	}

	r := s.GetRect()
	ox := s.Box.MarginLeft + s.Box.PaddingLeft
	oy := s.Box.MarginTop + s.Box.PaddingTop
	if s.Box.BorderLeft {
		ox++
	}
	if s.Box.BorderTop {
		oy++
	}

	totalUsed := fixedTotal + growW*growCount + growRemainder + gapTotal
	x := 0
	if growCount == 0 {
		switch s.Align {
		case "center":
			x = (w - totalUsed) / 2
		case "right":
			x = w - totalUsed
		}
	}

	growIndex := 0
	for i, child := range s.Children {
		cw := child.Width()
		if cw == 0 {
			cw = growW
			if growIndex < growRemainder {
				cw++
			}
			growIndex++
		}
		if cw <= 0 || x >= w {
			continue
		}
		if x+cw > w {
			cw = w - x
		}
		sub := inner.Sub(Rect{X: x, Y: 0, W: cw, H: h})
		child.SetRect(Rect{X: r.X + ox + x, Y: r.Y + oy, W: cw, H: h})
		child.Render(sub)
		x += cw
		if i < len(s.Children)-1 {
			x += s.Gap
		}
	}
}

func (s *HStackWidget) HandleEvent(ev tcell.Event) EventResult {
	for _, child := range s.Children {
		if result := child.HandleEvent(ev); result != EventIgnored {
			return result
		}
	}
	return EventIgnored
}
