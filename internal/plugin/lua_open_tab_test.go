package plugin

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestOpenTabWithIDFocusesExistingTab(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelEditor: true})
	defer done()
	open := map[string]string{}
	var opened, focused []string
	p.OpenTab = func(key, title string, _ *PluginPanelWidget) {
		open[key] = title
		opened = append(opened, key)
	}
	p.FocusTab = func(key string) bool {
		if _, ok := open[key]; ok {
			focused = append(focused, key)
			return true
		}
		return false
	}
	p.CloseTab = func(key string) { delete(open, key) }

	err := p.State.DoString(`
		local ttt = require("ttt")
		local r = function(panel) end
		a = ttt.open_tab({ id = "ns", title = "Namespaces", render = r })
		b = ttt.open_tab({ id = "ns", title = "Namespaces", render = r })
		c = ttt.open_tab({ title = "Legacy", render = r })
		ttt.close_tab("ns")
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(opened) != 2 || opened[0] != "plugin:test:ns" || opened[1] != "Legacy" {
		t.Fatalf("opened = %v", opened)
	}
	if len(focused) != 1 || focused[0] != "plugin:test:ns" {
		t.Fatalf("second open with the same id should focus the tab, focused = %v", focused)
	}
	if a, b, c := p.State.GetGlobal("a"), p.State.GetGlobal("b"), p.State.GetGlobal("c"); a.String() != "ns" || b.String() != "ns" || c.String() != "Legacy" {
		t.Fatalf("returned ids = %v %v %v", a, b, c)
	}
	if _, ok := open["plugin:test:ns"]; ok {
		t.Fatal("close_tab should close the tab opened with that id")
	}
	if open["Legacy"] != "Legacy" {
		t.Fatal("a tab opened without an id keeps its title as the key")
	}
}

func TestOpenTabOnCloseRunsOnceAfterClose(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelEditor: true})
	defer done()
	var posted []*PluginAsyncResult
	p.PostAsync = func(r *PluginAsyncResult) { posted = append(posted, r) }
	var panel *PluginPanelWidget
	p.OpenTab = func(_, _ string, w *PluginPanelWidget) { panel = w }

	err := p.State.DoString(`
		local ttt = require("ttt")
		closed = 0
		ttt.open_tab({ id = "detail", title = "Detail", render = function() end,
			on_close = function() closed = closed + 1 end })
	`)
	if err != nil {
		t.Fatal(err)
	}
	panel.Close()
	panel.Close()
	if v := p.State.GetGlobal("closed"); v != lua.LNumber(0) {
		t.Fatalf("on_close ran inside Close: closed = %v", v)
	}
	if len(posted) != 1 {
		t.Fatalf("posted %d callbacks, want 1", len(posted))
	}
	posted[0].Callback()
	if v := p.State.GetGlobal("closed"); v != lua.LNumber(1) {
		t.Fatalf("closed = %v, want 1", v)
	}
}
