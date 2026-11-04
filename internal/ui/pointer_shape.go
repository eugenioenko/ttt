package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

// PointerShapeAt must run after event routing so a drag that just ended already
// reads as ended. An overlay blocks everything beneath it, as it does for
// clicks, and PushOverlay cancels any capture held by the main tree.
func (r *Root) PointerShapeAt(mx, my int) string {
	target := r.Main
	if top := r.TopOverlayWidget(); top != nil {
		target = top
	}
	if shape := widgets.PointerShapeAt(target, mx, my); shape != "" {
		return shape
	}
	return term.PointerDefault
}

func (a *WidgetAdapter) PointerShapeAt(mx, my int) string {
	if a.capturedWidget != nil {
		return widgets.PointerShapeAt(a.capturedWidget, mx, my)
	}
	if popup := a.popupAt(mx, my); popup != nil {
		return widgets.PointerShapeAt(popup, mx, my)
	}
	return widgets.PointerShapeAt(a.W, mx, my)
}

func (s *SplitPanelWidget) PointerShapeAt(mx, my int) string {
	if s.dragging {
		return term.PointerEWResize
	}
	for _, child := range []Widget{s.Left, s.Right} {
		if widgets.OwnsPointerCapture(child) {
			return widgets.PointerShapeAt(child, mx, my)
		}
	}
	if s.OverDivider(mx, my) {
		return term.PointerEWResize
	}
	if !s.GetRect().Contains(mx, my) {
		return ""
	}
	if s.ShowLeft && mx < s.DividerScreenX() {
		return widgets.PointerShapeAt(s.Left, mx, my)
	}
	return widgets.PointerShapeAt(s.Right, mx, my)
}

func (cs *ContentSplitWidget) PointerShapeAt(mx, my int) string {
	if cs.dragging {
		return cs.ResizeShape()
	}
	for _, child := range []Widget{cs.Top, cs.Bottom} {
		if widgets.OwnsPointerCapture(child) {
			return widgets.PointerShapeAt(child, mx, my)
		}
	}
	if cs.OverDivider(mx, my) {
		return cs.ResizeShape()
	}
	if cs.ShowBottom && cs.Bottom != nil && cs.Bottom.GetRect().Contains(mx, my) {
		return widgets.PointerShapeAt(cs.Bottom, mx, my)
	}
	return widgets.PointerShapeAt(cs.Top, mx, my)
}

func (s *SidebarWidget) PointerShapeAt(mx, my int) string {
	if s.capturedChild != nil {
		return widgets.PointerShapeAt(s.capturedChild, mx, my)
	}
	if s.Tabs.PointerGestureActive() || my == s.GetRect().Y {
		return s.Tabs.PointerShapeAt(mx, my)
	}
	return widgets.PointerShapeAt(s.ActiveWidget(), mx, my)
}

func (bp *BottomPanelWidget) PointerShapeAt(mx, my int) string {
	if bp.Tabs.PointerGestureActive() || my == bp.GetRect().Y {
		return bp.Tabs.PointerShapeAt(mx, my)
	}
	return widgets.PointerShapeAt(bp.ActiveWidget(), mx, my)
}

func (m *MenuBarWidget) PointerShapeAt(mx, my int) string {
	r := m.GetRect()
	if !r.Contains(mx, my) {
		return ""
	}
	localX := mx - r.X
	for _, span := range m.ItemSpans() {
		if localX >= span.Start && localX < span.End {
			return term.PointerHand
		}
	}
	return ""
}

func (t *TabBarWidget) PointerShapeAt(mx, my int) string {
	if t.drag.Dragging() {
		return term.PointerGrabbing
	}
	if t.MoreButton != nil && t.MoreButton.GetRect().Contains(mx, my) {
		return term.PointerHand
	}
	r := t.GetRect()
	if !r.Contains(mx, my) {
		return ""
	}
	arrowW := t.renderArrowW
	if arrowW > 0 && mx < r.X+arrowW {
		return handIf(t.hasOverflowLeft)
	}
	rightZoneStart := r.X + t.renderInnerRight
	if arrowW > 0 && mx >= rightZoneStart && mx < rightZoneStart+arrowW {
		return handIf(t.hasOverflowRight)
	}
	localX := mx - r.X - arrowW + t.ScrollOffset
	for _, span := range t.tabSpans {
		if localX >= span.start && localX < span.end {
			return term.PointerHand
		}
	}
	return ""
}

