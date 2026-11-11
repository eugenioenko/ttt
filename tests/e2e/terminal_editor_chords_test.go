package e2e

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func TestTerminalChordsDoNotEditHiddenBuffer(t *testing.T) {
	for _, location := range []string{"terminal.toggle", "terminal.newInEditor"} {
		for _, tc := range []struct {
			name   string
			prefix tcell.Key
			key    rune
			bytes  string
		}{
			{"delete", tcell.KeyCtrlK, 'k', "k!"},
			{"join", tcell.KeyCtrlK, 'j', "j!"},
			{"sort", tcell.KeyCtrlK, 'o', "o!"},
			{"multicursor", tcell.KeyCtrlK, 'l', "l!"},
			{"format", tcell.KeyCtrlL, 'e', "\x0ce!"},
			{"lsp format", tcell.KeyCtrlL, 'f', "\x0cf!"},
			{"remapped delete", tcell.KeyCtrlG, 'x', "\x07x!"},
		} {
			t.Run(location+"/"+tc.name, func(t *testing.T) {
				h := newTestHarness(t, 100, 30)
				defer h.stop()
				h.app.Keybindings = append(config.DefaultKeybindings(), config.KeyBinding{Key: "ctrl+g x", Command: "editor.deleteLine"})
				h.app.RebindKeys()
				original := "zebra\napple\npear"
				h.app.NewFileFromText(original)
				buf := h.app.EditorGroup.ActiveBuffer()
				original = strings.Join(buf.Lines, "\n")
				version := buf.Version()
				h.app.Settings.Terminal.Shell = "/bin/sh"
				h.app.Settings.Terminal.Args = []string{"-c", "stty raw -echo; printf editor-chord-ready; exec cat"}
				h.exec(location)
				entries := h.app.Terminals
				if location == "terminal.newInEditor" {
					entries = h.app.EditorTerminals
				}
				dev := entries[0].Term
				defer dev.Close()
				waitForRawTail(t, dev, "editor-chord-ready")
				h.pressCtrl(tc.prefix)
				h.pressRune(tc.key)
				h.pressRune('!')
				if strings.Join(buf.Lines, "\n") != original || buf.Version() != version {
					t.Fatalf("terminal chord changed hidden buffer: %q", buf.Lines)
				}
				waitForRawTail(t, dev, tc.bytes)
				if h.app.Root.AwaitingTerminalKey() {
					t.Fatal("terminal forwarding remained armed")
				}
				h.pressCtrl(tcell.KeyCtrlK)
				h.pressRune('p')
				if !h.app.Root.HasOverlay() {
					t.Fatal("global Quick Open chord stopped working")
				}
			})
		}
	}
}
