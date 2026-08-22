package ui

import (
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type WidgetAdapter struct {
	BaseWidget
	W                         widgets.Widget
	focus                     *widgets.FocusManager
	popups                    []widgets.PopupRenderer
	capturedWidget            widgets.Widget
	pointerCaptureInvalidated func()
	cancelingPointerCapture   bool
}

func NewWidgetAdapter(w widgets.Widget) *WidgetAdapter {
	wa := &WidgetAdapter{W: w, focus: widgets.NewFocusManager()}
	wa.focus.Collect(w)
	wa.collectPopups()
	wa.wireTabbedCallbacks(w)
	return wa
}

// EnableScrollIntoView keeps the focused widget on screen while tabbing through
// content taller than its scroll view. Opt-in so existing panels and dialogs
// keep their current scrolling behaviour.
func (a *WidgetAdapter) EnableScrollIntoView() {
	a.focus.OnFocusChange = func(fw widgets.FocusableWidget) {
		widgets.ScrollIntoView(a.W, fw)
	}
}

// Popup-bearing widgets are cached rather than rediscovered each frame, and
// refreshed alongside focus whenever the tree changes.
func (a *WidgetAdapter) collectPopups() {
	a.popups = nil
	var walk func(widgets.Widget)
	walk = func(w widgets.Widget) {
		if pr, ok := w.(widgets.PopupRenderer); ok {
			a.popups = append(a.popups, pr)
		}
		if cw, ok := w.(widgets.ContainerWidget); ok {
			for _, child := range cw.WidgetChildren() {
				walk(child)
			}
		}
	}
	walk(a.W)
}

func (a *WidgetAdapter) wireTabbedCallbacks(w widgets.Widget) {
	switch v := w.(type) {
	case *widgets.TabbedWidget:
		v.OnChange = func(int) { a.RebuildFocus() }
		for _, child := range v.Children {
			a.wireTabbedCallbacks(child)
		}
	case *widgets.VStackWidget:
		for _, child := range v.Children {
			a.wireTabbedCallbacks(child)
		}
	case *widgets.HStackWidget:
		for _, child := range v.Children {
			a.wireTabbedCallbacks(child)
		}
	case *widgets.BoxWidget:
		if v.Child != nil {
			a.wireTabbedCallbacks(v.Child)
		}
	case *widgets.ScrollViewWidget:
		if v.Child != nil {
			a.wireTabbedCallbacks(v.Child)
		}
	case *ContentSplitWidget:
		if v.Top != nil {
			a.wireTabbedCallbacks(v.Top)
		}
		if v.Bottom != nil {
			a.wireTabbedCallbacks(v.Bottom)
		}
	}
}

func (a *WidgetAdapter) Inner() widgets.Widget { return a.W }

func (a *WidgetAdapter) FocusedWidget() widgets.Widget { return a.focus.Focused() }

func (a *WidgetAdapter) Focusable() bool { return true }

func (a *WidgetAdapter) SetFocused(focused bool) {
	a.focus.SetActive(focused)
}

func (a *WidgetAdapter) Render(surface Surface) {
	r := a.GetRect()
	a.W.SetRect(Rect{X: r.X, Y: r.Y, W: r.W, H: r.H})
	a.W.Render(surface)

	// Popups draw after the tree so they overlay it. Every popup-bearing widget is
	// considered rather than only the focused one, so ownership of a popup never
	// has to be inferred from where focus happens to sit.
	bounds := Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
	for _, pr := range a.popups {
		if pb, ok := pr.(widgets.PopupBounder); ok {
			pb.SetPopupBounds(bounds)
		}
		if !pr.HasPopup() {
			continue
		}
		rect := pr.PopupRect()
		pr.RenderPopup(surface.Sub(Rect{
			X: rect.X - bounds.X, Y: rect.Y - bounds.Y, W: rect.W, H: rect.H,
		}))
	}
}

// popupAt returns the widget whose open popup covers the point, if any. Bounds
// come from the same list Render draws from, so hit testing and painting cannot
// disagree about where a popup is.
func (a *WidgetAdapter) popupAt(mx, my int) widgets.Widget {
	for _, pr := range a.popups {
		if !pr.HasPopup() {
			continue
		}
		r := pr.PopupRect()
		if mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H {
			if w, ok := pr.(widgets.Widget); ok {
				return w
			}
		}
	}
	return nil
}

func (a *WidgetAdapter) RebuildFocus() {
	a.focus.Collect(a.W)
	a.collectPopups()
}

func (a *WidgetAdapter) RewireTabbedCallbacks() {
	a.wireTabbedCallbacks(a.W)
}

func (a *WidgetAdapter) CursorPosition() (int, int, bool) {
	if fw := a.focus.Focused(); fw != nil {
		if cp, ok := fw.(widgets.CursorPositioner); ok {
			return cp.CursorPosition()
		}
	}
	return 0, 0, false
}

func (a *WidgetAdapter) HandleEvent(ev tcell.Event) EventResult {
	if tev, ok := ev.(*tcell.EventMouse); ok && a.capturedWidget != nil {
		result := a.capturedWidget.HandleEvent(ev)
		if tev.Buttons() == tcell.ButtonNone {
			a.capturedWidget = nil
		}
		return result
	}
	// Popups are painted over the tree, so they must claim clicks over the rows
	// they cover before those rows get a chance at them.
	if tev, ok := ev.(*tcell.EventMouse); ok {
		if w := a.popupAt(tev.Position()); w != nil {
			result := w.HandleEvent(ev)
			if result == EventCaptured {
				a.captureWidget(w)
			}
			return result
		}
	}
	if result := a.focus.HandleEvent(ev); result != EventIgnored {
		if result == EventCaptured {
			a.captureWidget(a.focus.LastEventTarget())
		}
		return result
	}
	result := a.W.HandleEvent(ev)
	if result == EventCaptured {
		a.captureWidget(a.W)
	}
	return result
}

func (a *WidgetAdapter) captureWidget(owner widgets.Widget) {
	if owner == nil {
		return
	}
	a.capturedWidget = owner
	widgets.SetPointerCaptureInvalidated(owner, func() {
		if a.capturedWidget != owner || a.cancelingPointerCapture {
			return
		}
		a.capturedWidget = nil
		if a.pointerCaptureInvalidated != nil {
			a.pointerCaptureInvalidated()
		}
	})
}

func (a *WidgetAdapter) CancelPointerCapture() bool {
	captured := a.capturedWidget
	canceled := captured != nil
	a.capturedWidget = nil
	a.cancelingPointerCapture = true
	if captured != nil {
		canceled = widgets.CancelPointerCapture(captured) || canceled
	}
	a.cancelingPointerCapture = false
	if canceled && a.pointerCaptureInvalidated != nil {
		a.pointerCaptureInvalidated()
	}
	return canceled
}

func (a *WidgetAdapter) OwnsPointerCapture() bool {
	if a.capturedWidget == nil {
		return false
	}
	owner, ok := a.capturedWidget.(widgets.PointerCaptureOwner)
	if !ok || owner.OwnsPointerCapture() {
		return true
	}
	a.capturedWidget = nil
	return false
}

func (a *WidgetAdapter) InvalidatePointerInteraction() bool {
	captured := a.capturedWidget
	invalidated := captured != nil
	a.capturedWidget = nil
	a.cancelingPointerCapture = true
	if captured != nil {
		invalidated = widgets.InvalidatePointerInteraction(captured) || invalidated
	}
	a.cancelingPointerCapture = false
	if invalidated && a.pointerCaptureInvalidated != nil {
		a.pointerCaptureInvalidated()
	}
	return invalidated
}

func (a *WidgetAdapter) SetPointerCaptureInvalidated(invalidated func()) {
	a.pointerCaptureInvalidated = invalidated
}
