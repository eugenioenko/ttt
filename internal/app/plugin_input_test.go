package app

import (
	"context"
	"strings"
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

func TestPluginTypeEventAllocationLimit(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"ASCII limit", strings.Repeat("x", maxPluginTypeBytes), maxPluginTypeBytes},
		{"UTF-8 limit", strings.Repeat("é", maxPluginTypeBytes/2), maxPluginTypeBytes / 2},
		{"ASCII overflow", strings.Repeat("x", maxPluginTypeBytes+1), -1},
		{"UTF-8 overflow", strings.Repeat("é", maxPluginTypeBytes/2+1), -1},
		{"before CRLF normalization", strings.Repeat("\r\n", maxPluginTypeBytes/2+1), -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events, err := pluginTextEvents(tc.text)
			if tc.want < 0 {
				if err == nil || !strings.Contains(err.Error(), "use automation paste") || events != nil {
					t.Fatalf("oversized type returned %d events, %v", len(events), err)
				}
			} else if err != nil || len(events) != tc.want {
				t.Fatalf("type returned %d events, %v; want %d", len(events), err, tc.want)
			}
		})
	}
}

func TestPluginTypingRejectsOversizedTextWithoutEditing(t *testing.T) {
	for _, api := range []string{"type", "automation"} {
		t.Run(api, func(t *testing.T) {
			a := newListenTestApp(t)
			p := wiredInputTestPlugin(t, a)
			text := strings.Repeat("x", maxPluginTypeBytes+1)
			var err error
			if api == "type" {
				err = runExecMain(a, func() error { return p.SimulateText(text) })
			} else {
				_, err = a.runPluginAutomation(context.Background(), p, "type", text)
			}
			if err == nil || !strings.Contains(err.Error(), "type input exceeds") {
				t.Fatalf("oversized type = %v", err)
			}
			if got := inputTestBuffer(t, a); got != "" {
				t.Fatalf("rejected type edited the buffer (%d bytes)", len(got))
			}
			if _, err := a.runPluginAutomation(context.Background(), p, "paste", text); err != nil {
				t.Fatal(err)
			}
			if got := inputTestBuffer(t, a); got != text {
				t.Fatalf("bulk paste inserted %d bytes, want %d", len(got), len(text))
			}
		})
	}
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
					if err := runExecMain(a, func() error { return p.SimulateText(text) }); err != nil {
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
				if err := p.SimulateText("stale"); err != nil {
					return err
				}
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
						return p.SimulateText("fresh")
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
