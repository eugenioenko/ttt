package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestTreeScrollbarCaptureCanBeCanceled(t *testing.T) {
	items := make([]*TreeNode, 10)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('a' + i)), Label: "item"}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	tree.SetRect(Rect{X: 0, Y: 0, W: 5, H: 3})
	tree.Render(newVirtualSurface(5, 3))
	invalidations := 0
	tree.SetPointerCaptureInvalidated(func() { invalidations++ })

	if got := tree.HandleEvent(tcell.NewEventMouse(4, 0, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("scrollbar press result = %v, want captured", got)
	}
	if !tree.OwnsPointerCapture() || !tree.CancelPointerCapture() {
		t.Fatal("tree did not own and cancel its scrollbar capture")
	}
	if tree.OwnsPointerCapture() {
		t.Fatal("tree retained canceled scrollbar capture")
	}
	if invalidations != 1 {
		t.Fatalf("capture invalidations = %d, want 1", invalidations)
	}
	if tree.CancelPointerCapture() || invalidations != 1 {
		t.Fatalf("idle cancellation changed state: invalidated=%d", invalidations)
	}
	if got := tree.HandleEvent(tcell.NewEventMouse(9, 9, tcell.Button1, 0)); got != EventIgnored {
		t.Fatalf("post-cancel out-of-bounds drag result = %v, want ignored", got)
	}
}

func TestTreeScrollbarUsesStoredScreenGeometryOnVirtualSurface(t *testing.T) {
	items := make([]*TreeNode, 10)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('a' + i)), Label: "item"}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	tree.SetRect(Rect{X: 20, Y: 30, W: 5, H: 3})
	tree.Render(newVirtualSurface(5, 3))

	if got := tree.HandleEvent(tcell.NewEventMouse(4, 0, tcell.Button1, 0)); got != EventIgnored {
		t.Fatalf("render-local coordinate result = %v, want ignored", got)
	}
	if got := tree.HandleEvent(tcell.NewEventMouse(24, 30, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("stored screen coordinate result = %v, want captured", got)
	}
	if got := tree.HandleEvent(tcell.NewEventMouse(50, 50, tcell.ButtonNone, 0)); got != EventConsumed {
		t.Fatalf("off-widget release result = %v, want consumed", got)
	}
}
