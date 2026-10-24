package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

// ---------------------------------------------------------------------------
// ButtonWidget tests
// ---------------------------------------------------------------------------

func TestButtonAcceleratorParsing(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "&Save",
		OnClick: func() { clicked = true },
	})

	if btn.label != "Save" {
		t.Fatalf("expected label 'Save', got %q", btn.label)
	}
	if btn.accelIndex != 0 {
		t.Fatalf("expected accelIndex 0, got %d", btn.accelIndex)
	}
	if btn.accelRune != 'S' {
		t.Fatalf("expected accelRune 'S', got %q", btn.accelRune)
	}

	// Lowercase 's' should trigger
	renderWidget(btn, 0, 0, 10, 3)
	ev := tcell.NewEventKey(tcell.KeyRune, "s", tcell.ModNone)
	result := btn.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("lowercase accel rune should trigger button")
	}
	if !clicked {
		t.Error("OnClick should have been called via accel")
	}

	// Uppercase 'S' should also trigger
	clicked = false
	ev = tcell.NewEventKey(tcell.KeyRune, "S", tcell.ModNone)
	result = btn.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("uppercase accel rune should trigger button")
	}
	if !clicked {
		t.Error("OnClick should have been called via uppercase accel")
	}
}

func TestButtonNoAccelerator(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "Cancel"})

	if btn.label != "Cancel" {
		t.Fatalf("expected label 'Cancel', got %q", btn.label)
	}
	if btn.accelIndex != -1 {
		t.Fatalf("expected accelIndex -1, got %d", btn.accelIndex)
	}
	if btn.accelRune != 0 {
		t.Fatalf("expected no accelRune, got %q", btn.accelRune)
	}

	// Random rune should not trigger
	renderWidget(btn, 0, 0, 10, 3)
	ev := tcell.NewEventKey(tcell.KeyRune, "c", tcell.ModNone)
	result := btn.HandleEvent(ev)
	if result != EventIgnored {
		t.Error("button without accelerator should ignore rune keys")
	}
}

func TestButtonDimensions(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "&Save"})

	// Label is "Save" (4 runes), default padding left=1, right=1 => width = 4+2 = 6
	if got := btn.Width(); got != 6 {
		t.Errorf("expected Width()=6, got %d", got)
	}
	// Height is always 1 + box overhead (default has no vertical padding/borders)
	if got := btn.Height(); got != 1 {
		t.Errorf("expected Height()=1, got %d", got)
	}
}

func TestButtonClickInside(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "OK",
		OnClick: func() { clicked = true },
	})

	renderWidget(btn, 5, 5, 10, 1)

	// Mouse-down arms the button; release activates it.
	result := btn.HandleEvent(mousePress(6, 5))
	if result != EventCaptured {
		t.Errorf("mouse-down result = %v, want captured", result)
	}
	if clicked {
		t.Fatal("OnClick ran before mouse-up")
	}
	btn.HandleEvent(mouseRelease(6, 5))
	if !clicked {
		t.Error("OnClick should fire on mouse-up")
	}
}

func TestButtonClickOutside(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "OK",
		OnClick: func() { clicked = true },
	})

	renderWidget(btn, 5, 5, 10, 1)

	// Click outside the rect
	ev := mouseClick(0, 0)
	result := btn.HandleEvent(ev)
	if result != EventIgnored {
		t.Error("click outside button should return EventIgnored")
	}
	if clicked {
		t.Error("OnClick should NOT fire on click outside")
	}
}

func TestButtonKeyboardTriggersFocused(t *testing.T) {
	clickCount := 0
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Submit",
		OnClick: func() { clickCount++ },
	})
	btn.SetFocused(true)
	renderWidget(btn, 0, 0, 10, 1)

	// Enter triggers
	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := btn.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("Enter should trigger focused button")
	}
	if clickCount != 1 {
		t.Errorf("expected 1 click, got %d", clickCount)
	}

	// Space triggers
	ev = tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone)
	result = btn.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("Space should trigger focused button")
	}
	if clickCount != 2 {
		t.Errorf("expected 2 clicks, got %d", clickCount)
	}

	// Non-matching key should be ignored (no accelerator on this button)
	ev = tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone)
	result = btn.HandleEvent(ev)
	if result != EventIgnored {
		t.Error("non-matching key should be ignored by focused button without accel")
	}
}

func TestButtonKeyboardIgnoredUnfocused(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Submit",
		OnClick: func() { clicked = true },
	})
	btn.SetFocused(false)
	renderWidget(btn, 0, 0, 10, 1)

	// Enter should NOT trigger unfocused button
	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := btn.HandleEvent(ev)
	if result != EventIgnored {
		t.Error("Enter should not trigger unfocused button")
	}
	if clicked {
		t.Error("OnClick should NOT fire when unfocused and Enter pressed")
	}
}

func TestButtonAccelTriggersUnfocused(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "&Delete",
		OnClick: func() { clicked = true },
	})
	btn.SetFocused(false)
	renderWidget(btn, 0, 0, 10, 1)

	// Accel rune triggers even when unfocused
	ev := tcell.NewEventKey(tcell.KeyRune, "d", tcell.ModNone)
	result := btn.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("accel key should trigger even when unfocused")
	}
	if !clicked {
		t.Error("OnClick should fire via accel key when unfocused")
	}
}

// ---------------------------------------------------------------------------
// CheckboxWidget tests
// ---------------------------------------------------------------------------

