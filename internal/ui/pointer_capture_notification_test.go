package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

type notificationCaptureProbe struct {
	BaseWidget
	active      bool
	invalidated func()
}

func (p *notificationCaptureProbe) Height() int    { return 0 }
func (p *notificationCaptureProbe) Width() int     { return 0 }
func (p *notificationCaptureProbe) Render(Surface) {}
func (p *notificationCaptureProbe) HandleEvent(ev tcell.Event) EventResult {
	mouse, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}
	if mouse.Buttons() == tcell.ButtonNone {
		p.active = false
		return EventConsumed
	}
	if mouse.Buttons()&tcell.Button1 != 0 {
		p.active = true
		return EventCaptured
	}
	return EventIgnored
}
func (p *notificationCaptureProbe) CancelPointerCapture() bool {
	active := p.active
	p.active = false
	if active && p.invalidated != nil {
		p.invalidated()
	}
	return active
}
func (p *notificationCaptureProbe) InvalidatePointerInteraction() bool {
	return p.CancelPointerCapture()
}
func (p *notificationCaptureProbe) OwnsPointerCapture() bool { return p.active }
func (p *notificationCaptureProbe) SetPointerCaptureInvalidated(invalidated func()) {
	p.invalidated = invalidated
}

func TestSplitPanelCancellationNotifiesOnce(t *testing.T) {
	child := &notificationCaptureProbe{}
	split := NewSplitPanelWidget()
	split.ShowLeft = false
	split.Right = child
	split.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})
	invalidations := 0
	split.SetPointerCaptureInvalidated(func() { invalidations++ })

	split.HandleEvent(tcell.NewEventMouse(5, 5, tcell.Button1, 0))
	if !split.CancelPointerCapture() {
		t.Fatal("split did not cancel child capture")
	}
	if invalidations != 1 {
		t.Fatalf("split cancellation invalidations = %d, want 1", invalidations)
	}

	invalidations = 0
	split.HandleEvent(tcell.NewEventMouse(5, 5, tcell.Button1, 0))
	split.HandleEvent(tcell.NewEventMouse(5, 5, tcell.ButtonNone, 0))
	if invalidations != 0 {
		t.Fatalf("split release invalidations = %d, want 0", invalidations)
	}

	split.HandleEvent(tcell.NewEventMouse(5, 5, tcell.Button1, 0))
	if !split.InvalidatePointerInteraction() {
		t.Fatal("split did not invalidate child capture")
	}
	if invalidations != 1 {
		t.Fatalf("split invalidation notifications = %d, want 1", invalidations)
	}
}

func TestContentSplitCancellationNotifiesOnce(t *testing.T) {
	child := &notificationCaptureProbe{}
	split := NewContentSplitWidget()
	split.Top = child
	split.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})
	invalidations := 0
	split.SetPointerCaptureInvalidated(func() { invalidations++ })

	split.HandleEvent(tcell.NewEventMouse(5, 5, tcell.Button1, 0))
	if !split.InvalidatePointerInteraction() {
		t.Fatal("content split did not invalidate child capture")
	}
	if invalidations != 1 {
		t.Fatalf("content split invalidation notifications = %d, want 1", invalidations)
	}

	invalidations = 0
	split.HandleEvent(tcell.NewEventMouse(5, 5, tcell.Button1, 0))
	split.HandleEvent(tcell.NewEventMouse(5, 5, tcell.ButtonNone, 0))
	if invalidations != 0 {
		t.Fatalf("content split release invalidations = %d, want 0", invalidations)
	}

	split.HandleEvent(tcell.NewEventMouse(5, 5, tcell.Button1, 0))
	if !split.CancelPointerCapture() {
		t.Fatal("content split did not cancel child capture")
	}
	if invalidations != 1 {
		t.Fatalf("content split cancellation notifications = %d, want 1", invalidations)
	}
}
