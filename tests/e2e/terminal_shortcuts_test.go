package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestTerminalShortcutForwarding(t *testing.T) {
	for _, command := range []string{"terminal.toggle", "terminal.newInEditor"} {
		t.Run(command, func(t *testing.T) {
			h := newTestHarness(t, 120, 35)
			defer h.stop()
			h.app.Settings.Terminal.Shell = "/bin/sh"
			h.app.Settings.Terminal.Args = []string{"-c", "stty raw -echo; printf e2e-forward-ready; exec cat"}
			h.exec(command)
			entry := h.app.Terminals
			if command == "terminal.newInEditor" {
				entry = h.app.EditorTerminals
			}
			dev := entry[0].Term
			terminalFocus := h.app.Root.Focused
			defer dev.Close()
			waitForRawTail(t, dev, "e2e-forward-ready")

			h.pressCtrl(tcell.KeyCtrlK)
			h.pressCtrl(tcell.KeyCtrlP)
			if h.app.Root.HasOverlay() || h.app.Root.AwaitingTerminalKey() {
				t.Fatal("Ctrl+K Ctrl+P opened the palette or stayed armed")
			}
			waitForRawTail(t, dev, "\x10")
			h.pressCtrl(tcell.KeyCtrlP)
			if !h.app.Root.HasOverlay() {
				t.Fatal("forwarding must apply only once")
			}
			palette, ok := h.app.Root.Focused.(*ui.SelectDialogWidget)
			if !ok {
				t.Fatal("command palette did not take focus")
			}
			palette.OnExecute("terminal.sendNextKey")
			if !h.app.Root.AwaitingTerminalKey() {
				t.Fatal("palette command did not restore terminal focus and arm forwarding")
			}
			h.pressRune('t')
			waitForRawTail(t, dev, "\x10t")
			h.pressCtrl(tcell.KeyCtrlP)
			palette = h.app.Root.Focused.(*ui.SelectDialogWidget)
			palette.OnExecute("terminal.toggleShortcutForwarding")
			if !h.app.Root.ForwardShortcuts || h.app.Root.Focused != terminalFocus || h.app.Settings.Terminal.ForwardShortcuts {
				t.Fatal("session toggle must preserve terminal focus and the saved default")
			}
			h.pressKey(tcell.KeyF6, tcell.ModCtrl)
			if h.app.Root.ForwardShortcuts {
				t.Fatal("Ctrl+F6 did not toggle back while direct forwarding was enabled")
			}

			before := len(h.app.Terminals)
			h.pressCtrl(tcell.KeyCtrlK)
			h.pressRune('t')
			if len(h.app.Terminals) != before+1 {
				t.Fatal("Ctrl+K T did not create a terminal")
			}
			active := h.app.Terminals[len(h.app.Terminals)-1].Term
			defer active.Close()
			waitForRawTail(t, active, "e2e-forward-ready")
			s := *h.app.Settings
			s.Terminal.ForwardShortcuts = true
			h.app.ApplySettings(s)
			h.pressCtrl(tcell.KeyCtrlK)
			h.pressCtrl(tcell.KeyCtrlP)
			if h.app.Root.HasOverlay() || h.app.Root.AwaitingTerminalKey() {
				t.Fatal("direct forwarding captured a shortcut")
			}
			waitForRawTail(t, active, "\x0b\x10")
		})
	}
}
