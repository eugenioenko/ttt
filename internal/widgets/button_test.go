package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

// surfaceRowText extracts the text from row y of the surface as a string.
func surfaceRowText(s *testSurface, y int) string {
	if y < 0 || y >= len(s.cells) {
		return ""
	}
	runes := make([]rune, len(s.cells[y]))
	for x, c := range s.cells[y] {
		if c.Ch == 0 {
			runes[x] = ' '
		} else {
			runes[x] = c.Ch
		}
	}
	return string(runes)
}

func TestButtonRenderLabel(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "OK"})
	s := renderWidget(btn, 0, 0, 20, 1)
	row := surfaceRowText(s, 0)
	// Default BoxModel has PaddingLeft=1, PaddingRight=1, so label starts at x=1
	if row[1] != 'O' || row[2] != 'K' {
		t.Errorf("expected label 'OK' at offset 1, got row: %q", row)
	}
}

func TestButtonAcceleratorUnderline(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "&Save"})
	s := renderWidget(btn, 0, 0, 20, 1)

	// Accelerator removes '&', so label is "Save". "S" is at accelIndex=0.
	// With PaddingLeft=1, the 'S' cell is at surface column 1.
	if btn.accelIndex != 0 {
		t.Fatalf("expected accelIndex=0, got %d", btn.accelIndex)
	}
	if btn.accelRune != 'S' {
		t.Fatalf("expected accelRune='S', got %c", btn.accelRune)
	}
	if btn.label != "Save" {
		t.Fatalf("expected label='Save', got %q", btn.label)
	}
	// The underline should be set on the cell at the accel position
	cell := s.cells[0][1] // PaddingLeft=1, so accelIndex=0 maps to surface x=1
	if !cell.Underline {
		t.Error("accelerator rune 'S' should be underlined")
	}
	if cell.Ch != 'S' {
		t.Errorf("expected 'S' at accel position, got %c", cell.Ch)
	}
	// Non-accel characters should NOT be underlined
	cell2 := s.cells[0][2]
	if cell2.Underline {
		t.Error("non-accelerator rune should not be underlined")
	}
}

func TestButtonAcceleratorMiddle(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "E&xit"})
	if btn.label != "Exit" {
		t.Fatalf("expected label='Exit', got %q", btn.label)
	}
	if btn.accelIndex != 1 {
		t.Fatalf("expected accelIndex=1, got %d", btn.accelIndex)
	}
	if btn.accelRune != 'x' {
		t.Fatalf("expected accelRune='x', got %c", btn.accelRune)
	}
}

func TestButtonClickFiresOnClick(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Click Me",
		OnClick: func() { clicked = true },
	})
	renderWidget(btn, 5, 3, 20, 1)

	// Mouse-down arms the button; an immediate release activates it.
	result := btn.HandleEvent(mousePress(6, 3))
	if result != EventCaptured {
		t.Errorf("mouse-down result = %v, want captured", result)
	}
	if clicked {
		t.Fatal("OnClick ran before mouse-up")
	}
	result = btn.HandleEvent(mouseRelease(6, 3))
	if result != EventConsumed {
		t.Errorf("mouse-up result = %v, want consumed", result)
	}
	if !clicked {
		t.Error("OnClick should run on mouse-up")
	}
}

func TestButtonDraggingInDoesNotActivate(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Button",
		OnClick: func() { clicked = true },
	})
	renderWidget(btn, 5, 3, 20, 1)

	btn.HandleEvent(mousePress(4, 3))
	btn.HandleEvent(mousePress(6, 3))
	btn.HandleEvent(mouseRelease(6, 3))
	if clicked {
		t.Fatal("dragging into button activated without a press originating inside it")
	}
}

func TestButtonCanClickAgainAfterCanceledPress(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Button",
		OnClick: func() { clicked = true },
	})
	renderWidget(btn, 5, 3, 20, 1)

	btn.HandleEvent(mousePress(6, 3))
	if !btn.CancelPointerCapture() {
		t.Fatal("button did not cancel the active press")
	}
	// The canceled press's release may be routed elsewhere.
	btn.HandleEvent(mousePress(6, 3))
	btn.HandleEvent(mouseRelease(6, 3))
	if !clicked {
		t.Fatal("button did not activate on the new press and release")
	}
}

func TestButtonClickOutsideBounds(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Button",
		OnClick: func() { clicked = true },
	})
	renderWidget(btn, 5, 3, 20, 1)

	// Click outside the button rect
	ev := mouseClick(0, 0)
	result := btn.HandleEvent(ev)
	if result == EventConsumed {
		t.Error("click outside button should not be consumed")
	}
	if clicked {
		t.Error("OnClick should not be called for click outside bounds")
	}
}

func TestButtonReleaseOutsideDoesNotActivate(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Button",
		OnClick: func() { clicked = true },
	})
	renderWidget(btn, 5, 3, 20, 1)

	btn.HandleEvent(mousePress(6, 3))
	btn.HandleEvent(mouseRelease(0, 0))
	if clicked {
		t.Error("mouse-up outside button should not activate it")
	}
}

func TestButtonEnterKeyFiresOnClick(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Submit",
		OnClick: func() { clicked = true },
	})
	btn.SetFocused(true)

	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := btn.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("enter key on focused button should be consumed")
	}
	if !clicked {
		t.Error("OnClick should be called on Enter when focused")
	}
}

