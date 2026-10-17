package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/plugin"
)

func focusPluginInput(t *testing.T, h *testHarness) {
	t.Helper()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "chat.lua")
	if err := os.WriteFile(src, []byte(`
		require("ttt").register({ bottom = { title = "Chat", render = function(panel)
			panel:input({ placeholder = "say" })
		end } })
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("chat")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	t.Cleanup(p.Destroy)
	h.app.ContentSplit.ShowBottom = true
	h.app.BottomPanel.SetActivePanel("plugin.chat")
	h.redraw()
	h.app.Root.SetFocus(h.app.BottomPanel.ActiveWidget())
	h.redraw()
}

func TestPasteGoesToAFocusedPluginInput(t *testing.T) {
	h := newTestHarness(t, 160, 30)
	defer h.stop()
	focusPluginInput(t, h)
	h.app.PasteText("pasted text")
	h.redraw()
	h.assertContains("pasted text")
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "" {
		t.Fatalf("editor line = %q, want the paste only in the plugin input", got)
	}
}

func TestPasteCommandGoesToAFocusedPluginInput(t *testing.T) {
	h := newTestHarness(t, 160, 30)
	defer h.stop()
	clipboard.DisableSystem()
	focusPluginInput(t, h)
	clipboard.Set("from clipboard")
	h.exec("editor.paste")
	h.redraw()
	h.assertContains("from clipboard")
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "" {
		t.Fatalf("editor line = %q, want the paste only in the plugin input", got)
	}
}

func TestPasteGoesToAPluginTabInput(t *testing.T) {
	h := newTestHarness(t, 160, 30)
	defer h.stop()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "tab.lua")
	if err := os.WriteFile(src, []byte(`
		function show()
			require("ttt").open_tab({ title = "T", render = function(panel)
				panel:input({ placeholder = "say" })
			end })
		end
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("tab")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	t.Cleanup(p.Destroy)
	if err := p.State.DoString(`show()`); err != nil {
		t.Fatal(err)
	}
	h.redraw()
	h.pressRune('k')
	h.app.PasteText("pasted")
	h.redraw()
	h.assertContains("kpasted")
}
