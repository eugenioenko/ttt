package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestCheckboxRenderUnchecked(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Option A"})
	s := renderWidget(cb, 0, 0, 30, 1)

	// Unchecked checkbox renders: " [ ] Option A"
	// Cells: 0=' ', 1='[', 2=' ', 3=']', 4=' ', 5='O', ...
	if s.cells[0][1].Ch != '[' {
		t.Errorf("expected '[' at x=1, got %c", s.cells[0][1].Ch)
	}
	if s.cells[0][2].Ch != ' ' {
		t.Errorf("expected ' ' (unchecked) at x=2, got %c", s.cells[0][2].Ch)
	}
	if s.cells[0][3].Ch != ']' {
		t.Errorf("expected ']' at x=3, got %c", s.cells[0][3].Ch)
	}
	if s.cells[0][5].Ch != 'O' {
		t.Errorf("expected 'O' at x=5, got %c", s.cells[0][5].Ch)
	}
}

func TestCheckboxRenderChecked(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Option B", Checked: true})
	s := renderWidget(cb, 0, 0, 30, 1)

	// Checked checkbox renders: " [x] Option B"
	if s.cells[0][2].Ch != 'x' {
		t.Errorf("expected 'x' (checked) at x=2, got %c", s.cells[0][2].Ch)
	}
}

func TestCheckboxClickToggles(t *testing.T) {
	var changed bool
	var newState bool
	cb := NewCheckboxWidget(CheckboxConfig{
		Label: "Toggle",
		OnChange: func(checked bool) {
			changed = true
			newState = checked
		},
	})
	renderWidget(cb, 5, 3, 20, 1)

	// Initially unchecked
	if cb.Config.Checked {
		t.Fatal("checkbox should start unchecked")
	}

	// Click inside the checkbox rect
	ev := mouseClick(6, 3)
	result := cb.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("click inside checkbox should be consumed")
	}
	if !changed {
		t.Error("OnChange should have been called")
	}
	if !newState {
		t.Error("checkbox should be checked after toggle")
	}
	if !cb.Config.Checked {
		t.Error("Config.Checked should be true after toggle")
	}

	// Click again to uncheck
	changed = false
	ev2 := mouseClick(7, 3)
	cb.HandleEvent(ev2)
	if !changed {
		t.Error("OnChange should have been called on second toggle")
	}
	if newState {
		t.Error("checkbox should be unchecked after second toggle")
	}
}

func TestCheckboxClickOutside(t *testing.T) {
	clicked := false
	cb := NewCheckboxWidget(CheckboxConfig{
		Label:    "Test",
		OnChange: func(bool) { clicked = true },
	})
	renderWidget(cb, 5, 3, 20, 1)

	ev := mouseClick(0, 0)
	result := cb.HandleEvent(ev)
	if result == EventConsumed {
		t.Error("click outside checkbox should not be consumed")
	}
	if clicked {
		t.Error("OnChange should not be called for click outside bounds")
	}
}

func TestCheckboxEnterKeyToggles(t *testing.T) {
	toggled := false
	cb := NewCheckboxWidget(CheckboxConfig{
		Label:    "Enter Test",
		OnChange: func(bool) { toggled = true },
	})
	cb.SetFocused(true)

	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := cb.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("enter key on focused checkbox should be consumed")
	}
	if !toggled {
		t.Error("OnChange should be called on Enter when focused")
	}
	if !cb.Config.Checked {
		t.Error("checkbox should be checked after Enter toggle")
	}
}

func TestCheckboxSpaceKeyToggles(t *testing.T) {
	toggled := false
	cb := NewCheckboxWidget(CheckboxConfig{
		Label:    "Space Test",
		OnChange: func(bool) { toggled = true },
	})
	cb.SetFocused(true)

	ev := tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone)
	result := cb.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("space key on focused checkbox should be consumed")
	}
	if !toggled {
		t.Error("OnChange should be called on Space when focused")
	}
}

