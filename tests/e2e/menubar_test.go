package e2e

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

const menuBarRow = "File"

func TestMenuBarToggleHidesAndRestores(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	if !strings.Contains(h.screenRow(0), menuBarRow) {
		t.Fatalf("menu bar should be on row 0 by default, got %q", h.screenRow(0))
	}

	h.exec("menubar.toggle")

	if h.app.Settings.Editor.IsMenuBarVisible() {
		t.Error("setting should report the menu bar as hidden after toggle")
	}
	if h.app.MenuBar.Visible {
		t.Error("menu bar widget should be hidden after toggle")
	}
	if strings.Contains(h.screenRow(0), menuBarRow) {
		t.Errorf("row 0 should be reclaimed when the menu bar is hidden, got %q", h.screenRow(0))
	}

	h.exec("menubar.toggle")

	if !h.app.Settings.Editor.IsMenuBarVisible() {
		t.Error("setting should report the menu bar as visible after second toggle")
	}
	if !strings.Contains(h.screenRow(0), menuBarRow) {
		t.Errorf("menu bar should be back on row 0, got %q", h.screenRow(0))
	}
}

// Hiding must say how to get the bar back — the command is otherwise only
// reachable through a menu that is no longer on screen.
func TestMenuBarHideShowsRestoreHint(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.exec("menubar.toggle")

	h.assertContains("Menu bar hidden")
	h.assertContains("Alt+Shift+M")
}

// With no shortcut bound, the hint has to name the command instead — otherwise
// it says the bar is hidden and nothing about getting it back.
func TestMenuBarHintFallsBackToCommandName(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.reg.ClearAllShortcuts()
	h.exec("menubar.toggle")

	h.assertContains("View: Toggle Menu Bar")
}

// The menu.* shortcuts stay bound while the bar is hidden — the dropdown floats
// on its own and the bar stays hidden throughout.
func TestMenuDropdownFloatsWhileMenuBarHidden(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.exec("menubar.toggle")
	if h.app.MenuBar.Visible {
		t.Fatal("menu bar should start hidden for this test")
	}

	h.exec("menu.view")

	h.assertContains("Command Palette")
	if h.app.MenuBar.Visible {
		t.Error("opening a dropdown must not reveal the hidden menu bar")
	}
	if strings.Contains(h.screenRow(0), menuBarRow) {
		t.Errorf("menu bar drawn on row 0 while hidden, got %q", h.screenRow(0))
	}

	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if h.app.MenuBar.Visible || h.app.Settings.Editor.IsMenuBarVisible() {
		t.Error("menu bar should still be hidden after the dropdown closes")
	}
	h.assertNotContains("Command Palette")
}

// A floating dropdown starts at the top edge — the row the bar would have used.
func TestFloatingDropdownAnchorsToTopRow(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.exec("menubar.toggle")
	h.exec("menu.view")

	if row := h.screenRow(0); !strings.ContainsRune(row, '╭') {
		t.Errorf("dropdown should start on row 0 when the bar is hidden, got %q", row)
	}
}
