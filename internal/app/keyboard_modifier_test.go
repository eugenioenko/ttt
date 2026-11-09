package app

import (
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
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
		{config.ModifierControlSuper, "Mod+Alt+J", tcell.ModCtrl | tcell.ModMeta},
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

func TestKeyboardModifierCommandLinePreservesHookEvent(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	s := config.DefaultSettings()
	s.Keyboard.ModifierKey = config.ModifierSuper
	a := buildTestApp(t, s)
	w := a.ShowCommandLine(":", nil, nil, nil)
	w.Input().SetText("alpha beta")
	key := tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModMeta)
	var seen *tcell.EventKey
	w.OnKey = func(ev *tcell.EventKey) bool { seen = ev; return false }
	a.Root.HandleEvent(key)
	if seen != key || !w.Input().HasSelection() {
		t.Fatal("command-line hook must receive the original event and the input must accept Super+A")
	}
}

func TestKeyboardModifierRefreshSkipsUnboundShortcuts(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	s := config.DefaultSettings()
	a := buildTestApp(t, s)
	a.Reg = command.NewRegistry()
	a.Keybindings = []config.KeyBinding{
		{Key: "ctrl+alt+j", Command: "test.valid"},
		{Key: "", Command: "test.empty"},
		{Key: "invalid+j", Command: "test.invalid"},
		{Key: "invalid+j", Command: "test.valid"},
	}
	p := &plugin.Plugin{Name: "shortcuts", Enabled: true, PluginKeybindings: []plugin.PluginKeybinding{
		{Key: "ctrl+alt+h", Command: "test.pluginValid"},
		{Key: "", Command: "test.pluginEmpty"},
		{Key: "invalid+h", Command: "test.pluginInvalid"},
		{Key: "invalid+h", Command: "test.pluginValid"},
	}}
	for _, id := range []string{"test.valid", "test.empty", "test.invalid", "test.pluginValid", "test.pluginEmpty", "test.pluginInvalid"} {
		a.Reg.Register(command.Command{ID: id})
	}
	a.RebindKeys()
	a.PluginManager = plugin.NewManager(t.TempDir(), filepath.Join(t.TempDir(), "plugins.json"))
	a.PluginManager.RegisterDebugPlugin(p)
	a.registerPluginCommandsAndKeys(p)
	for _, mode := range []string{config.ModifierControl, config.ModifierControlSuper, config.ModifierSuper} {
		s.Keyboard.ModifierKey = mode
		a.ApplySettings(s)
		for id, want := range map[string]string{
			"test.valid":         FormatKeyBinding("ctrl+alt+j", a.Root.ControlModifiers),
			"test.pluginValid":   FormatKeyBinding("ctrl+alt+h", a.Root.ControlModifiers),
			"test.empty":         "",
			"test.invalid":       "",
			"test.pluginEmpty":   "",
			"test.pluginInvalid": "",
		} {
			cmd, _ := a.Reg.Get(id)
			if cmd.Shortcut != want {
				t.Errorf("mode %s: %s shortcut = %q, want %q", mode, id, cmd.Shortcut, want)
			}
		}
	}
}
