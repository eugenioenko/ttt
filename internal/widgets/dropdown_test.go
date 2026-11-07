package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestDropdownRenderLabel(t *testing.T) {
	dd := NewDropdownWidget(DropdownConfig{
		Label: "Actions",
		Entries: []MenuEntry{
			{Label: "Copy", Command: "copy"},
		},
	})
	s := renderWidget(dd, 0, 0, 15, 1)

	// The label "Actions" should appear on the button surface
	text := extractText(s, 0, 0, 7)
	// Button has PaddingLeft=1 by default, so label starts at x=1
	textPadded := extractText(s, 1, 0, 7)
	if text != "Actions" && textPadded != "Actions" {
		t.Errorf("dropdown should render label 'Actions', got %q / padded %q", text, textPadded)
	}
}

func TestDropdownDefaultLabel(t *testing.T) {
	dd := NewDropdownWidget(DropdownConfig{
		Entries: []MenuEntry{
			{Label: "Copy", Command: "copy"},
		},
	})
	// Default label is "⋮"
	s := renderWidget(dd, 0, 0, 10, 1)

	// Check with padding offset
	found := false
	for x := 0; x < 5; x++ {
		if s.cells[0][x].Ch == '⋮' {
			found = true
			break
		}
	}
	if !found {
		t.Error("dropdown with empty label should default to '⋮'")
	}
}

func TestDropdownClickFiresOnMenu(t *testing.T) {
	var receivedEntries []MenuEntry
	var receivedX, receivedY int
	called := false

	entries := []MenuEntry{
		{Label: "Copy", Command: "copy"},
		{Label: "Paste", Command: "paste"},
	}

	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Menu",
		Entries: entries,
		OnMenu: func(e []MenuEntry, sx, sy int) {
			called = true
			receivedEntries = e
			receivedX = sx
			receivedY = sy
		},
	})
	renderWidget(dd, 5, 10, 10, 1)

	// Click inside the dropdown rect
	click := mouseClick(6, 10)
	result := dd.HandleEvent(click)

	if !called {
		t.Fatal("clicking dropdown should fire OnMenu")
	}
	if result != EventConsumed {
		t.Error("click should be consumed")
	}
	if len(receivedEntries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(receivedEntries))
	}
	if receivedEntries[0].Command != "copy" {
		t.Errorf("first entry command should be 'copy', got %q", receivedEntries[0].Command)
	}

	// Screen coordinates: X = rect.X = 5, Y = rect.Y + rect.H = 10 + 1 = 11
	if receivedX != 5 {
		t.Errorf("screenX should be 5, got %d", receivedX)
	}
	if receivedY != 11 {
		t.Errorf("screenY should be 11 (below dropdown), got %d", receivedY)
	}
}

func TestDropdownClickOutsideBounds(t *testing.T) {
	called := false
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Menu",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
		OnMenu: func(e []MenuEntry, sx, sy int) {
			called = true
		},
	})
	renderWidget(dd, 5, 10, 10, 1)

	// Click outside
	click := mouseClick(0, 0)
	result := dd.HandleEvent(click)

	if called {
		t.Error("click outside should not fire OnMenu")
	}
	if result == EventConsumed {
		t.Error("click outside should not be consumed")
	}
}

func TestDropdownClickRightEdge(t *testing.T) {
	called := false
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Go",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
		OnMenu: func(e []MenuEntry, sx, sy int) {
			called = true
		},
	})
	renderWidget(dd, 5, 10, 10, 1)

	// Click at x=14 (just inside right edge: X=5, W=10, so max X is 14)
	click := mouseClick(14, 10)
	result := dd.HandleEvent(click)

	if !called {
		t.Error("click at right edge should fire OnMenu")
	}
	if result != EventConsumed {
		t.Error("click at right edge should be consumed")
	}
}

func TestDropdownClickJustOutsideRight(t *testing.T) {
	called := false
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Go",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
		OnMenu: func(e []MenuEntry, sx, sy int) {
			called = true
		},
	})
	renderWidget(dd, 5, 10, 10, 1)

	// Click at x=15 (just outside: X=5, W=10, so x=15 is out)
	click := mouseClick(15, 10)
	dd.HandleEvent(click)

	if called {
		t.Error("click just outside right edge should not fire OnMenu")
	}
}

func TestDropdownNoEntriesNoCallback(t *testing.T) {
	called := false
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Empty",
		Entries: []MenuEntry{},
		OnMenu: func(e []MenuEntry, sx, sy int) {
			called = true
		},
	})
	renderWidget(dd, 0, 0, 10, 1)

	click := mouseClick(1, 0)
	result := dd.HandleEvent(click)

	if called {
		t.Error("dropdown with empty entries should not fire OnMenu")
	}
	if result == EventConsumed {
		t.Error("dropdown with empty entries should not consume click")
	}
}

