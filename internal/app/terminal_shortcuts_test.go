package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
)

func TestTerminalShortcutSettingsStartupAndLiveApply(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	s := config.DefaultSettings()
	if s.Terminal.ForwardShortcuts {
		t.Fatal("TTT shortcuts must have priority by default")
	}
	s.Terminal.ForwardShortcuts = true
	a := buildTestApp(t, s)
	if !a.Root.ForwardShortcuts {
		t.Fatal("startup ignored terminal.forwardShortcuts")
	}
	s.Terminal.ForwardShortcuts = false
	a.ApplySettings(s)
	if a.Root.ForwardShortcuts {
		t.Fatal("live apply ignored terminal.forwardShortcuts")
	}

	entries := 0
	for _, category := range settingsCategories() {
		section := ""
		for _, field := range category.Fields {
			if field.Kind == settingSection {
				section = field.Label
			}
			if field.Label == i18n.Text("settings.forwardTerminalShortcuts") || field.Command == "terminal.sendNextKey" {
				entries++
				if category.Title != i18n.Text("settings.general") || section != i18n.Text("settings.keyboard") {
					t.Fatal("terminal keyboard controls must live under General → Keyboard")
				}
			}
		}
	}
	if entries != 2 {
		t.Fatalf("found %d keyboard controls, want setting and shortcut", entries)
	}
}

func TestTerminalShortcutSessionToggle(t *testing.T) {
	for _, saved := range []bool{false, true} {
		s := config.DefaultSettings()
		s.Terminal.ForwardShortcuts = saved
		a := buildTestApp(t, s)
		a.ToggleTerminalShortcutForwarding()
		wantNotice := i18n.Text("status.terminalShortcutsForwarded")
		if saved {
			wantNotice = i18n.Text("status.tttShortcutsPrioritized")
		}
		if a.Status.Notification != wantNotice {
			t.Fatalf("toggle notice = %q, want %q", a.Status.Notification, wantNotice)
		}
		if a.Root.ForwardShortcuts == saved || a.Settings.Terminal.ForwardShortcuts != saved {
			t.Fatal("toggle must change current state without changing the saved default")
		}
		s.Editor.WordWrap = !s.Editor.WordWrap
		a.ApplySettings(s)
		if a.Root.ForwardShortcuts == saved {
			t.Fatal("unrelated settings reset the session override")
		}
		next := buildTestApp(t, *a.Settings)
		if next.Root.ForwardShortcuts != saved {
			t.Fatal("new session did not restore the saved default")
		}
		s.Terminal.ForwardShortcuts = !saved
		a.ApplySettings(s)
		if a.Root.ForwardShortcuts != s.Terminal.ForwardShortcuts {
			t.Fatal("changed saved default did not apply")
		}
	}
}
