package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/plugin"
	lua "github.com/yuin/gopher-lua"
)

func loadTabKeysPlugin(t *testing.T, h *testHarness, name, source string) *plugin.Plugin {
	t.Helper()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, name+".lua")
	if err := os.WriteFile(src, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin(name)
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	p.SetPostAsync(func(*plugin.PluginAsyncResult) {})
	return p
}

func runPluginLua(t *testing.T, h *testHarness, p *plugin.Plugin, code string) {
	t.Helper()
	if err := p.State.DoString(code); err != nil {
		t.Fatal(err)
	}
	h.redraw()
}

func tabIndexByPath(h *testHarness, path string) int {
	for i := range h.app.EditorGroup.TabCount() {
		if p, _ := h.app.EditorGroup.TabInfo(i); p == path {
			return i
		}
	}
	return -1
}

func TestPluginTabIDIsNotABufferAtTheSamePath(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	p := loadTabKeysPlugin(t, h, "pages", `
		ttt = require("ttt")
		function open(name)
			return ttt.open_tab({ id = name, title = "Page " .. name,
				render = function(panel) panel:label("content of " .. name) end })
		end
	`)
	defer p.Destroy()
	eg := h.app.EditorGroup

	runPluginLua(t, h, p, `ttt.open_readonly("Lookalike", {"readonly text"}, "plugin:pages:a")`)
	before := eg.TabCount()
	runPluginLua(t, h, p, `open("a")`)
	if eg.TabCount() != before+1 {
		t.Fatalf("open_tab reused the read-only buffer: tabs = %d, want %d", eg.TabCount(), before+1)
	}
	h.assertContains("content of a")

	runPluginLua(t, h, p, `ttt.close_tab("a")`)
	if eg.TabCount() != before {
		t.Fatalf("close_tab left %d tabs, want %d", eg.TabCount(), before)
	}
	runPluginLua(t, h, p, `ttt.close_tab("a"); ttt.close_tab("plugin:pages:a")`)
	if eg.TabCount() != before {
		t.Fatalf("close_tab closed the read-only buffer: tabs = %d, want %d", eg.TabCount(), before)
	}
	if tabIndexByPath(h, "plugin:pages:a") < 0 {
		t.Fatal("read-only buffer at plugin:pages:a is gone")
	}
}

func TestPluginFileHandlerTabIDClosesAndFocuses(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	p := loadTabKeysPlugin(t, h, "pdfview", `
		ttt = require("ttt")
		ttt.register({ file_handlers = {
			{ id = "pdf", magic = "%PDF-", open = function(path, tab)
				tab_id = tab.id
				tab.render = function(panel) panel:label("PDF view") end
			end },
		} })
	`)
	defer p.Destroy()
	eg := h.app.EditorGroup

	pdf := filepath.Join(h.dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01\x02\xff"), 0644); err != nil {
		t.Fatal(err)
	}
	text := filepath.Join(h.dir, "alpha.txt")
	eg.OpenFile(pdf)
	eg.OpenFile(text)
	h.redraw()
	if got := p.State.GetGlobal("tab_id"); got != lua.LString(pdf) {
		t.Fatalf("tab.id = %v, want %q", got, pdf)
	}

	runPluginLua(t, h, p, `focused = ttt.focus_tab(tab_id)`)
	if p.State.GetGlobal("focused") != lua.LTrue || eg.ActiveTabIndex() != tabIndexByPath(h, pdf) {
		t.Fatalf("focus_tab(tab.id) = %v, active tab %d, want the PDF tab", p.State.GetGlobal("focused"), eg.ActiveTabIndex())
	}
	h.assertContains("PDF view")

	runPluginLua(t, h, p, `ttt.close_tab(tab_id)`)
	if tabIndexByPath(h, pdf) >= 0 {
		t.Fatal("close_tab(tab.id) left the PDF tab open")
	}
	runPluginLua(t, h, p, `ttt.close_tab("`+text+`")`)
	if tabIndexByPath(h, text) < 0 {
		t.Fatal("close_tab closed a text file tab the plugin does not own")
	}
	runPluginLua(t, h, p, `focused = ttt.focus_tab(tab_id)`)
	if p.State.GetGlobal("focused") != lua.LFalse {
		t.Fatal("focus_tab found the closed PDF tab")
	}
}
