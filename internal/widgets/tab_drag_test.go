package widgets

import "testing"

func TestTabDragStateRequiresIntentionalMovement(t *testing.T) {
	var drag TabDragState
	drag.Begin(1, "b", 10)
	if drag.SourceID() != "b" {
		t.Fatalf("source ID = %q, want b", drag.SourceID())
	}
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

func TestTabDragStateRecognizesVerticalMovement(t *testing.T) {
	var drag TabDragState
	drag.BeginAt(1, "b", 10, 5)
	if drag.UpdateAt(10, 6, 1) {
		t.Fatal("one-row jitter started a drag")
	}
	if !drag.UpdateAt(10, 7, 1) {
		t.Fatal("vertical movement did not start a drag")
	}
	from, _, dragged := drag.End()
	if !dragged || from != 1 || drag.Active() {
		t.Fatal("vertical drag did not finish cleanly")
	}
}
