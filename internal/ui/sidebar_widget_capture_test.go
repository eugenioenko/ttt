package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type sidebarCaptureProbe struct {
	BaseWidget
	capturing       bool
	cancelCount     int
	invalidateCount int
	releaseCount    int
}

func TestSidebarCancelStopsTreeScrollbarDrag(t *testing.T) {
	items := make([]*widgets.TreeNode, 20)
	for i := range items {
		items[i] = &widgets.TreeNode{ID: string(rune('a' + i)), Label: "item"}
	}
	tree := widgets.NewTreeWidget(widgets.TreeConfig{Items: items})
	adapter := NewWidgetAdapter(&widgets.BoxWidget{Child: tree})
	sidebar := NewSidebarWidget()
	sidebar.AddPanel("tree", "Tree", adapter)
	split := NewSplitPanelWidget()
	split.Left = sidebar
	split.Right = &mockWidget{}
	split.DividerPos = 20
	split.ShowLeft = true
	root := NewRoot(widgets.NewVStackWidget(split))
	root.SetSize(50, 10)
	root.Render(makeGrid(50, 10))
	outerInvalidated := sidebar.pointerCaptureInvalidated
	invalidations := 0
	sidebar.SetPointerCaptureInvalidated(func() {
		invalidations++
		outerInvalidated()
	})

	scrollbarX := sidebar.GetRect().X + sidebar.GetRect().W - 1
	scrollbarY := sidebar.GetRect().Y + 2
	if got := root.HandleEvent(tcell.NewEventMouse(scrollbarX, scrollbarY, tcell.Button1, 0)); got != EventConsumed {
		t.Fatalf("scrollbar press result = %v, want root-consumed capture", got)
	}
	if !root.PointerCaptureActive() || split.capturedChild != sidebar {
		t.Fatal("scrollbar press did not establish the outer capture chain")
	}
	if !sidebar.CancelPointerCapture() {
		t.Fatal("sidebar did not cancel tree scrollbar capture")
	}
	if root.PointerCaptureActive() || split.capturedChild != nil || tree.OwnsPointerCapture() || adapter.OwnsPointerCapture() || sidebar.OwnsPointerCapture() {
		t.Fatal("tree scrollbar capture survived sidebar cancellation")
	}
	if invalidations != 1 {
		t.Fatalf("nested capture invalidations = %d, want 1", invalidations)
	}
	if got := sidebar.HandleEvent(tcell.NewEventMouse(30, 30, tcell.Button1, 0)); got != EventIgnored {
		t.Fatalf("post-cancel out-of-bounds drag result = %v, want ignored", got)
	}
}

func (p *sidebarCaptureProbe) Height() int    { return 0 }
func (p *sidebarCaptureProbe) Width() int     { return 0 }
func (p *sidebarCaptureProbe) Render(Surface) {}
func (p *sidebarCaptureProbe) HandleEvent(ev tcell.Event) EventResult {
	mouse, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}
	if mouse.Buttons() == tcell.ButtonNone {
		p.capturing = false
		p.releaseCount++
		return EventConsumed
	}
	if mouse.Buttons()&tcell.Button1 != 0 {
		p.capturing = true
		return EventCaptured
	}
	return EventIgnored
}
func (p *sidebarCaptureProbe) CancelPointerCapture() bool {
	wasCapturing := p.capturing
	p.capturing = false
	p.cancelCount++
	return wasCapturing
}
func (p *sidebarCaptureProbe) InvalidatePointerInteraction() bool {
	wasCapturing := p.capturing
	p.capturing = false
	p.invalidateCount++
	return wasCapturing
}
func (p *sidebarCaptureProbe) OwnsPointerCapture() bool { return p.capturing }

func TestSidebarTracksAndCancelsCapturedPanel(t *testing.T) {
	panel := &sidebarCaptureProbe{}
	sidebar := NewSidebarWidget()
	sidebar.AddPanel("panel", "Panel", panel)
	sidebar.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})
	invalidations := 0
	sidebar.SetPointerCaptureInvalidated(func() { invalidations++ })

	if got := sidebar.HandleEvent(tcell.NewEventMouse(2, 3, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("press result = %v, want captured", got)
	}
	if !sidebar.OwnsPointerCapture() {
		t.Fatal("sidebar did not report its captured panel")
	}
	if !sidebar.CancelPointerCapture() {
		t.Fatal("sidebar did not report canceled panel capture")
	}
	if sidebar.capturedChild != nil || sidebar.OwnsPointerCapture() {
		t.Fatal("sidebar retained canceled panel capture")
	}
	if panel.cancelCount != 1 {
		t.Fatalf("panel cancellation count = %d, want 1", panel.cancelCount)
	}
	if invalidations != 1 {
		t.Fatalf("capture invalidations = %d, want 1", invalidations)
	}
}

func TestSidebarInvalidatesCapturedPanelAndPreservesNormalRelease(t *testing.T) {
	panel := &sidebarCaptureProbe{}
	sidebar := NewSidebarWidget()
	sidebar.AddPanel("panel", "Panel", panel)
	sidebar.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})

	sidebar.HandleEvent(tcell.NewEventMouse(2, 3, tcell.Button1, 0))
	if !sidebar.InvalidatePointerInteraction() {
		t.Fatal("sidebar did not report invalidated panel capture")
	}
	if panel.invalidateCount != 1 || sidebar.capturedChild != nil {
		t.Fatalf("invalidated panel = %d, captured child = %v", panel.invalidateCount, sidebar.capturedChild)
	}

	sidebar.HandleEvent(tcell.NewEventMouse(2, 3, tcell.Button1, 0))
	if got := sidebar.HandleEvent(tcell.NewEventMouse(30, 30, tcell.ButtonNone, 0)); got != EventConsumed {
		t.Fatalf("release result = %v, want consumed", got)
	}
	if panel.releaseCount != 1 || sidebar.capturedChild != nil {
		t.Fatalf("release count = %d, captured child = %v", panel.releaseCount, sidebar.capturedChild)
	}
}
