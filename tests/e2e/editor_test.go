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

func paletteBorderWidth(row string) int {
	runes := []rune(row)
	for _, border := range [][2]rune{{'╭', '╮'}, {'╔', '╗'}, {'┌', '┐'}} {
		start := -1
		for i, r := range runes {
			if r == border[0] {
				start = i
				break
			}
		}
		if start < 0 {
			continue
		}
		for i := start + 1; i < len(runes); i++ {
			if runes[i] == border[1] {
				return i - start + 1
			}
		}
	}
	return 0
}

func TestCommandPaletteResponsiveWidths(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		want   int
	}{
		{name: "narrow", width: 30, height: 24, want: 26},
		{name: "typical", width: 80, height: 24, want: 48},
		{name: "wide", width: 200, height: 50, want: 90},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHarness(t, tt.width, tt.height)
			defer h.stop()

			h.pressCtrl(tcell.KeyCtrlP)
			if got := paletteBorderWidth(h.screenRow(2)); got != tt.want {
				t.Fatalf("command palette width=%d, want %d", got, tt.want)
			}
		})
	}
}

func TestCommandPaletteSharedWidthBindingsAndTransitions(t *testing.T) {
	h := newTestHarness(t, 200, 50)
	defer h.stop()

	h.pressCtrl(tcell.KeyCtrlP)
	palette, ok := h.app.Root.TopOverlayWidget().(*ui.SelectDialogWidget)
	if !ok {
		t.Fatalf("Ctrl+P opened %T, want command palette", h.app.Root.TopOverlayWidget())
	}
	if palette.Input.Text != ">" || len(palette.Items) != len(h.reg.List()) {
		t.Fatalf("Ctrl+P input=%q items=%d, want > and complete registry of %d", palette.Input.Text, len(palette.Items), len(h.reg.List()))
	}
	if got := paletteBorderWidth(h.screenRow(2)); got != 90 {
		t.Fatalf("Ctrl+P command palette width=%d, want 90; row=%q", got, h.screenRow(2))
	}

	h.pressRune('?')
	if got := paletteBorderWidth(h.screenRow(2)); got != 90 {
		t.Fatalf("help palette width=%d, want shared width 90", got)
	}
	palette.Selected = 3

	h.pressKey(tcell.KeyBackspace2, tcell.ModNone)
	h.pressRune('>')
	if got := paletteBorderWidth(h.screenRow(2)); got != 90 {
		t.Fatalf("help-to-command palette width=%d, want 90", got)
	}
	if palette.Selected != 0 {
		t.Fatalf("help-to-command selected=%d, want 0", palette.Selected)
	}

	h.pressKey(tcell.KeyBackspace2, tcell.ModNone)
	if palette.Input.Text != "" {
		t.Fatalf("file-mode input=%q, want empty", palette.Input.Text)
	}
	if got := paletteBorderWidth(h.screenRow(2)); got != 90 {
		t.Fatalf("file palette width=%d, want 90", got)
	}
	h.assertContains("alpha.txt")

	h.pressRune(':')
	if palette.Input.Text != ":" {
		t.Fatalf("go-to-line input=%q, want colon prefix", palette.Input.Text)
	}
	if got := paletteBorderWidth(h.screenRow(2)); got != 90 {
		t.Fatalf("go-to-line palette width=%d, want 90", got)
	}

	h.pressKey(tcell.KeyBackspace2, tcell.ModNone)
	h.click(54, 3)
	if h.app.Root.HasOverlay() {
		t.Fatal("outside click did not dismiss file palette")
	}

	h.pressCtrl(tcell.KeyCtrlP)
	if got := paletteBorderWidth(h.screenRow(2)); got != 90 {
		t.Fatalf("reopened command palette width=%d, want 90", got)
	}
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	h.pressCtrl(tcell.KeyCtrlK)
	h.pressRune('p')
	palette, ok = h.app.Root.TopOverlayWidget().(*ui.SelectDialogWidget)
	if !ok {
		t.Fatalf("Ctrl+K P opened %T, want file search", h.app.Root.TopOverlayWidget())
	}
	if palette.Input.Text != "" {
		t.Fatalf("Ctrl+K P input=%q, want empty file search", palette.Input.Text)
	}
	if got := paletteBorderWidth(h.screenRow(2)); got != 90 {
		t.Fatalf("Ctrl+K P file palette width=%d, want 90", got)
	}
}

func TestCommandPaletteHelpOrientsThenNavigates(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.pressCtrl(tcell.KeyCtrlP)
	h.pressRune('?')

	h.assertContains("Workspace map")
	h.assertContains("folders, tabs, and editor groups")
	h.assertNotContains("Open Folder")

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

func TestCommandPaletteHelpNoMatchAndEscape(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.pressCtrl(tcell.KeyCtrlP)
	h.pressRune('?')
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.assertContains("Open Folder")

	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	h.assertContains("Workspace map")
	for _, r := range "qxzvjk" {
		h.pressRune(r)
	}
	h.assertContains(`No help entries match "qxzvjk"`)
	h.assertContains("Try > for all commands")

	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected help palette to dismiss, got %d overlays", len(h.app.Root.Overlays))
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