func TestCheckboxKeyIgnoredWhenNotFocused(t *testing.T) {
	toggled := false
	cb := NewCheckboxWidget(CheckboxConfig{
		Label:    "Unfocused",
		OnChange: func(bool) { toggled = true },
	})
	// Not focused

	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := cb.HandleEvent(ev)
	if result == EventConsumed {
		t.Error("enter key on unfocused checkbox should not be consumed")
	}
	if toggled {
		t.Error("OnChange should not be called when checkbox is not focused")
	}
}

func TestCheckboxFocusStyling(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Styled"})

	// Focus is carried by the bracket colour, with no background change, so the
	// mark and label must render identically either way.
	cb.SetFocused(false)
	s1 := renderWidget(cb, 0, 0, 30, 1)
	if got := s1.cells[0][1].Style; got != term.StyleBorder {
		t.Errorf("unfocused bracket = %v, want StyleBorder", got)
	}

	cb.SetFocused(true)
	s2 := renderWidget(cb, 0, 0, 30, 1)
	if got := s2.cells[0][1].Style; got != term.StyleBorderActive {
		t.Errorf("focused bracket = %v, want StyleBorderActive", got)
	}
	if s2.cells[0][3].Style != term.StyleBorderActive {
		t.Error("closing bracket should track focus too")
	}

	for _, x := range []int{2, 5, 6} {
		if s1.cells[0][x].Style != s2.cells[0][x].Style {
			t.Errorf("cell %d changed style on focus; only brackets should", x)
		}
	}
}

func TestCheckboxFocusable(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "F"})
	if !cb.Focusable() {
		t.Error("checkbox should be focusable")
	}
	cb.SetFocused(true)
	if !cb.IsFocused() {
		t.Error("checkbox should report focused after SetFocused(true)")
	}
	cb.SetFocused(false)
	if cb.IsFocused() {
		t.Error("checkbox should not report focused after SetFocused(false)")
	}
}

func TestCheckboxHeight(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Test"})
	if cb.Height() != 1 {
		t.Errorf("expected height=1, got %d", cb.Height())
	}
}

func TestCheckboxHeightWithBoxModel(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Test"})
	cb.SetBoxModel(BoxModel{
		MarginTop:    1,
		MarginBottom: 1,
	})
	// Height = 1 + margin top(1) + margin bottom(1) = 3
	if cb.Height() != 3 {
		t.Errorf("expected height=3, got %d", cb.Height())
	}
}

func TestCheckboxNoOnChangeCallback(t *testing.T) {
	// Ensure toggling works even without OnChange callback (no panic)
	cb := NewCheckboxWidget(CheckboxConfig{Label: "No callback"})
	cb.SetFocused(true)
	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	cb.HandleEvent(ev)
	if !cb.Config.Checked {
		t.Error("checkbox should toggle even without OnChange callback")
	}
}

func TestCheckboxLabelRendered(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Hello"})
	s := renderWidget(cb, 0, 0, 30, 1)

	// Label starts at x=5: " [x] Hello"
	expected := "Hello"
	for i, ch := range expected {
		if s.cells[0][5+i].Ch != ch {
			t.Errorf("expected %c at x=%d, got %c", ch, 5+i, s.cells[0][5+i].Ch)
		}
	}
}

// A fullwidth rune covers two columns, so a label drawn one column per rune
// drops every other character (#1066).
func TestCheckboxRendersFullwidthLabel(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "チェック", Checked: true})

	s := renderWidget(cb, 0, 0, 20, 1)

	if got := rowText(s, 0); got != " [x] チェック" {
		t.Errorf("row = %q, want %q", got, " [x] チェック")
	}
}

// A fullwidth rune that would cross the last column becomes a space: the
// terminal paints it across two columns regardless of clipping (#1066).
func TestCheckboxFullwidthLabelStaysInsideTheEdge(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "チェック", Checked: true})

	// The label starts at column 5, so only one fullwidth rune fits in eight
	// columns.
	s := renderWidget(cb, 0, 0, 8, 1)

	if got := s.cells[0][5].Ch; got != 'チ' {
		t.Errorf("cell 5 = %q, want the first fullwidth rune", got)
	}
	if got := s.cells[0][7].Ch; got != ' ' {
		t.Errorf("cell 7 = %q, want a space where the next rune would cross the edge", got)
	}
}
