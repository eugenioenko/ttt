package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestSelectableListKeyNav(t *testing.T) {
	sl := &SelectableList{}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}
	items := 5

	// Down
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyDown, "", 0), r, items)
	if sl.Selected != 1 {
		t.Fatalf("expected 1, got %d", sl.Selected)
	}

	// Down x3
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyDown, "", 0), r, items)
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyDown, "", 0), r, items)
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyDown, "", 0), r, items)
	if sl.Selected != 4 {
		t.Fatalf("expected 4, got %d", sl.Selected)
	}

	// Down past end — clamped
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyDown, "", 0), r, items)
	if sl.Selected != 4 {
		t.Fatalf("expected 4 (clamped), got %d", sl.Selected)
	}

	// Up
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyUp, "", 0), r, items)
	if sl.Selected != 3 {
		t.Fatalf("expected 3, got %d", sl.Selected)
	}

	// Up past start — clamped
	sl.Selected = 0
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyUp, "", 0), r, items)
	if sl.Selected != 0 {
		t.Fatalf("expected 0 (clamped), got %d", sl.Selected)
	}
}

func TestSelectableListEnter(t *testing.T) {
	sl := &SelectableList{Selected: 2}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}

	res := sl.HandleListEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0), r, 5)
	if res.Result != EventConsumed {
		t.Fatal("Enter should be consumed")
	}
	if res.Action != ListActionActivate {
		t.Fatal("Enter should produce ListActionActivate")
	}
}

func TestSelectableListClick(t *testing.T) {
	sl := &SelectableList{}
	r := Rect{X: 5, Y: 10, W: 20, H: 10}

	// Click at screen y=12 → localY=2, idx=2
	res := sl.HandleListEvent(tcell.NewEventMouse(8, 12, tcell.Button1, 0), r, 5)
	if res.Result != EventConsumed {
		t.Fatal("click should be consumed")
	}
	if res.Action != ListActionActivate {
		t.Fatal("click should produce ListActionActivate")
	}
	if sl.Selected != 2 {
		t.Fatalf("expected selected 2, got %d", sl.Selected)
	}
}

func TestSelectableListRightClick(t *testing.T) {
	sl := &SelectableList{}
	r := Rect{X: 5, Y: 10, W: 20, H: 10}

	res := sl.HandleListEvent(tcell.NewEventMouse(8, 11, tcell.Button2, 0), r, 5)
	if res.Result != EventConsumed {
		t.Fatal("right-click should be consumed")
	}
	if res.Action != ListActionContext {
		t.Fatal("right-click should produce ListActionContext")
	}
	if sl.Selected != 1 {
		t.Fatalf("expected selected 1, got %d", sl.Selected)
	}
	if res.ScreenX != 8 || res.ScreenY != 11 {
		t.Fatalf("expected screen coords (8,11), got (%d,%d)", res.ScreenX, res.ScreenY)
	}
}

func TestSelectableListClickOutOfRange(t *testing.T) {
	sl := &SelectableList{}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}

	// Click at y=7, but only 3 items
	res := sl.HandleListEvent(tcell.NewEventMouse(5, 7, tcell.Button1, 0), r, 3)
	if res.Result != EventConsumed {
		t.Fatal("click should be consumed even out of range")
	}
	if res.Action != ListActionNone {
		t.Fatal("out-of-range click should not activate")
	}
	if sl.Selected != 0 {
		t.Fatalf("selected should not change, got %d", sl.Selected)
	}
}

func TestSelectableListWheelScroll(t *testing.T) {
	sl := &SelectableList{ScrollTop: 5}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}

	sl.HandleListEvent(tcell.NewEventMouse(0, 0, tcell.WheelUp, 0), r, 100)
	if sl.ScrollTop != 2 {
		t.Fatalf("expected ScrollTop 2, got %d", sl.ScrollTop)
	}

	// Wheel up past 0
	sl.HandleListEvent(tcell.NewEventMouse(0, 0, tcell.WheelUp, 0), r, 100)
	if sl.ScrollTop != 0 {
		t.Fatalf("expected ScrollTop 0, got %d", sl.ScrollTop)
	}

	// Wheel down
	sl.HandleListEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, 0), r, 100)
	if sl.ScrollTop != 3 {
		t.Fatalf("expected ScrollTop 3, got %d", sl.ScrollTop)
	}

	// Wheel down clamp to max
	sl.ScrollTop = 95
	sl.HandleListEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, 0), r, 100)
	if sl.ScrollTop != 90 {
		t.Fatalf("expected ScrollTop 90 (max), got %d", sl.ScrollTop)
	}
}

func TestSelectableListEnsureVisible(t *testing.T) {
	sl := &SelectableList{Selected: 15, ScrollTop: 0}

	sl.EnsureVisible(10)
	if sl.ScrollTop != 6 {
		t.Fatalf("expected ScrollTop 6, got %d", sl.ScrollTop)
	}

	// Selected above scroll
	sl.Selected = 2
	sl.EnsureVisible(10)
	if sl.ScrollTop != 2 {
		t.Fatalf("expected ScrollTop 2, got %d", sl.ScrollTop)
	}
}

