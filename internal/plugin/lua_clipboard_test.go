package plugin

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestClipboardReadWriteGranted(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{Commands: true, ClipboardRead: true})
	defer cleanup()

	clip := ""
	p.ClipboardWrite = func(text string) { clip = text }
	p.ClipboardRead = func() string { return clip }

	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.clipboard_write("hello")
		result = ttt.clipboard_read()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got := p.State.GetGlobal("result"); got != lua.LString("hello") {
		t.Errorf("expected 'hello', got %v", got)
	}
}

func TestClipboardReadDenied(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{Commands: true})
	defer cleanup()

	clip := "secret"
	called := false
	p.ClipboardRead = func() string {
		called = true
		return clip
	}

	err := p.State.DoString(`
		local ttt = require("ttt")
		result = ttt.clipboard_read()
	`)
	if err == nil {
		t.Fatal("expected error when clipboard.read permission not granted")
	}
	if !strings.Contains(err.Error(), "clipboard.read permission not granted") {
		t.Fatalf("expected clipboard.read permission error, got: %v", err)
	}
	if called {
		t.Error("ClipboardRead hook must not be called when denied")
	}
	if got := p.State.GetGlobal("result"); got != lua.LNil {
		t.Errorf("expected result to stay nil, got %v", got)
	}
}

func TestClipboardReadNilHook(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{ClipboardRead: true})
	defer cleanup()

	err := p.State.DoString(`
		local ttt = require("ttt")
		result = ttt.clipboard_read()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	got, ok := p.State.GetGlobal("result").(lua.LString)
	if !ok {
		t.Fatalf("expected string, got %T", p.State.GetGlobal("result"))
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
