package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func makeSelectItems(labels ...string) []SelectItem {
	items := make([]SelectItem, len(labels))
	for i, l := range labels {
		items[i] = SelectItem{ID: l, Label: l}
	}
	return items
}

func sendRune(w Widget, r rune) EventResult {
	ev := tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone)
	return w.HandleEvent(ev)
}

func sendKey(w Widget, key tcell.Key) EventResult {
	ev := tcell.NewEventKey(key, "", tcell.ModNone)
	return w.HandleEvent(ev)
}

func TestSelectFilteringShowsAll(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("Apple", "Banana", "Cherry"),
	})
	sw.SetFocused(true)

	// With no filter text, all items should be visible
	if len(sw.filtered) != 3 {
		t.Fatalf("expected 3 filtered items, got %d", len(sw.filtered))
	}
}

func TestSelectFilteringCaseInsensitive(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("Apple", "Banana", "Apricot", "Cherry"),
	})
	sw.SetFocused(true)

	// Type "ap" to filter - should match Apple and Apricot (case insensitive)
	sendRune(sw, 'a')
	sendRune(sw, 'p')

	if len(sw.filtered) != 2 {
		t.Fatalf("expected 2 filtered items for 'ap', got %d", len(sw.filtered))
	}

	// Verify the correct items are shown
	for _, idx := range sw.filtered {
		label := sw.Config.Items[idx].Label
		if label != "Apple" && label != "Apricot" {
			t.Errorf("unexpected filtered item: %s", label)
		}
	}
}

func TestSelectFilteringResetsSelection(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("Alpha", "Beta", "Gamma"),
	})
	sw.SetFocused(true)

	// Move selection down
	sendKey(sw, tcell.KeyDown)
	if sw.selected != 1 {
		t.Fatalf("expected selected=1 after down, got %d", sw.selected)
	}

	// Type to filter - selection should reset to 0
	sendRune(sw, 'a')
	if sw.selected != 0 {
		t.Errorf("typing should reset selected to 0, got %d", sw.selected)
	}
}

func TestSelectFilteringEmptyShowsAll(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("Apple", "Banana", "Cherry"),
	})
	sw.SetFocused(true)

	// Type and then delete to clear filter
	sendRune(sw, 'x')
	if len(sw.filtered) != 0 {
		t.Fatalf("expected 0 filtered items for 'x', got %d", len(sw.filtered))
	}

	// Backspace to clear
	sendKey(sw, tcell.KeyBackspace2)
	if len(sw.filtered) != 3 {
		t.Errorf("expected 3 filtered items after clearing, got %d", len(sw.filtered))
	}
}

func TestSelectKeyboardNavigationDown(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("Alpha", "Beta", "Gamma"),
	})

	if sw.selected != 0 {
		t.Fatalf("initial selected should be 0, got %d", sw.selected)
	}

	sendKey(sw, tcell.KeyDown)
	if sw.selected != 1 {
		t.Errorf("expected selected=1 after Down, got %d", sw.selected)
	}

	sendKey(sw, tcell.KeyDown)
	if sw.selected != 2 {
		t.Errorf("expected selected=2 after second Down, got %d", sw.selected)
	}

	// Down at the end should not go past the last item
	sendKey(sw, tcell.KeyDown)
	if sw.selected != 2 {
		t.Errorf("Down at last item should clamp, got selected=%d", sw.selected)
	}
}

func TestSelectKeyboardNavigationUp(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("Alpha", "Beta", "Gamma"),
	})

	// Up at start should be no-op
	sendKey(sw, tcell.KeyUp)
	if sw.selected != 0 {
		t.Errorf("Up at first item should clamp, got selected=%d", sw.selected)
	}

	// Move down then back up
	sendKey(sw, tcell.KeyDown)
	sendKey(sw, tcell.KeyDown)
	if sw.selected != 2 {
		t.Fatalf("expected selected=2, got %d", sw.selected)
	}

	sendKey(sw, tcell.KeyUp)
	if sw.selected != 1 {
		t.Errorf("expected selected=1 after Up, got %d", sw.selected)
	}
}

func TestSelectKeyboardNavigationConsumed(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("Alpha", "Beta"),
	})

	if got := sendKey(sw, tcell.KeyDown); got != EventConsumed {
		t.Error("Down key should return EventConsumed")
	}
	if got := sendKey(sw, tcell.KeyUp); got != EventConsumed {
		t.Error("Up key should return EventConsumed")
	}
}

func TestSelectEnterFiresOnSelect(t *testing.T) {
	var selectedID string
	sw := NewSelectWidget(SelectConfig{
		Items:    makeSelectItems("Alpha", "Beta", "Gamma"),
		OnSelect: func(id string) { selectedID = id },
	})

	// Select the second item and press Enter
	sendKey(sw, tcell.KeyDown)
	sendKey(sw, tcell.KeyEnter)

	if selectedID != "Beta" {
		t.Errorf("expected OnSelect called with 'Beta', got '%s'", selectedID)
	}
}

func TestSelectEnterWithFilter(t *testing.T) {
	var selectedID string
	sw := NewSelectWidget(SelectConfig{
		Items:    makeSelectItems("Apple", "Banana", "Apricot"),
		OnSelect: func(id string) { selectedID = id },
	})
	sw.SetFocused(true)

	// Type "ban" to filter down to Banana only
	sendRune(sw, 'b')
	sendRune(sw, 'a')
	sendRune(sw, 'n')

	if len(sw.filtered) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(sw.filtered))
	}

	sendKey(sw, tcell.KeyEnter)
	if selectedID != "Banana" {
		t.Errorf("expected OnSelect with 'Banana', got '%s'", selectedID)
	}
}

