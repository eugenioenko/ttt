package e2e

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

// A menu takes focus while it is open. Closing it must hand focus back, or key
// input goes to a menu widget that is no longer on screen and nothing responds.
func TestMenuBarDropdownRestoresFocusOnDismiss(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.app.Root.SetFocus(h.app.EditorGroup)

	h.exec("menu.view")
	if h.app.Root.Focused == h.app.EditorGroup {
		t.Fatal("dropdown should have taken focus")
	}

	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if h.app.Root.Focused != h.app.EditorGroup {
		t.Errorf("focus should return to the editor, got %T", h.app.Root.Focused)
	}
}

// Typed input has to reach the buffer again — the symptom the focus handoff
// exists to prevent.
func TestEditorAcceptsInputAfterDismissingMenu(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.app.EditorGroup.OpenFile(h.dir + "/alpha.txt")
	h.app.Root.SetFocus(h.app.EditorGroup)

	h.exec("menu.edit")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	h.pressRune('Z')

	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "Za" {
		t.Errorf("typing after dismissing a menu did not reach the buffer: line = %q, want %q", got, "Za")
	}
}

// Focus goes back where it came from, not unconditionally to the editor.
func TestMenuRestoresFocusToOpenerNotEditor(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.exec("sidebar.search")
	opener := h.app.Root.Focused
	if opener == h.app.EditorGroup {
		t.Fatal("expected the search panel to hold focus")
	}

	h.exec("menu.view")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if h.app.Root.Focused != opener {
		t.Errorf("focus should return to the search panel, got %T", h.app.Root.Focused)
	}
}

// Walking between dropdowns re-enters the open path with a menu focused; the
// original return target must survive that.
func TestMenuNavigationKeepsOriginalReturnFocus(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.app.Root.SetFocus(h.app.EditorGroup)

	h.exec("menu.file")
	h.pressKey(tcell.KeyRight, tcell.ModNone)
	h.pressKey(tcell.KeyRight, tcell.ModNone)
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if h.app.Root.Focused != h.app.EditorGroup {
		t.Errorf("focus should return to the editor after navigating dropdowns, got %T", h.app.Root.Focused)
	}
}

// Restoring focus must not fight the selected command: a command that focuses
// something itself runs after the handoff and wins.
func TestMenuCommandFocusWinsOverRestore(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.app.Root.SetFocus(h.app.EditorGroup)

	h.exec("menu.view")
	// View menu: Command Palette, Quick Open, ─, Explore (separators are skipped).
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	if h.app.Root.Focused == h.app.EditorGroup {
		t.Error("Explore should have taken focus, but it was restored to the editor")
	}
	if h.app.Sidebar.ActivePanel != "explorer" {
		t.Errorf("expected the explorer panel, got %q", h.app.Sidebar.ActivePanel)
	}
}

// Right-clicking the editor opens the same kind of menu through a different
// path, and has the same handoff obligation.
func TestEditorContextMenuRestoresFocusOnDismiss(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.app.EditorGroup.OpenFile(h.dir + "/alpha.txt")
	h.app.Root.SetFocus(h.app.EditorGroup)
	h.redraw()

	h.rightClick(40, 8)
	h.assertContains("Go to Definition")

	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if h.app.Root.Focused != h.app.EditorGroup {
		t.Errorf("focus should return to the editor, got %T", h.app.Root.Focused)
	}
}
