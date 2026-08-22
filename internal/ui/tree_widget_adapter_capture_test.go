package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type adapterCaptureProbe struct {
	widgets.BaseWidget
	height     int
	focused    bool
	capturing  bool
	heldEvents int
}

type adapterPopupCaptureProbe struct {
	adapterCaptureProbe
	popup Rect
}

type adapterRootCaptureProbe struct {
	widgets.BaseWidget
	capturing  bool
	heldEvents int
}

func (p *adapterRootCaptureProbe) Height() int            { return 0 }
func (p *adapterRootCaptureProbe) Width() int             { return 0 }
func (p *adapterRootCaptureProbe) Render(widgets.Surface) {}
func (p *adapterRootCaptureProbe) HandleEvent(ev tcell.Event) widgets.EventResult {
	mouse, ok := ev.(*tcell.EventMouse)
	if !ok {
		return widgets.EventIgnored
	}
	if mouse.Buttons() == tcell.ButtonNone {
		p.capturing = false
		return widgets.EventConsumed
	}
	if mouse.Buttons()&tcell.Button1 != 0 {
		if p.capturing {
			p.heldEvents++
		}
		p.capturing = true
		return widgets.EventCaptured
	}
	return widgets.EventIgnored
}
func (p *adapterRootCaptureProbe) CancelPointerCapture() bool {
	active := p.capturing
	p.capturing = false
	return active
}
func (p *adapterRootCaptureProbe) OwnsPointerCapture() bool { return p.capturing }

func (p *adapterPopupCaptureProbe) HasPopup() bool      { return true }
func (p *adapterPopupCaptureProbe) PopupRect() Rect     { return p.popup }
func (p *adapterPopupCaptureProbe) RenderPopup(Surface) {}

func (p *adapterCaptureProbe) Height() int             { return p.height }
func (p *adapterCaptureProbe) Width() int              { return 0 }
func (p *adapterCaptureProbe) Render(widgets.Surface)  {}
func (p *adapterCaptureProbe) Focusable() bool         { return true }
func (p *adapterCaptureProbe) SetFocused(focused bool) { p.focused = focused }
func (p *adapterCaptureProbe) IsFocused() bool         { return p.focused }
func (p *adapterCaptureProbe) HandleEvent(ev tcell.Event) widgets.EventResult {
	mouse, ok := ev.(*tcell.EventMouse)
	if !ok {
		return widgets.EventIgnored
	}
	if mouse.Buttons() == tcell.ButtonNone {
		p.capturing = false
		return widgets.EventConsumed
	}
	if mouse.Buttons()&tcell.Button1 != 0 {
		if p.capturing {
			p.heldEvents++
		}
		p.capturing = true
		return widgets.EventCaptured
	}
	return widgets.EventIgnored
}

func (p *adapterCaptureProbe) CancelPointerCapture() bool {
	active := p.capturing
	p.capturing = false
	return active
}

func (p *adapterCaptureProbe) OwnsPointerCapture() bool { return p.capturing }

func (p *adapterCaptureProbe) InvalidatePointerInteraction() bool {
	return p.CancelPointerCapture()
}

func TestWidgetAdapterRetainsFocusRoutedCaptureAcrossChildren(t *testing.T) {
	first := &adapterCaptureProbe{height: 2}
	second := &adapterCaptureProbe{height: 2}
	stack := widgets.NewVStackWidget(first, second)
	adapter := NewWidgetAdapter(stack)
	adapter.SetRect(Rect{X: 0, Y: 0, W: 10, H: 4})
	adapter.Render(NewRenderSurface(makeGrid(10, 4), Rect{X: 0, Y: 0, W: 10, H: 4}))
	invalidations := 0
	adapter.SetPointerCaptureInvalidated(func() { invalidations++ })

	if got := adapter.HandleEvent(tcell.NewEventMouse(1, 3, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("press result = %v, want captured", got)
	}
	if got := adapter.HandleEvent(tcell.NewEventMouse(1, 0, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("cross-child drag result = %v, want captured", got)
	}
	if second.heldEvents != 1 || first.capturing {
		t.Fatalf("capture moved between children: second held=%d first capturing=%v", second.heldEvents, first.capturing)
	}
	adapter.HandleEvent(tcell.NewEventMouse(1, 0, tcell.ButtonNone, 0))
	if adapter.capturedWidget != nil || second.capturing {
		t.Fatal("release retained adapter capture")
	}
	if invalidations != 0 {
		t.Fatalf("normal release invalidations = %d, want 0", invalidations)
	}

	adapter.HandleEvent(tcell.NewEventMouse(1, 3, tcell.Button1, 0))
	if !adapter.CancelPointerCapture() {
		t.Fatal("adapter did not cancel capture")
	}
	if invalidations != 1 {
		t.Fatalf("capture invalidations = %d, want 1", invalidations)
	}

	adapter.HandleEvent(tcell.NewEventMouse(1, 3, tcell.Button1, 0))
	if !adapter.InvalidatePointerInteraction() || second.capturing {
		t.Fatal("adapter did not invalidate exact capture owner")
	}
	if invalidations != 2 {
		t.Fatalf("capture invalidations = %d, want 2", invalidations)
	}
}

func TestWidgetAdapterRetainsPopupCaptureOutsidePopup(t *testing.T) {
	popup := &adapterPopupCaptureProbe{
		adapterCaptureProbe: adapterCaptureProbe{height: 2},
		popup:               Rect{X: 0, Y: 2, W: 10, H: 2},
	}
	other := &adapterCaptureProbe{height: 2}
	stack := widgets.NewVStackWidget(other, popup)
	adapter := NewWidgetAdapter(stack)
	adapter.SetRect(Rect{X: 0, Y: 0, W: 10, H: 4})
	adapter.Render(NewRenderSurface(makeGrid(10, 4), Rect{X: 0, Y: 0, W: 10, H: 4}))

	if got := adapter.HandleEvent(tcell.NewEventMouse(1, 3, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("popup press result = %v, want captured", got)
	}
	adapter.HandleEvent(tcell.NewEventMouse(1, 0, tcell.Button1, 0))
	if popup.heldEvents != 1 || other.capturing {
		t.Fatalf("popup capture moved to underlying child: popup held=%d other capturing=%v", popup.heldEvents, other.capturing)
	}
	adapter.HandleEvent(tcell.NewEventMouse(1, 0, tcell.ButtonNone, 0))
	if popup.capturing {
		t.Fatal("popup owner did not receive release")
	}
}

func TestWidgetAdapterPreservesDirectRootCapture(t *testing.T) {
	root := &adapterRootCaptureProbe{}
	adapter := NewWidgetAdapter(root)
	adapter.SetRect(Rect{X: 0, Y: 0, W: 10, H: 4})
	adapter.Render(NewRenderSurface(makeGrid(10, 4), Rect{X: 0, Y: 0, W: 10, H: 4}))

	if got := adapter.HandleEvent(tcell.NewEventMouse(1, 1, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("root press result = %v, want captured", got)
	}
	adapter.HandleEvent(tcell.NewEventMouse(20, 20, tcell.Button1, 0))
	if root.heldEvents != 1 {
		t.Fatalf("root held events = %d, want 1", root.heldEvents)
	}
	adapter.HandleEvent(tcell.NewEventMouse(20, 20, tcell.ButtonNone, 0))
	if root.capturing || adapter.capturedWidget != nil {
		t.Fatal("root release retained capture")
	}
}