func TestButtonSpaceKeyFiresOnClick(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Submit",
		OnClick: func() { clicked = true },
	})
	btn.SetFocused(true)

	ev := tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone)
	result := btn.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("space key on focused button should be consumed")
	}
	if !clicked {
		t.Error("OnClick should be called on Space when focused")
	}
}

func TestButtonEnterKeyIgnoredWhenNotFocused(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Submit",
		OnClick: func() { clicked = true },
	})
	// Not focused

	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := btn.HandleEvent(ev)
	// Enter should not trigger if not focused (no accel match either)
	if clicked {
		t.Error("OnClick should not be called on Enter when not focused")
	}
	if result == EventConsumed {
		t.Error("enter key on unfocused button should not be consumed")
	}
}

func TestButtonFocusStyling(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "Test"})

	// Render unfocused
	btn.SetFocused(false)
	s1 := renderWidget(btn, 0, 0, 20, 1)
	unfocusedStyle := s1.cells[0][1].Style // PaddingLeft=1, label starts at x=1
	if unfocusedStyle == term.StyleButtonFocused {
		t.Error("unfocused button should not use StyleButtonFocused")
	}

	// Render focused
	btn.SetFocused(true)
	s2 := renderWidget(btn, 0, 0, 20, 1)
	focusedStyle := s2.cells[0][1].Style
	if focusedStyle != term.StyleButtonFocused {
		t.Errorf("focused button should use StyleButtonFocused, got %v", focusedStyle)
	}
}

func TestButtonFocusable(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "X"})
	if !btn.Focusable() {
		t.Error("button should be focusable")
	}
	btn.SetFocused(true)
	if !btn.IsFocused() {
		t.Error("button should report focused after SetFocused(true)")
	}
	btn.SetFocused(false)
	if btn.IsFocused() {
		t.Error("button should not report focused after SetFocused(false)")
	}
}

func TestButtonHeightIncludesBoxOverhead(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{
		Label: "Test",
		Box: &BoxModel{
			MarginTop:     1,
			MarginBottom:  1,
			PaddingTop:    1,
			PaddingBottom: 1,
		},
	})
	// Height = 1 (label line) + margin top(1) + margin bottom(1) + padding top(1) + padding bottom(1) = 5
	expected := 1 + 1 + 1 + 1 + 1
	if btn.Height() != expected {
		t.Errorf("expected height=%d, got %d", expected, btn.Height())
	}
}

func TestButtonWidth(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "Save"})
	// Width = len("Save")=4 + PaddingLeft(1) + PaddingRight(1) = 6
	if btn.Width() != 6 {
		t.Errorf("expected width=6, got %d", btn.Width())
	}
}

func TestButtonOnCommand(t *testing.T) {
	var received string
	btn := NewButtonWidget(ButtonConfig{
		Label:     "Run",
		Command:   "editor.run",
		OnCommand: func(cmd string) { received = cmd },
	})
	btn.SetFocused(true)
	btn.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if received != "editor.run" {
		t.Errorf("expected command 'editor.run', got %q", received)
	}
}

func TestButtonAcceleratorKeyTrigger(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "&Save",
		OnClick: func() { clicked = true },
	})
	// Not focused, but accel key 's' should trigger
	ev := tcell.NewEventKey(tcell.KeyRune, "s", tcell.ModNone)
	result := btn.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("accelerator key should be consumed")
	}
	if !clicked {
		t.Error("OnClick should be called via accelerator key")
	}
}

func TestButtonAcceleratorKeyCaseInsensitive(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "&Save",
		OnClick: func() { clicked = true },
	})
	// Uppercase 'S' should also match
	ev := tcell.NewEventKey(tcell.KeyRune, "S", tcell.ModNone)
	result := btn.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("accelerator key (uppercase) should be consumed")
	}
	if !clicked {
		t.Error("OnClick should be called via uppercase accelerator key")
	}
}

// A fullwidth rune covers two columns, so a label drawn one column per rune
// drops every other character (#1065).
func TestButtonRendersFullwidthLabel(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "&ボタン"})

	s := renderWidget(btn, 0, 0, 20, 1)

	// The label starts after the button's left padding.
	if got := rowText(s, 0); !strings.Contains(got, "ボタン") {
		t.Errorf("row = %q, want it to hold the whole label", got)
	}
	// The label is six columns wide, plus the button's own padding.
	if want := 6 + btn.BoxOverheadW(); btn.Width() != want {
		t.Errorf("width = %d, want %d", btn.Width(), want)
	}
	// The accelerator is the first rune, which starts at column 1.
	if !s.cells[0][1].Underline {
		t.Error("the accelerator rune is not underlined")
	}
}

// The accelerator is a rune index, so it has to be counted in runes and not
// in columns (#1065).
func TestButtonUnderlinesFullwidthAccelerator(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "日本&語"})

	s := renderWidget(btn, 0, 0, 20, 1)

	if got := rowText(s, 0); !strings.Contains(got, "日本語") {
		t.Errorf("row = %q, want it to hold the whole label", got)
	}
	// The underline belongs on the third rune, which starts at column 5.
	if !s.cells[0][5].Underline {
		t.Error("the accelerator rune is not underlined")
	}
}