func handIf(clickable bool) string {
	if clickable {
		return term.PointerHand
	}
	return term.PointerDefault
}

func (g *EditorGroupWidget) PointerShapeAt(mx, my int) string {
	// Hover and completion popups are painted over the text and take its clicks.
	if g.Hover != nil && g.Hover.GetRect().Contains(mx, my) {
		return term.PointerDefault
	}
	if g.Autocomplete != nil && g.Autocomplete.GetRect().Contains(mx, my) {
		return term.PointerDefault
	}
	if !g.Editor.mouseDown {
		if shape := g.TabBar.PointerShapeAt(mx, my); shape != "" {
			return shape
		}
	}
	t := g.activeTab()
	switch {
	case t == nil:
		return ""
	case t.Content != nil:
		return widgets.PointerShapeAt(t.Content, mx, my)
	case t.Diff != nil:
		return t.Diff.PointerShapeAt(mx, my)
	}
	return g.Editor.PointerShapeAt(mx, my)
}

func (d *DiffEditorWidget) PointerShapeAt(mx, my int) string {
	d.ensurePanes()
	if d.extendedFetching || d.Loading {
		return ""
	}
	if d.captured != nil {
		return d.captured.PointerShapeAt(mx, my)
	}
	pane := d.paneAt(mx, my)
	if pane == nil {
		return ""
	}
	if _, onGap := d.gapAt(pane, my); onGap {
		return term.PointerHand
	}
	return pane.PointerShapeAt(mx, my)
}

func (e *EditorPaneWidget) PointerShapeAt(mx, my int) string {
	if e.scrollbar.IsDragging() || e.hscrollbar.IsDragging() {
		return term.PointerDefault
	}
	if e.mouseDown {
		return term.PointerText
	}
	r := e.GetRect()
	if !r.Contains(mx, my) {
		return ""
	}
	overScrollbar := e.scrollbar.Visible() && mx == e.scrollbar.X &&
		my >= e.scrollbar.Y && my < e.scrollbar.Y+e.scrollbar.Height
	overHScrollbar := e.hscrollbar.Visible() && my == e.hscrollbar.Y &&
		mx >= e.hscrollbar.X && mx < e.hscrollbar.X+e.hscrollbar.Width
	if overScrollbar || overHScrollbar || mx < r.X+e.GutterWidth() {
		return term.PointerDefault
	}
	if e.rowAt(my - r.Y).isPhantom() {
		return term.PointerDefault
	}
	return term.PointerText
}

func (v *VerticalTabBar) PointerShapeAt(mx, my int) string {
	r := v.GetRect()
	if r.H > 0 && !r.Contains(mx, my) {
		return ""
	}
	ly := my - r.Y
	if ly >= 0 && ly < v.Count {
		return term.PointerHand
	}
	return term.PointerDefault
}

func (tp *TerminalPanelWidget) PointerShapeAt(mx, my int) string {
	if tp.activeOwnsPointer() {
		return widgets.PointerShapeAt(tp.ActiveWidget(), mx, my)
	}
	r := tp.GetRect()
	if !r.Contains(mx, my) {
		return ""
	}
	stripW := min(VerticalTabBarWidth, max(r.W, 0))
	if mx < r.X+stripW {
		return tp.TabBar.PointerShapeAt(mx, my)
	}
	if tw, ok := tp.ActiveWidget().(*TerminalWidget); ok {
		if tw.scrollbar.Visible() && mx == tw.scrollbar.X && my >= tw.scrollbar.Y && my < tw.scrollbar.Y+tw.scrollbar.Height {
			return term.PointerDefault
		}
	}
	if w := tp.ActiveWidget(); w != nil {
		if shape := widgets.PointerShapeAt(w, mx, my); shape != "" {
			return shape
		}
	}
	return term.PointerDefault
}

func (tw *TerminalWidget) PointerShapeAt(mx, my int) string {
	if tw.scrollbar.IsDragging() {
		return term.PointerDefault
	}
	if tw.scrollbar.Visible() && mx == tw.scrollbar.X && my >= tw.scrollbar.Y && my < tw.scrollbar.Y+tw.scrollbar.Height {
		return term.PointerDefault
	}
	r := tw.GetRect()
	if !r.Contains(mx, my) {
		return ""
	}
	if tw.ctrlHeld {
		pos := tw.screenToLine(mx, my)
		if tw.linkAt(pos.Line, pos.Col) != nil {
			return term.PointerHand
		}
	}
	return term.PointerDefault
}
