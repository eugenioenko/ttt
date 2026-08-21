package e2e

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestStartup(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.assertContains("File")
	h.assertContains("Edit")
	h.assertContains("View")
	h.assertContains("Explore")
}

func TestMenuBarRendered(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	row := h.screenRow(0)
	if !strings.Contains(row, "File") {
		t.Errorf("menu bar should contain 'File', got: %s", row)
	}
	if !strings.Contains(row, "Help") {
		t.Errorf("menu bar should contain 'Help', got: %s", row)
	}
}

func TestNewFile(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")
	if !h.app.EditorGroup.IsActiveVirtual() {
		t.Error("expected new file tab to be virtual")
	}
	h.assertContains("untitled")
}

func TestCommandPaletteOpenClose(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("command.palette")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected 1 overlay, got %d", len(h.app.Root.Overlays))
	}

	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected 0 overlays after Escape, got %d", len(h.app.Root.Overlays))
	}
}

func TestCommandPaletteDoesNotStack(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.pressCtrl(tcell.KeyCtrlP)
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected 1 overlay after first Ctrl+P, got %d", len(h.app.Root.Overlays))
	}

	h.pressCtrl(tcell.KeyCtrlP)
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected 1 overlay after second Ctrl+P, got %d", len(h.app.Root.Overlays))
	}
}

func TestCommandPaletteHelpOrientsThenNavigates(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.pressCtrl(tcell.KeyCtrlP)
	h.pressRune('?')

	// Empty help is an orientation surface, not a command shortlist. The first
	// topic explains the workspace model in the dedicated detail row.
	h.assertContains("Workspace map")
	h.assertContains("folders, tabs, and editor groups")
	h.assertNotContains("Open Folder")

	// Search spans the registered command list, and its displayed shortcut is
	// the one already derived from the configured keybindings by BindKeys.
	for _, r := range "Open Folder" {
		h.pressRune(r)
	}
	h.assertContains("Open Folder")
	cmd, ok := h.reg.Get("workspace.openFolder")
	if !ok {
		t.Fatal("workspace.openFolder command is not registered")
	}
	if cmd.Shortcut == "" {
		t.Fatal("workspace.openFolder should have a derived shortcut")
	}
	h.assertContains(cmd.Shortcut)

	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if len(h.app.Root.Overlays) == 0 {
		t.Fatal("executing Open Folder from help should open its dialog")
	}
}

func TestGoToLineDialog(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("editor.goToLine")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected 1 overlay, got %d", len(h.app.Root.Overlays))
	}

	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected 0 overlays after Escape, got %d", len(h.app.Root.Overlays))
	}
}

func TestFindDialog(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("search.find")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected 1 overlay, got %d", len(h.app.Root.Overlays))
	}

	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected 0 overlays after Escape, got %d", len(h.app.Root.Overlays))
	}
}

func TestFindDialogRefocus(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("search.find")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected 1 overlay, got %d", len(h.app.Root.Overlays))
	}

	fb, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatal("expected FindBarWidget overlay")
	}

	h.click(40, 12)
	_, _, vis := fb.CursorPosition()
	if vis {
		t.Fatal("expected find bar cursor hidden after clicking editor")
	}

	h.exec("search.find")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected still 1 overlay, got %d", len(h.app.Root.Overlays))
	}
	_, _, vis = fb.CursorPosition()
	if !vis {
		t.Fatal("expected find bar cursor visible after re-invoking search.find")
	}
}

func TestCommandPaletteOpensOverFindBar(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("search.find")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected 1 overlay, got %d", len(h.app.Root.Overlays))
	}
	if _, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget); !ok {
		t.Fatal("expected FindBarWidget overlay")
	}

	h.exec("command.palette")
	if len(h.app.Root.Overlays) != 2 {
		t.Fatalf("expected 2 overlays after command.palette, got %d", len(h.app.Root.Overlays))
	}
	if _, ok := h.app.Root.TopOverlayWidget().(*ui.SelectDialogWidget); !ok {
		t.Fatalf("expected SelectDialogWidget on top, got %T", h.app.Root.TopOverlayWidget())
	}
}

func TestThemeSwitchDialog(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("theme.switch")
	if len(h.app.Root.Overlays) == 1 {
		h.pressKey(tcell.KeyEscape, tcell.ModNone)
		if len(h.app.Root.Overlays) != 0 {
			t.Fatalf("expected 0 overlays after Escape, got %d", len(h.app.Root.Overlays))
		}
	}
}