func TestSelectEnterConsumed(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("Alpha"),
	})

	if got := sendKey(sw, tcell.KeyEnter); got != EventConsumed {
		t.Error("Enter should return EventConsumed")
	}
}

func TestSelectEscapeFiresOnDismiss(t *testing.T) {
	dismissed := false
	sw := NewSelectWidget(SelectConfig{
		Items:     makeSelectItems("Alpha", "Beta"),
		OnDismiss: func() { dismissed = true },
	})

	result := sendKey(sw, tcell.KeyEscape)
	if result != EventConsumed {
		t.Error("Escape should return EventConsumed")
	}
	if !dismissed {
		t.Error("OnDismiss should be called on Escape")
	}
}

func TestSelectEscapeWithoutCallback(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("Alpha"),
		// no OnDismiss
	})

	// Should not panic
	result := sendKey(sw, tcell.KeyEscape)
	if result != EventConsumed {
		t.Error("Escape should return EventConsumed even without OnDismiss callback")
	}
}

func TestSelectHeightInline(t *testing.T) {
	// Non-collapsible with 5 items: height = items + 1 (input) = 6
	sw := NewSelectWidget(SelectConfig{
		Items: makeSelectItems("A", "B", "C", "D", "E"),
	})

	if got := sw.Height(); got != 6 {
		t.Errorf("expected Height=6 for 5 items inline, got %d", got)
	}
}

func TestSelectHeightInlineWithDivider(t *testing.T) {
	// With divider: height = items + 1 (input) + 2 (dividers) = items + 3
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("A", "B", "C", "D", "E"),
		ShowDivider: true,
	})

	if got := sw.Height(); got != 8 {
		t.Errorf("expected Height=8 for 5 items with divider, got %d", got)
	}
}

func TestSelectHeightCappedAt11(t *testing.T) {
	// Many items: height should be capped at 11
	items := makeSelectItems(
		"A", "B", "C", "D", "E", "F", "G", "H", "I", "J",
		"K", "L", "M", "N", "O",
	)
	sw := NewSelectWidget(SelectConfig{
		Items: items,
	})

	if got := sw.Height(); got != 11 {
		t.Errorf("expected Height=11 (capped), got %d", got)
	}
}

func TestSelectHeightCollapsible(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("A", "B", "C", "D", "E"),
		Collapsible: true,
	})

	if got := sw.Height(); got != 1 {
		t.Errorf("expected Height=1 for collapsible, got %d", got)
	}
}

func TestSelectOnChangeOnNavigation(t *testing.T) {
	var changedID string
	sw := NewSelectWidget(SelectConfig{
		Items:    makeSelectItems("Alpha", "Beta", "Gamma"),
		OnChange: func(id string) { changedID = id },
	})

	sendKey(sw, tcell.KeyDown)
	if changedID != "Beta" {
		t.Errorf("expected OnChange with 'Beta', got '%s'", changedID)
	}

	sendKey(sw, tcell.KeyDown)
	if changedID != "Gamma" {
		t.Errorf("expected OnChange with 'Gamma', got '%s'", changedID)
	}
}

func TestSelectOnChangeOnFilter(t *testing.T) {
	var changedID string
	sw := NewSelectWidget(SelectConfig{
		Items:    makeSelectItems("Apple", "Banana", "Cherry"),
		OnChange: func(id string) { changedID = id },
	})
	sw.SetFocused(true)

	// Typing "b" should filter to Banana and fire OnChange with first match
	sendRune(sw, 'b')
	if changedID != "Banana" {
		t.Errorf("expected OnChange with 'Banana' after filter, got '%s'", changedID)
	}
}

func TestSelectPopupScrollbarOffWidgetCaptureLifecycle(t *testing.T) {
	labels := make([]string, 20)
	for i := range labels {
		labels[i] = string(rune('a' + i))
	}
	sw := NewSelectWidget(SelectConfig{Items: makeSelectItems(labels...), Collapsible: true})
	sw.SetRect(Rect{X: 10, Y: 5, W: 10, H: 1})
	sw.setOpen(true)
	sw.Render(newVirtualSurface(10, 1))
	popup := sw.PopupRect()
	sw.RenderPopup(newVirtualSurface(popup.W, popup.H))
	invalidations := 0
	sw.SetPointerCaptureInvalidated(func() { invalidations++ })

	barX := popup.X + popup.W - 2
	barY := popup.Y + 1
	if got := sw.HandleEvent(tcell.NewEventMouse(barX, barY, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("popup scrollbar press result = %v, want captured", got)
	}
	if got := sw.HandleEvent(tcell.NewEventMouse(80, 80, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("off-popup drag result = %v, want captured", got)
	}
	if got := sw.HandleEvent(tcell.NewEventMouse(80, 80, tcell.ButtonNone, 0)); got != EventConsumed {
		t.Fatalf("off-popup release result = %v, want consumed", got)
	}
	if sw.OwnsPointerCapture() || invalidations != 0 {
		t.Fatalf("normal release retained or invalidated capture: invalidations=%d", invalidations)
	}

	sw.HandleEvent(tcell.NewEventMouse(barX, barY, tcell.Button1, 0))
	sw.ClosePopup()
	if sw.OwnsPointerCapture() || invalidations != 1 {
		t.Fatalf("closing popup did not invalidate capture: owns=%v invalidations=%d", sw.OwnsPointerCapture(), invalidations)
	}
}
