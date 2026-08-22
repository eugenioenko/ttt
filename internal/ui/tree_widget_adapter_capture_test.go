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

func TestWidgetAdapterRetainsFocusRoutedCaptureAcrossChildren(t *testing.T) {
	first := &adapterCaptureProbe{height: 2}
	second := &adapterCaptureProbe{height: 2}
	stack := widgets.NewVStackWidget(first, second)
	adapter := NewWidgetAdapter(stack)
	adapter.SetRect(Rect{X: 0, Y: 0, W: 10, H: 4})
	adapter.Render(NewRenderSurface(makeGrid(10, 4), Rect{X: 0, Y: 0, W: 10, H: 4}))
	invalidations := 0
	adapter.SetPointerCaptureInvalidated(func() { invalidations++ })

	if got := adapter.HandleEvent(tcell.NewEventMouse(1, 0, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("press result = %v, want captured", got)
	}
	if got := adapter.HandleEvent(tcell.NewEventMouse(1, 3, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("cross-child drag result = %v, want captured", got)
	}
	if first.heldEvents != 1 || second.capturing {
		t.Fatalf("capture moved between children: first held=%d second capturing=%v", first.heldEvents, second.capturing)
	}
	adapter.HandleEvent(tcell.NewEventMouse(1, 3, tcell.ButtonNone, 0))
	if adapter.rootCaptured || first.capturing {
		t.Fatal("release retained adapter capture")
	}
	if invalidations != 0 {
		t.Fatalf("normal release invalidations = %d, want 0", invalidations)
	}

	adapter.HandleEvent(tcell.NewEventMouse(1, 0, tcell.Button1, 0))
	if !adapter.CancelPointerCapture() {
		t.Fatal("adapter did not cancel capture")
	}
	if invalidations != 1 {
		t.Fatalf("capture invalidations = %d, want 1", invalidations)
	}
}
