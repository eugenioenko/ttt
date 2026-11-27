package ui

import (
	"context"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
)

const DividerHighlightDelay = 300 * time.Millisecond

type resizeDivider interface {
	Dragging() bool
	OverDivider(mx, my int) bool
	dividerLine() Rect
	// dividerJunction is a tee a parent border draws where the line ends,
	// outside the divider's own rect.
	dividerJunction() (x, y int, glyph rune, ok bool)
}

func registerDivider(surface Surface, divider resizeDivider) {
	if rs, ok := surface.(*RenderSurface); ok && rs.frame != nil && rs.frame.dividers != nil {
		*rs.frame.dividers = append(*rs.frame.dividers, divider)
	}
}

// Editor-split dividers are drawn by the groups' tab bars, so this must run
// after the whole main tree renders and before overlays paint over it.
func (r *Root) highlightDivider(surface *RenderSurface, dividers []resizeDivider) {
	for _, divider := range dividers {
		if divider.Dragging() {
			r.clearDividerHover()
			r.hoverDivider = divider
			paintDivider(surface, divider)
			return
		}
	}
	hovered := r.hoveredDivider(dividers)
	if hovered == nil {
		r.clearDividerHover()
		return
	}
	if hovered != r.hoverDivider {
		r.hoverDivider = nil
		r.armDividerHover(hovered)
	}
	if hovered == r.hoverDivider {
		paintDivider(surface, hovered)
	}
}

func (r *Root) hoveredDivider(dividers []resizeDivider) resizeDivider {
	if !r.pointerSeen {
		return nil
	}
	switch r.PointerShapeAt(r.pointerX, r.pointerY) {
	case term.PointerEWResize, term.PointerNSResize:
	default:
		return nil
	}
	for _, divider := range dividers {
		if divider.OverDivider(r.pointerX, r.pointerY) {
			return divider
		}
	}
	return nil
}

// Without a delivering task the highlight shows immediately, which keeps
// SimScreen tests synchronous.
func (r *Root) armDividerHover(divider resizeDivider) {
	if r.DividerHoverTask == nil {
		r.hoverDivider = divider
		return
	}
	r.DividerHoverTask.ScheduleKey(divider,
		func(context.Context) (struct{}, error) { return struct{}{}, nil },
		func(struct{}, error) { r.hoverDivider = divider })
}

func (r *Root) clearDividerHover() {
	r.hoverDivider = nil
	if r.DividerHoverTask != nil {
		r.DividerHoverTask.Stop()
	}
}

func paintDivider(surface *RenderSurface, divider resizeDivider) {
	line := divider.dividerLine()
	for y := line.Y; y < line.Y+line.H; y++ {
		for x := line.X; x < line.X+line.W; x++ {
			paintDividerCell(surface, x, y, 0)
		}
	}
	if x, y, glyph, ok := divider.dividerJunction(); ok {
		paintDividerCell(surface, x, y, glyph)
	}
}

func paintDividerCell(surface *RenderSurface, x, y int, glyph rune) {
	c, ok := surface.CellAt(x, y)
	if !ok || (glyph != 0 && c.Ch != glyph) {
		return
	}
	if c.Style == term.StyleBorder || c.Style == term.StyleBorderActive {
		c.Style = term.StyleBorderHover
		surface.SetCell(x, y, c)
	}
}

func (s *SplitPanelWidget) dividerLine() Rect {
	r := s.GetRect()
	x := r.X
	if s.ShowLeft {
		x = s.DividerScreenX()
	}
	return Rect{X: x, Y: r.Y, W: 1, H: r.H}
}

func (s *SplitPanelWidget) dividerJunction() (int, int, rune, bool) { return 0, 0, 0, false }

func (cs *ContentSplitWidget) dividerLine() Rect {
	r := cs.GetRect()
	if cs.Position == SplitRight {
		return Rect{X: cs.DividerScreenX(), Y: r.Y, W: 1, H: r.H}
	}
	return Rect{X: r.X, Y: cs.DividerScreenY(), W: r.W, H: 1}
}

func (cs *ContentSplitWidget) dividerJunction() (int, int, rune, bool) {
	if cs.Position != SplitRight {
		return 0, 0, 0, false
	}
	b := term.SingleBorderSet()
	if cs.Borders != nil {
		b = *cs.Borders
	}
	r := cs.GetRect()
	return cs.DividerScreenX(), r.Y + r.H, b.BottomTee, true
}