func TestDropdownNoOnMenuCallback(t *testing.T) {
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "NoHandler",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
		OnMenu:  nil,
	})
	renderWidget(dd, 0, 0, 15, 1)

	// Should not panic
	click := mouseClick(1, 0)
	result := dd.HandleEvent(click)

	if result == EventConsumed {
		t.Error("dropdown with nil OnMenu should not consume click")
	}
}

func TestDropdownHeightAndWidth(t *testing.T) {
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Test",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
	})

	// Height and Width delegate to the inner button
	h := dd.Height()
	w := dd.Width()

	if h < 1 {
		t.Errorf("dropdown height should be at least 1, got %d", h)
	}
	if w < 1 {
		t.Errorf("dropdown width should be at least 1, got %d", w)
	}
}

func TestDropdownNonMouseEventIgnored(t *testing.T) {
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Test",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
	})
	renderWidget(dd, 0, 0, 10, 1)

	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := dd.HandleEvent(ev)

	if result == EventConsumed {
		t.Error("non-mouse events should be ignored")
	}
}

func TestDropdownBoxModel(t *testing.T) {
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "B",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
		Box: &BoxModel{
			PaddingLeft:  2,
			PaddingRight: 2,
		},
	})

	// The box model should propagate to the inner button
	w := dd.Width()
	// "B" is 1 char + padding 2+2 = 5
	if w != 5 {
		t.Errorf("dropdown width with box padding should be 5, got %d", w)
	}
}

func TestDropdownRightClickIgnored(t *testing.T) {
	called := false
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Menu",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
		OnMenu: func(e []MenuEntry, sx, sy int) {
			called = true
		},
	})
	renderWidget(dd, 0, 0, 10, 1)

	// Right-click (Button2) should not trigger
	rightClick := tcell.NewEventMouse(1, 0, tcell.Button2, tcell.ModNone)
	result := dd.HandleEvent(rightClick)

	if called {
		t.Error("right-click should not fire OnMenu")
	}
	if result == EventConsumed {
		t.Error("right-click should not be consumed")
	}
}

func TestDropdownIsFocusable(t *testing.T) {
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Menu",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
	})

	if !dd.Focusable() {
		t.Fatal("dropdown should be focusable")
	}
	if dd.IsFocused() {
		t.Error("dropdown should not start focused")
	}
	dd.SetFocused(true)
	if !dd.IsFocused() {
		t.Error("dropdown should report focus after SetFocused(true)")
	}
	dd.SetFocused(false)
	if dd.IsFocused() {
		t.Error("dropdown should drop focus after SetFocused(false)")
	}
}

func TestDropdownOpensMenuFromKeyboardWhenFocused(t *testing.T) {
	called := 0
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Menu",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
		OnMenu: func(e []MenuEntry, sx, sy int) {
			called++
		},
	})
	renderWidget(dd, 5, 10, 10, 1)
	dd.SetFocused(true)

	enter := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	if result := dd.HandleEvent(enter); result != EventConsumed {
		t.Error("Enter on a focused dropdown should be consumed")
	}
	space := tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone)
	if result := dd.HandleEvent(space); result != EventConsumed {
		t.Error("Space on a focused dropdown should be consumed")
	}

	if called != 2 {
		t.Errorf("Enter and Space should each open the menu, got %d calls", called)
	}
}

func TestDropdownIgnoresKeyboardWhenNotFocused(t *testing.T) {
	called := false
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Menu",
		Entries: []MenuEntry{{Label: "A", Command: "a"}},
		OnMenu: func(e []MenuEntry, sx, sy int) {
			called = true
		},
	})
	renderWidget(dd, 0, 0, 10, 1)

	enter := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	if result := dd.HandleEvent(enter); result == EventConsumed {
		t.Error("Enter on an unfocused dropdown should be ignored")
	}
	if called {
		t.Error("unfocused dropdown should not fire OnMenu")
	}
}

func TestDropdownJoinsTheFocusRing(t *testing.T) {
	// A panel with an input, a button and a dropdown: Tab must reach the
	// dropdown, otherwise its menu is reachable by mouse only (#1050).
	inp := NewInputWidget(InputConfig{})
	btn := NewButtonWidget(ButtonConfig{Label: "Go"})
	dd := NewDropdownWidget(DropdownConfig{
		Label:   "Pick",
		Entries: []MenuEntry{{Label: "One", Command: "1"}},
	})

	vs := NewVStackWidget(inp, btn, dd)

	fm := NewFocusManager()
	fm.SetActive(true)
	fm.Collect(vs)

	var found bool
	for _, item := range fm.Items() {
		if item == Widget(dd) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("dropdown should be part of the focus ring")
	}
}
