package plugin

import (
	"errors"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestInputAPIsRequireInputPermission(t *testing.T) {
	for _, call := range []string{"ttt.click(1, 2)", "ttt.drag(1, 2, 3, 4)"} {
		p, cleanup := newTestPluginBase(PermissionSet{})
		called := false
		p.SimulateClick = func(int, int) { called = true }
		p.SimulateDrag = func(int, int, int, int) { called = true }
		err := p.State.DoString(`local ttt = require("ttt")` + "\n" + call)
		cleanup()
		if err == nil || !strings.Contains(err.Error(), "input permission not granted") {
			t.Fatalf("%s error = %v, want input permission error", call, err)
		}
		if called {
			t.Fatalf("%s reached the input hook without permission", call)
		}
	}
}

func TestInputAPIsWithPermission(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{Input: true})
	defer cleanup()
	var clicks, drags int
	p.SimulateClick = func(x, y int) {
		if x == 1 && y == 2 {
			clicks++
		}
	}
	p.SimulateDrag = func(x1, y1, x2, y2 int) {
		if x1 == 1 && y2 == 4 {
			drags++
		}
	}
	if err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.click(1, 2)
		ttt.drag(1, 2, 3, 4)
	`); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 || drags != 1 {
		t.Fatalf("clicks = %d, drags = %d", clicks, drags)
	}
}

func TestScreenCaptureRequiresScreenRead(t *testing.T) {
	for _, call := range []string{"ttt.screenshot()", "ttt.debug()", "ttt.screenshot('/tmp/x')"} {
		p, cleanup := newTestPluginBase(PermissionSet{FsWrite: FsAccess{Paths: []string{FsAnyPath}}})
		fs := &mockFilesystemAPI{}
		p.Filesystem = fs
		p.ScreenText = func() string { return "screen" }
		p.DebugState = func() (string, error) { return "{}", nil }
		err := p.State.DoString(`local ttt = require("ttt")` + "\n" + call)
		cleanup()
		if err == nil || !strings.Contains(err.Error(), "screen.read permission not granted") {
			t.Fatalf("%s error = %v, want screen.read permission error", call, err)
		}
		if len(fs.written) != 0 {
			t.Fatalf("%s wrote %v without permission", call, fs.written)
		}
	}
}

func TestScreenCaptureReturnsText(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{ScreenRead: true, EditorRead: true, TerminalRead: true})
	defer cleanup()
	p.ScreenText = func() string { return "screen" }
	p.DebugState = func() (string, error) { return `{"ok":true}`, nil }
	if err := p.State.DoString(`
		local ttt = require("ttt")
		shot = ttt.screenshot()
		state = ttt.debug()
	`); err != nil {
		t.Fatal(err)
	}
	if got := p.State.GetGlobal("shot"); got != lua.LString("screen") {
		t.Errorf("screenshot = %v", got)
	}
	if got := p.State.GetGlobal("state"); got != lua.LString(`{"ok":true}`) {
		t.Errorf("debug = %v", got)
	}
}

func TestDebugNeedsEditorAndTerminalRead(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{ScreenRead: true, EditorRead: true})
	defer cleanup()
	p.DebugState = func() (string, error) { return "{}", nil }
	err := p.State.DoString(`local ttt = require("ttt")
		ttt.debug()`)
	if err == nil || !strings.Contains(err.Error(), "terminal.read permission not granted") {
		t.Fatalf("debug error = %v", err)
	}
}

func TestScreenCaptureWritesThroughPluginFilesystem(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{ScreenRead: true, EditorRead: true, TerminalRead: true, FsWrite: FsAccess{Paths: []string{FsAnyPath}}})
	defer cleanup()
	fs := &mockFilesystemAPI{}
	p.Filesystem = fs
	p.ScreenText = func() string { return "screen" }
	p.DebugState = func() (string, error) { return "{}", nil }
	if err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.screenshot("/tmp/shot.txt")
		ttt.debug("/tmp/state.json")
	`); err != nil {
		t.Fatal(err)
	}
	if fs.written["/tmp/shot.txt"] != "screen" || fs.written["/tmp/state.json"] != "{}" {
		t.Fatalf("written = %v", fs.written)
	}
}

func TestKeyAndTypeRequireInputPermission(t *testing.T) {
	for _, call := range []string{`ttt.key("ctrl+s")`, `ttt.type("hi")`} {
		p, cleanup := newTestPluginBase(PermissionSet{})
		called := false
		p.SimulateKeys = func(string) error { called = true; return nil }
		p.SimulateText = func(string) error { called = true; return nil }
		err := p.State.DoString(`local ttt = require("ttt")` + "\n" + call)
		cleanup()
		if err == nil || !strings.Contains(err.Error(), "input permission not granted") {
			t.Fatalf("%s error = %v, want input permission error", call, err)
		}
		if called {
			t.Fatalf("%s reached the input hook without permission", call)
		}
	}
}

func TestKeyAndTypeWithPermission(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{Input: true})
	defer cleanup()
	var keys, text []string
	p.SimulateKeys = func(combo string) error {
		if combo == "nope" {
			return errors.New("unknown key")
		}
		keys = append(keys, combo)
		return nil
	}
	p.SimulateText = func(s string) error {
		if s == "too long" {
			return errors.New("type input exceeds limit")
		}
		text = append(text, s)
		return nil
	}
	if err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.key("ctrl+k ctrl+s")
		ttt.type("héllo")
		ok, err = pcall(ttt.key, "nope")
		type_ok, type_err = pcall(ttt.type, "too long")
	`); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "ctrl+k ctrl+s" || len(text) != 1 || text[0] != "héllo" {
		t.Fatalf("keys = %q, text = %q", keys, text)
	}
	if p.State.GetGlobal("ok") != lua.LFalse || !strings.Contains(p.State.GetGlobal("err").String(), "unknown key") {
		t.Fatalf("invalid combo: ok = %v, err = %v", p.State.GetGlobal("ok"), p.State.GetGlobal("err"))
	}
	if p.State.GetGlobal("type_ok") != lua.LFalse || !strings.Contains(p.State.GetGlobal("type_err").String(), "type input exceeds") {
		t.Fatalf("invalid type: ok = %v, err = %v", p.State.GetGlobal("type_ok"), p.State.GetGlobal("type_err"))
	}
}
