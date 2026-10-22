package app

import (
	"context"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

func wiredInputTestPlugin(t *testing.T, a *App) *plugin.Plugin {
	t.Helper()
	p := &plugin.Plugin{Name: "input-test", Enabled: true, State: lua.NewState()}
	if err := runExecMain(a, func() error {
		a.PluginManager = &plugin.Manager{}
		a.WirePlugin(p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := runExecMain(a, func() error { p.Destroy(); return nil }); err != nil {
			t.Error(err)
		}
	})
	return p
}

func inputTestBuffer(t *testing.T, a *App) string {
	t.Helper()
	var text string
	if err := runExecMain(a, func() error { text = NewPluginEditorAPI(a).BufferText(); return nil }); err != nil {
		t.Fatal(err)
	}
	return text
}

func TestPluginTypingPreservesNewlines(t *testing.T) {
	for _, api := range []string{"type", "automation"} {
		for _, text := range []string{"one\ntwo", "one\r\ntwo", "one\rtwo"} {
			t.Run(api+"/"+text, func(t *testing.T) {
				a := newListenTestApp(t)
				p := wiredInputTestPlugin(t, a)
				ctrlJ := false
				if err := runExecMain(a, func() error {
					a.Root.AddGlobalKey(tcell.KeyCtrlJ, tcell.ModCtrl, 0, func() { ctrlJ = true })
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if api == "type" {
					if err := runExecMain(a, func() error { p.SimulateText(text); return nil }); err != nil {
						t.Fatal(err)
					}
				} else if _, err := a.runPluginAutomation(context.Background(), p, "type", text); err != nil {
					t.Fatal(err)
				}
				if got := inputTestBuffer(t, a); got != "one\ntwo" {
					t.Fatalf("typed text = %q", got)
				}
				if ctrlJ {
					t.Fatal("typing a newline ran the Ctrl+J binding")
				}
			})
		}
	}
}

func TestQueuedPluginInputExpiresWithOrigin(t *testing.T) {
	for _, change := range []string{"disable", "unload", "reload"} {
		t.Run(change, func(t *testing.T) {
			a := newListenTestApp(t)
			p := wiredInputTestPlugin(t, a)
			if err := runExecMain(a, func() error {
				p.SimulateText("stale")
				if err := p.SimulateKeys("enter"); err != nil {
					return err
				}
				if change == "disable" {
					p.Enabled = false
				} else {
					p.Destroy()
					if change == "reload" {
						p.State, p.Enabled = lua.NewState(), true
						a.WirePlugin(p)
						p.SimulateText("fresh")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			want := ""
			if change == "reload" {
				want = "fresh"
			}
			if got := inputTestBuffer(t, a); got != want {
				t.Fatalf("input after %s = %q, want %q", change, got, want)
			}
		})
	}
}
