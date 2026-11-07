package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/plugin"
	lua "github.com/yuin/gopher-lua"
)

func TestPluginOpenTabIDFocusesTheOpenTabAndCloses(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "pages.lua")
	if err := os.WriteFile(src, []byte(`
		local ttt = require("ttt")
		closed = 0
		function open(name)
			return ttt.open_tab({
				id = name,
				title = "Page " .. name,
				render = function(panel) panel:label("content of " .. name) end,
				on_close = function() closed = closed + 1 end,
			})
		end
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("pages")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	defer p.Destroy()
	var posted []*plugin.PluginAsyncResult
	p.SetPostAsync(func(r *plugin.PluginAsyncResult) { posted = append(posted, r) })

	eg := h.app.EditorGroup
	before := eg.TabCount()
	run := func(code string) {
		t.Helper()
		if err := p.State.DoString(code); err != nil {
			t.Fatal(err)
		}
		h.redraw()
	}
	activePath := func() string {
		path, _ := eg.TabInfo(eg.ActiveTabIndex())
		return path
	}

	run(`open("a"); open("b")`)
	if eg.TabCount() != before+2 {
		t.Fatalf("tabs = %d, want %d", eg.TabCount(), before+2)
	}
	run(`id = open("a")`)
	if eg.TabCount() != before+2 {
		t.Fatalf("reopening id a added a tab: %d", eg.TabCount())
	}
	if got := activePath(); got != "plugin:pages:a" {
		t.Fatalf("active tab = %q, want the open tab a", got)
	}
	if got := p.State.GetGlobal("id"); got != lua.LString("a") {
		t.Fatalf("open_tab returned %v", got)
	}
	h.assertContains("content of a")

	run(`ttt = require("ttt"); ttt.close_tab("a")`)
	if eg.TabCount() != before+1 {
		t.Fatalf("close_tab left %d tabs", eg.TabCount())
	}
	for _, r := range posted {
		r.Callback()
	}
	if got := p.State.GetGlobal("closed"); got != lua.LNumber(1) {
		t.Fatalf("on_close ran %v times, want 1", got)
	}
}
