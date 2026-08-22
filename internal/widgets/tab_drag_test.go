package widgets

import "testing"

func TestTabDragStateRequiresIntentionalMovement(t *testing.T) {
	var drag TabDragState
	drag.Begin(1, 10)
	if drag.Update(11, 2) {
		t.Fatal("one-column jitter should not start a drag")
	}
	if !drag.Update(12, 2) {
		t.Fatal("two-column movement should start a drag")
	}
	from, to, dragged := drag.End()
	if !dragged || from != 1 || to != 2 {
		t.Fatalf("drag result = %d -> %d, dragged=%v", from, to, dragged)
	}
	if drag.Active() {
		t.Fatal("End should reset the gesture")
	}
}