func TestSelectableListClampSelected(t *testing.T) {
	sl := &SelectableList{Selected: 10}
	sl.ClampSelected(5)
	if sl.Selected != 4 {
		t.Fatalf("expected 4, got %d", sl.Selected)
	}

	sl.Selected = -1
	sl.ClampSelected(5)
	if sl.Selected != 0 {
		t.Fatalf("expected 0, got %d", sl.Selected)
	}

	// Empty list
	sl.Selected = 3
	sl.ClampSelected(0)
	if sl.Selected != 0 {
		t.Fatalf("expected 0 for empty list, got %d", sl.Selected)
	}
}

func TestSelectableListPageDown(t *testing.T) {
	sl := &SelectableList{Selected: 0, ScrollTop: 0}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}
	items := 50

	// PageDown moves by page size (rect.H = 10)
	res := sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, 0), r, items)
	if res.Result != EventConsumed {
		t.Fatal("PageDown should be consumed")
	}
	if sl.Selected != 10 {
		t.Fatalf("expected Selected 10, got %d", sl.Selected)
	}

	// PageDown again
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, 0), r, items)
	if sl.Selected != 20 {
		t.Fatalf("expected Selected 20, got %d", sl.Selected)
	}

	// PageDown near end — clamps to last item
	sl.Selected = 45
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, 0), r, items)
	if sl.Selected != 49 {
		t.Fatalf("expected Selected 49 (clamped), got %d", sl.Selected)
	}

	// PageDown at last item — stays at last
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, 0), r, items)
	if sl.Selected != 49 {
		t.Fatalf("expected Selected 49 (still), got %d", sl.Selected)
	}
}

func TestSelectableListPageUp(t *testing.T) {
	sl := &SelectableList{Selected: 30, ScrollTop: 25}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}
	items := 50

	// PageUp moves by page size (rect.H = 10)
	res := sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, 0), r, items)
	if res.Result != EventConsumed {
		t.Fatal("PageUp should be consumed")
	}
	if sl.Selected != 20 {
		t.Fatalf("expected Selected 20, got %d", sl.Selected)
	}

	// PageUp again
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, 0), r, items)
	if sl.Selected != 10 {
		t.Fatalf("expected Selected 10, got %d", sl.Selected)
	}

	// PageUp near start — clamps to 0
	sl.Selected = 5
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, 0), r, items)
	if sl.Selected != 0 {
		t.Fatalf("expected Selected 0 (clamped), got %d", sl.Selected)
	}

	// PageUp at start — stays at 0
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, 0), r, items)
	if sl.Selected != 0 {
		t.Fatalf("expected Selected 0 (still), got %d", sl.Selected)
	}
}

func TestSelectableListPageDownScrollOffset(t *testing.T) {
	sl := &SelectableList{Selected: 0, ScrollTop: 0}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}
	items := 50

	// PageDown should update ScrollTop to keep selection visible
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, 0), r, items)
	if sl.Selected != 10 {
		t.Fatalf("expected Selected 10, got %d", sl.Selected)
	}
	// Selected=10 should be visible: ScrollTop should be at least 1
	if sl.ScrollTop > sl.Selected || sl.Selected >= sl.ScrollTop+r.H {
		t.Fatalf("Selected %d not visible with ScrollTop %d and H %d", sl.Selected, sl.ScrollTop, r.H)
	}
}

func TestSelectableListPageUpScrollOffset(t *testing.T) {
	sl := &SelectableList{Selected: 40, ScrollTop: 35}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}
	items := 50

	// PageUp should update ScrollTop to keep selection visible
	sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, 0), r, items)
	if sl.Selected != 30 {
		t.Fatalf("expected Selected 30, got %d", sl.Selected)
	}
	if sl.ScrollTop > sl.Selected || sl.Selected >= sl.ScrollTop+r.H {
		t.Fatalf("Selected %d not visible with ScrollTop %d and H %d", sl.Selected, sl.ScrollTop, r.H)
	}
}

func TestSelectableListPageDownEmptyList(t *testing.T) {
	sl := &SelectableList{Selected: 0}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}

	// PageDown with 0 items should not panic, Selected stays 0
	res := sl.HandleListEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, 0), r, 0)
	if res.Result != EventConsumed {
		t.Fatal("PageDown should be consumed even for empty list")
	}
	if sl.Selected != 0 {
		t.Fatalf("expected Selected 0 for empty list, got %d", sl.Selected)
	}
}

func TestSelectableListUnhandledKey(t *testing.T) {
	sl := &SelectableList{}
	r := Rect{X: 0, Y: 0, W: 20, H: 10}

	res := sl.HandleListEvent(tcell.NewEventKey(tcell.KeyLeft, "", 0), r, 5)
	if res.Result != EventIgnored {
		t.Fatal("Left key should be ignored by SelectableList")
	}

	res = sl.HandleListEvent(tcell.NewEventKey(tcell.KeyRune, "x", 0), r, 5)
	if res.Result != EventIgnored {
		t.Fatal("rune key should be ignored by SelectableList")
	}
}
