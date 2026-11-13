package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestPluginCommandTitle(t *testing.T) {
	for _, tt := range []struct{ name, display, manifest, title, want string }{
		{"my-plugin", "My Plugin", "", "Open Dashboard", "My Plugin: Open Dashboard"},
		{"my-plugin", "", "", "Settings", "my-plugin: Settings"},
		{" my-plugin ", " \t", "", " Refresh ", "my-plugin: Refresh"},
		{"", "", "manifest", "Refresh", "manifest: Refresh"},
		{"", "", "", "Refresh", "Refresh"},
		{"my-plugin", "My Plugin", "", "My Plugin: Open Dashboard", "My Plugin: Open Dashboard"},
		{"my-plugin", "My Plugin", "", " my plugin  — Settings ", " my plugin  — Settings "},
		{"my-plugin", "My Plugin", "", "my-plugin | Refresh", "my-plugin | Refresh"},
		{"my-plugin", "My Plugin", "", "My Plugin - Refresh", "My Plugin - Refresh"},
		{"my-plugin", "My Plugin", "", "My Plugins: Refresh", "My Plugin: My Plugins: Refresh"},
		{"demo", "Kit", "", "kit: Open", "kit: Open"},
		{"demo", "kit", "", "Kit: Open", "Kit: Open"},
		{"demo", "Kit", "", " kit  - Open ", " kit  - Open "},
		{"demo", "kit", "", "\tKit – Open\t", "\tKit – Open\t"},
		{"demo", "Kit", "", "kit — Open", "kit — Open"},
		{"demo", "kit", "", "Kit | Open", "Kit | Open"},
		{"demo", "ſync", "", "Sync: Open", "Sync: Open"},
		{"demo", "Sync", "", "ſync: Open", "ſync: Open"},
		{"Kit", "Demo", "", "kit: Open", "kit: Open"},
		{"demo", "Demo", "Kit", "kit: Open", "kit: Open"},
		{"demo", "Kit", "", "kits: Open", "Kit: kits: Open"},
		{"demo", "kit", "", "Kit Open", "kit: Kit Open"},
		{"demo", "kit", "", "K", "kit: K"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			p := &plugin.Plugin{Name: tt.name, Manifest: plugin.Manifest{Name: tt.manifest, DisplayName: tt.display}}
			if got := pluginCommandTitle(p, tt.title); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPluginCommandRegistrationPreservesExecution(t *testing.T) {
	a := &App{Reg: command.NewRegistry(), Root: ui.NewRoot(nil)}
	calls := 0
	p := &plugin.Plugin{Name: "demo", Manifest: plugin.Manifest{DisplayName: "Demo Plugin"},
		Commands:          []plugin.PluginCommand{{ID: "demo.open", Title: "Open Dashboard", Handler: func() error { calls++; return nil }}},
		PluginKeybindings: []plugin.PluginKeybinding{{Key: "alt+j", Command: "demo.open"}}}
	a.registerPluginCommandsAndKeys(p)
	cmd, ok := a.Reg.FindByTitle("Open Dashboard")
	if !ok || cmd.ID != "demo.open" || cmd.PaletteTitle() != "Demo Plugin: Open Dashboard" || cmd.Shortcut == "" {
		t.Fatalf("command: %+v", cmd)
	}
	if !a.Reg.Execute("demo.open") || calls != 1 {
		t.Fatal("ID execution failed")
	}
	a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "j", tcell.ModAlt))
	if calls != 2 {
		t.Fatalf("keybinding calls = %d", calls)
	}
	if p.Commands[0].Title != "Open Dashboard" {
		t.Fatal("plugin title changed")
	}
}
