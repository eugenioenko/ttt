package plugin

import (
	"fmt"
	"slices"
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
	p.FocusTab = func(key string, focus bool) bool {
		if _, ok := open[key]; ok {
			focused = append(focused, fmt.Sprintf("%s:%t", key, focus))
			return true
		}
		return false
	}
	p.CloseTab = func(key string) bool {
		_, ok := open[key]
		delete(open, key)
		return ok
	}

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
	if want := []string{"plugin:test:ns:true", "plugin:test:ns:true", "Legacy:true"}; !slices.Equal(focused, want) {
		t.Fatalf("each open should focus its tab, focused = %v, want %v", focused, want)
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

func TestCloseTabPrefersTheIDOverATitle(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelEditor: true})
	defer done()
	open := map[string]bool{}
	p.OpenTab = func(key, _ string, _ *PluginPanelWidget) { open[key] = true }
	p.CloseTab = func(key string) bool {
		ok := open[key]
		delete(open, key)
		return ok
	}
	err := p.State.DoString(`
		local ttt = require("ttt")
		local r = function(panel) end
		ttt.open_tab({ title = "ns", render = r })
		ttt.open_tab({ id = "ns", title = "Namespaces", render = r })
		ttt.close_tab("ns")
	`)
	if err != nil {
		t.Fatal(err)
	}
	if open["plugin:test:ns"] || !open["ns"] {
		t.Fatalf("close_tab should close only the tab with that id, open = %v", open)
	}
	if err := p.State.DoString(`require("ttt").close_tab("ns")`); err != nil {
		t.Fatal(err)
	}
	if open["ns"] {
		t.Fatal("with no tab for the id, close_tab should fall back to the title")
	}
}

func TestOpenTabFocusFalseAndFocusTab(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelEditor: true})
	defer done()
	open := map[string]bool{}
	var calls []string
	p.OpenTab = func(key, _ string, _ *PluginPanelWidget) { open[key] = true }
	p.FocusTab = func(key string, focus bool) bool {
		calls = append(calls, fmt.Sprintf("%s:%t", key, focus))
		return open[key]
	}

	err := p.State.DoString(`
		local ttt = require("ttt")
		local r = function(panel) end
		ttt.open_tab({ id = "q", title = "Quiet", focus = false, render = r })
		ttt.open_tab({ id = "q", title = "Quiet", focus = false, render = r })
		found = ttt.focus_tab("q")
		missing = ttt.focus_tab("nope")
	`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"plugin:test:q:false",
		"plugin:test:q:false",
		"plugin:test:q:true",
		"plugin:test:nope:true",
		"nope:true",
	}
	if !slices.Equal(calls, want) {
		t.Fatalf("FocusTab calls = %v, want %v", calls, want)
	}
	if p.State.GetGlobal("found") != lua.LTrue || p.State.GetGlobal("missing") != lua.LFalse {
		t.Fatalf("focus_tab returned %v and %v", p.State.GetGlobal("found"), p.State.GetGlobal("missing"))
	}
}

func TestFocusTabAllowedForFileHandlers(t *testing.T) {
	for _, tt := range []struct {
		perms PermissionSet
		ok    bool
	}{
		{PermissionSet{PanelEditor: true}, true},
		{PermissionSet{FileHandlers: true}, true},
		{PermissionSet{}, false},
	} {
		p, done := newTestPluginBase(tt.perms)
		p.FocusTab = func(key string, _ bool) bool { return key == "plugin:test:/tmp/a.pdf" }
		err := p.State.DoString(`found = require("ttt").focus_tab("/tmp/a.pdf")`)
		if (err == nil) != tt.ok {
			t.Errorf("%+v: err = %v, want ok %v", tt.perms, err, tt.ok)
		}
		if tt.ok && p.State.GetGlobal("found") != lua.LTrue {
			t.Errorf("%+v: focus_tab did not find the tab", tt.perms)
		}
		done()
	}
}
