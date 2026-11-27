package plugin

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

func setupTestPluginForEvents(perms PermissionSet) (*Plugin, func()) {
	return newTestPluginBase(perms)
}

func TestEventsOnFileOpen(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{EventsFile: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		_G.opened_path = ""
		events.on("file.open", function(path)
			_G.opened_path = path
		end)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(p.EventListeners["file.open"]) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(p.EventListeners["file.open"]))
	}

	p.DispatchEvent("file.open", lua.LString("/tmp/test.go"))

	if p.State.GetGlobal("opened_path").String() != "/tmp/test.go" {
		t.Errorf("expected '/tmp/test.go', got %q", p.State.GetGlobal("opened_path").String())
	}
}

func TestEventsOnEditorChange(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{EventsEditor: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		_G.change_count = 0
		events.on("editor.change", function()
			_G.change_count = _G.change_count + 1
		end)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	p.DispatchEvent("editor.change")
	p.DispatchEvent("editor.change")

	if p.State.GetGlobal("change_count").String() != "2" {
		t.Errorf("expected 2 changes, got %s", p.State.GetGlobal("change_count").String())
	}
}

func TestEventsFileWithoutPermission(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("file.open", function() end)
	`)
	if err == nil {
		t.Fatal("expected error when events.file not granted")
	}
}

func TestEventsEditorWithoutPermission(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{EventsFile: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("editor.change", function() end)
	`)
	if err == nil {
		t.Fatal("expected error when events.editor not granted")
	}
}

func TestEventsUnknownEvent(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{EventsFile: true, EventsEditor: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("unknown.event", function() end)
	`)
	if err == nil {
		t.Fatal("expected error for unknown event")
	}
}

func TestEventsKeyPressConsumed(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{Keybindings: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("key.press", function(ev)
			if ev.key == "j" then
				_G.intercepted = "j"
				return true
			end
			return false
		end)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	ev := tcell.NewEventKey(tcell.KeyRune, "j", tcell.ModNone)
	tbl := keyEventToLua(p.State, ev, p.ControlModifiers)
	if !p.DispatchKeyEvent(tbl) {
		t.Fatal("expected key.press to be consumed")
	}
	if p.State.GetGlobal("intercepted").String() != "j" {
		t.Errorf("expected 'j', got %q", p.State.GetGlobal("intercepted").String())
	}
}

func TestEventsKeyPressNotConsumed(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{Keybindings: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("key.press", function(ev)
			return false
		end)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	ev := tcell.NewEventKey(tcell.KeyRune, "k", tcell.ModNone)
	tbl := keyEventToLua(p.State, ev, p.ControlModifiers)
	if p.DispatchKeyEvent(tbl) {
		t.Fatal("expected key.press not to be consumed")
	}
}

func TestEventsKeyPressModifierPreference(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{Keybindings: true})
	defer cleanup()
	if err := p.State.DoString(`
		require("ttt.events").on("key.press", function(ev)
			return ev.key == "Ctrl-R" and ev.mod == "ctrl"
		end)
	`); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{plugins: []*Plugin{p}}
	mask := tcell.ModCtrl
	p.ControlModifiers = &mask
	super := tcell.NewEventKey(tcell.KeyRune, "r", tcell.ModMeta)
	for _, mode := range []tcell.ModMask{tcell.ModCtrl, tcell.ModMeta, tcell.ModCtrl | tcell.ModMeta} {
		mask = mode
		if got := manager.DispatchKeyEvent(super); got != (mode&tcell.ModMeta != 0) {
			t.Fatalf("Super+R consumed = %v in mode %v", got, mode)
		}
		if !manager.DispatchKeyEvent(tcell.NewEventKey(tcell.KeyCtrlR, "", tcell.ModCtrl)) {
			t.Fatalf("physical Ctrl+R must still reach the hook in mode %v", mode)
		}
		if super.Key() != tcell.KeyRune || super.Str() != "r" || super.Modifiers() != tcell.ModMeta {
			t.Fatal("Lua conversion changed the native event")
		}
	}
}

func TestEventsKeyPressWithModifiers(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{Keybindings: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("key.press", function(ev)
			_G.got_key = ev.key
			_G.got_mod = ev.mod or ""
			return true
		end)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	ev := tcell.NewEventKey(tcell.KeyRune, "w", tcell.ModAlt)
	tbl := keyEventToLua(p.State, ev, p.ControlModifiers)
	p.DispatchKeyEvent(tbl)

	if p.State.GetGlobal("got_key").String() != "w" {
		t.Errorf("expected 'w', got %q", p.State.GetGlobal("got_key").String())
	}
	if p.State.GetGlobal("got_mod").String() != "alt" {
		t.Errorf("expected 'alt', got %q", p.State.GetGlobal("got_mod").String())
	}
}

func TestEventsKeyPressSpecialKey(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{Keybindings: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("key.press", function(ev)
			_G.got_key = ev.key
			return true
		end)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	ev := tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone)
	tbl := keyEventToLua(p.State, ev, p.ControlModifiers)
	p.DispatchKeyEvent(tbl)

	if p.State.GetGlobal("got_key").String() != "Esc" {
		t.Errorf("expected 'Esc', got %q", p.State.GetGlobal("got_key").String())
	}
}

func TestEventsKeyPressWithoutPermission(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("key.press", function() return true end)
	`)
	if err == nil {
		t.Fatal("expected error when events.keys not granted")
	}
}

func TestEventsMultipleListeners(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{EventsFile: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		_G.log = ""
		events.on("file.save", function(path)
			_G.log = _G.log .. "A:" .. path .. ";"
		end)
		events.on("file.save", function(path)
			_G.log = _G.log .. "B:" .. path .. ";"
		end)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	p.DispatchEvent("file.save", lua.LString("test.go"))

	expected := "A:test.go;B:test.go;"
	if p.State.GetGlobal("log").String() != expected {
		t.Errorf("expected %q, got %q", expected, p.State.GetGlobal("log").String())
	}
}

func TestEventsGutterClick(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{EditorBookmarks: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("gutter.click", function(path, line)
			_G.clicked = path .. ":" .. line
		end)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	m := &Manager{plugins: []*Plugin{p}}
	if !m.HasEventListeners("gutter.click") {
		t.Fatal("HasEventListeners(gutter.click) = false, want true")
	}
	if m.HasEventListeners("bookmark.change") {
		t.Fatal("HasEventListeners(bookmark.change) = true with no listener")
	}

	m.DispatchEvent("gutter.click", "/tmp/a.go", 3)
	if got := p.State.GetGlobal("clicked").String(); got != "/tmp/a.go:3" {
		t.Errorf("clicked = %q, want /tmp/a.go:3", got)
	}
}

func TestEventsGutterClickWithoutPermission(t *testing.T) {
	p, cleanup := setupTestPluginForEvents(PermissionSet{EventsEditor: true})
	defer cleanup()

	err := p.State.DoString(`
		local events = require("ttt.events")
		events.on("gutter.click", function() end)
	`)
	if err == nil {
		t.Fatal("expected error when editor.bookmarks not granted")
	}
}
