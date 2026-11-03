package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func TestKeyboardModifierStartupAndLiveApply(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	s := config.DefaultSettings()
	s.Keyboard.ModifierKey = config.ModifierSuper
	a := buildTestApp(t, s)
	a.Reg = command.NewRegistry()
	fired := 0
	a.Reg.Register(command.Command{ID: "test.custom", Handler: func() { fired++ }})
	a.Keybindings = []config.KeyBinding{{Key: "ctrl+alt+j", Command: "test.custom"}}
	a.RebindKeys()
	if a.Root.ControlModifiers != tcell.ModMeta {
		t.Fatal("startup ignored modifier setting")
	}
	for _, tc := range []struct {
		mode, label string
		mask        tcell.ModMask
	}{
		{config.ModifierSuper, "Super+Alt+J", tcell.ModMeta},
		{config.ModifierControlSuper, "Ctrl/Super+Alt+J", tcell.ModCtrl | tcell.ModMeta},
		{config.ModifierControl, "Ctrl+Alt+J", tcell.ModCtrl},
	} {
		s.Keyboard.ModifierKey = tc.mode
		a.ApplySettings(s)
		if a.Keybindings[0].Key != "ctrl+alt+j" {
			t.Fatal("setting rewrote binding definitions")
		}
		if a.Root.ControlModifiers != tc.mask {
			t.Fatalf("live mode = %v, want %v", a.Root.ControlModifiers, tc.mask)
		}
		cmd, _ := a.Reg.Get("test.custom")
		if cmd.Shortcut != tc.label || a.KeyFor("test.custom") != tc.label {
			t.Fatalf("shortcut=%q KeyFor=%q, want %q", cmd.Shortcut, a.KeyFor("test.custom"), tc.label)
		}
		before := fired
		a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "j", tcell.ModMeta|tcell.ModAlt))
		if (fired > before) != (tc.mask&tcell.ModMeta != 0) {
			t.Fatal("custom binding did not follow live setting")
		}
	}
}