func TestCheckboxToggle(t *testing.T) {
	var lastState bool
	changeCalled := false
	cb := NewCheckboxWidget(CheckboxConfig{
		Label:   "Enable feature",
		Checked: false,
		OnChange: func(checked bool) {
			changeCalled = true
			lastState = checked
		},
	})

	cb.toggle()
	if !cb.Config.Checked {
		t.Error("expected Checked=true after toggle")
	}
	if !changeCalled {
		t.Error("OnChange should be called on toggle")
	}
	if !lastState {
		t.Error("OnChange should receive true after first toggle")
	}

	changeCalled = false
	cb.toggle()
	if cb.Config.Checked {
		t.Error("expected Checked=false after second toggle")
	}
	if !changeCalled {
		t.Error("OnChange should be called on second toggle")
	}
	if lastState {
		t.Error("OnChange should receive false after second toggle")
	}
}

func TestCheckboxKeyEventsFocused(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Option"})
	cb.SetFocused(true)
	renderWidget(cb, 0, 0, 20, 1)

	// Enter toggles
	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := cb.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("Enter should be consumed when focused")
	}
	if !cb.Config.Checked {
		t.Error("Checkbox should be checked after Enter")
	}

	// Space toggles back
	ev = tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone)
	result = cb.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("Space should be consumed when focused")
	}
	if cb.Config.Checked {
		t.Error("Checkbox should be unchecked after Space toggle")
	}
}

func TestCheckboxKeyEventsUnfocused(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Option"})
	cb.SetFocused(false)
	renderWidget(cb, 0, 0, 20, 1)

	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := cb.HandleEvent(ev)
	if result != EventIgnored {
		t.Error("Enter should be ignored when unfocused")
	}
	if cb.Config.Checked {
		t.Error("Checkbox should remain unchecked when unfocused Enter pressed")
	}
}

func TestCheckboxMouseClickInside(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Toggle me"})
	renderWidget(cb, 5, 5, 20, 1)

	ev := mouseClick(6, 5)
	result := cb.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("click inside checkbox should be consumed")
	}
	if !cb.Config.Checked {
		t.Error("checkbox should be checked after click inside")
	}
}

func TestCheckboxMouseClickOutside(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Toggle me"})
	renderWidget(cb, 5, 5, 20, 1)

	ev := mouseClick(0, 0)
	result := cb.HandleEvent(ev)
	if result != EventIgnored {
		t.Error("click outside checkbox should return EventIgnored")
	}
	if cb.Config.Checked {
		t.Error("checkbox should remain unchecked after click outside")
	}
}

func TestCheckboxRenderCheckedVsUnchecked(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Opt"})

	// Unchecked: cell at position 2 (inside brackets) should be ' '
	s := renderWidget(cb, 0, 0, 20, 1)
	if s.cells[0][2].Ch != ' ' {
		t.Errorf("unchecked mark cell: expected ' ', got %q", s.cells[0][2].Ch)
	}

	// Toggle to checked
	cb.Config.Checked = true
	s = renderWidget(cb, 0, 0, 20, 1)
	if s.cells[0][2].Ch != 'x' {
		t.Errorf("checked mark cell: expected 'x', got %q", s.cells[0][2].Ch)
	}
}

// ---------------------------------------------------------------------------
// DialogWidget tests
// ---------------------------------------------------------------------------

func TestDialogEscapeDismissal(t *testing.T) {
	dismissed := false
	d := NewDialogWidget(40)
	d.OnDismiss = func() { dismissed = true }
	d.Build()

	ev := tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone)
	result := d.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("Escape should return EventConsumed")
	}
	if !dismissed {
		t.Error("Escape should call OnDismiss")
	}
}

func TestDialogModalEventSwallowing(t *testing.T) {
	d := NewDialogWidget(40)
	d.Build()

	// Regular key events should be consumed (dialog is modal)
	keys := []tcell.Key{tcell.KeyRune, tcell.KeyEnter, tcell.KeyTab, tcell.KeyUp}
	strs := []string{"a", "", "", ""}
	for i, key := range keys {
		ev := tcell.NewEventKey(key, strs[i], tcell.ModNone)
		result := d.HandleEvent(ev)
		if result != EventConsumed {
			t.Errorf("dialog should consume key event %v (modal), got EventIgnored", key)
		}
	}
}

func TestDialogButtonCallback(t *testing.T) {
	okClicked := false
	cancelClicked := false

	d := NewDialogWidget(40)
	d.Title = "Confirm"
	d.Buttons = []DialogButton{
		{Label: "&OK", Handler: func() { okClicked = true }},
		{Label: "&Cancel", Handler: func() { cancelClicked = true }},
	}
	d.Build()

	// Render the dialog to set rects on footer buttons
	renderWidget(d, 0, 0, 60, 20)

	// The footer is an HStack of buttons. Use the accelerator key to trigger.
	ev := tcell.NewEventKey(tcell.KeyRune, "o", tcell.ModNone)
	d.HandleEvent(ev)

	if !okClicked {
		t.Error("OK button handler should have fired via accelerator")
	}
	if cancelClicked {
		t.Error("Cancel button handler should NOT have fired")
	}
}

func TestDialogDefaultWidth(t *testing.T) {
	d := NewDialogWidget(0)
	if d.BoxWidth != 40 {
		t.Errorf("expected default BoxWidth=40 when 0 passed, got %d", d.BoxWidth)
	}
}
