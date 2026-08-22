package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestBoxForwardsTreeScrollbarCaptureLifecycle(t *testing.T) {
	items := make([]*TreeNode, 10)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('a' + i)), Label: "item"}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	box := &BoxWidget{Child: tree}
	box.SetRect(Rect{X: 0, Y: 0, W: 5, H: 3})
	box.Render(newVirtualSurface(5, 3))
	invalidations := 0
	box.SetPointerCaptureInvalidated(func() { invalidations++ })

	if got := box.HandleEvent(tcell.NewEventMouse(4, 0, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("boxed scrollbar press result = %v, want captured", got)
	}
	if !box.OwnsPointerCapture() || !box.CancelPointerCapture() {
		t.Fatal("box did not forward tree scrollbar capture and cancellation")
	}
	if tree.OwnsPointerCapture() || box.OwnsPointerCapture() {
		t.Fatal("boxed tree retained canceled scrollbar capture")
	}
	if invalidations != 1 {
		t.Fatalf("capture invalidations = %d, want 1", invalidations)
	}

	box.HandleEvent(tcell.NewEventMouse(4, 0, tcell.Button1, 0))
	if !box.InvalidatePointerInteraction() || tree.OwnsPointerCapture() {
		t.Fatal("box did not forward tree scrollbar invalidation")
	}
	if invalidations != 2 {
		t.Fatalf("capture invalidations = %d, want 2", invalidations)
	}
}
