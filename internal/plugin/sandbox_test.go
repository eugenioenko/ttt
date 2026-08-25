package plugin

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestManagerInstallRejectsNonHTTPS(t *testing.T) {
	m := NewManager(t.TempDir(), filepath.Join(t.TempDir(), "registry.json"))

	tests := []struct {
		url     string
		blocked bool
	}{
		{"https://github.com/user/plugin.git", false},
		{"http://github.com/user/plugin.git", true},
		{"file:///tmp/evil", true},
		{"git@github.com:user/plugin.git", true},
		{"ssh://git@github.com/user/plugin.git", true},
	}

	for _, tt := range tests {
		_, err := m.Install(tt.url, "")
		if tt.blocked && err == nil {
			t.Errorf("expected %q to be blocked", tt.url)
		}
		if !tt.blocked && err != nil && !strings.Contains(err.Error(), "git clone") {
			t.Errorf("expected %q to pass URL validation, got: %v", tt.url, err)
		}
	}
}

func TestPluginInitPathTraversalBlocked(t *testing.T) {
	dir := t.TempDir()

	p := &Plugin{
		Name: "evil",
		Dir:  dir,
		Manifest: Manifest{
			Name:  "evil",
			Entry: "../../etc/passwd",
		},
	}

	err := p.Init()
	if err == nil {
		t.Fatal("expected error for path traversal in entry field")
	}
	if p.State != nil {
		t.Error("expected State to be nil after failed init")
	}
}

func TestSandboxDangerousGlobalsRemoved(t *testing.T) {
	L := NewSandbox()
	defer L.Close()

	for _, name := range []string{
		"dofile", "loadfile", "load", "loadstring",
		"getfenv", "setfenv", "rawset", "rawget", "print",
	} {
		v := L.GetGlobal(name)
		if v != lua.LNil {
			t.Errorf("expected %s to be nil, got %s", name, v.Type().String())
		}
	}
}

func TestSandboxLoadCannotCompileCode(t *testing.T) {
	L := NewSandbox()
	defer L.Close()

	if err := L.DoString(`load("return 1+1")()`); err == nil {
		t.Error("load() should not be available to compile arbitrary code")
	}

	if err := L.DoString(`loadstring("return 1+1")()`); err == nil {
		t.Error("loadstring() should not be available to compile arbitrary code")
	}
}

func TestSandboxPackageLoadersRestricted(t *testing.T) {
	L := NewSandbox()
	defer L.Close()

	err := L.DoString(`
		local loaders = package.loaders
		if #loaders ~= 1 then
			error("expected exactly 1 loader (preload), got " .. #loaders)
		end
	`)
	if err != nil {
		t.Fatalf("package.loaders check failed: %v", err)
	}

	err = L.DoString(`
		local loader = package.loaders[2]
		if loader ~= nil then
			error("filesystem loader should not be present")
		end
	`)
	if err != nil {
		t.Fatalf("filesystem loader check failed: %v", err)
	}
}

func TestSandboxSafeModulesWork(t *testing.T) {
	L := NewSandbox()
	defer L.Close()

	if err := L.DoString(`local x = string.format("hello %s", "world")`); err != nil {
		t.Errorf("string.format should work: %v", err)
	}
	if err := L.DoString(`local t = {}; table.insert(t, 1)`); err != nil {
		t.Errorf("table.insert should work: %v", err)
	}
	if err := L.DoString(`local x = math.floor(3.14)`); err != nil {
		t.Errorf("math.floor should work: %v", err)
	}
}

func TestSandboxRequireRestricted(t *testing.T) {
	L := NewSandbox()
	defer L.Close()

	p := &Plugin{Granted: PermissionSet{PanelSidebar: true}}
	p.EventListeners = make(map[string][]*lua.LFunction)
	setupTTTModule(L, p)
	setupEditorModule(L, p)
	setupFsModule(L, p)
	setupSystemModule(L, p)
	setupNetModule(L, p)
	setupEventsModule(L, p)

	if err := L.DoString(`local ttt = require("ttt")`); err != nil {
		t.Errorf("require ttt should work: %v", err)
	}

	for _, mod := range []string{"ttt.editor", "ttt.fs", "ttt.system", "ttt.net", "ttt.events"} {
		if err := L.DoString(`local m = require("` + mod + `")`); err != nil {
			t.Errorf("require %s should work: %v", mod, err)
		}
	}

	if err := L.DoString(`require("os")`); err == nil {
		t.Error("require os should fail")
	}

	if err := L.DoString(`require("io")`); err == nil {
		t.Error("require io should fail")
	}
}

func TestSandboxRegisterSidebar(t *testing.T) {
	L := NewSandbox()
	defer L.Close()

	p := &Plugin{Granted: PermissionSet{PanelSidebar: true}}
	setupTTTModule(L, p)

	err := L.DoString(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Test Panel",
				render = function(panel) end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if p.SidebarTitle != "Test Panel" {
		t.Errorf("expected title 'Test Panel', got %q", p.SidebarTitle)
	}
	if p.RenderFunc == nil {
		t.Error("expected render function to be set")
	}
}

func TestSandboxRegisterWithoutPermission(t *testing.T) {
	L := NewSandbox()
	defer L.Close()

	p := &Plugin{Granted: PermissionSet{}}
	setupTTTModule(L, p)

	err := L.DoString(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Test Panel",
				render = function(panel) end,
			},
		})
	`)
	if err == nil {
		t.Fatal("expected error when panel.sidebar not granted")
	}

	if p.SidebarTitle != "" {
		t.Error("title should not be set without permission")
	}
}

func TestSandboxRegisterBottom(t *testing.T) {
	L := NewSandbox()
	defer L.Close()

	p := &Plugin{Granted: PermissionSet{PanelBottom: true}}
	setupTTTModule(L, p)

	err := L.DoString(`
		local ttt = require("ttt")
		ttt.register({
			bottom = {
				title = "Output",
				render = function(panel) end,
				on_event = function(ev) end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register bottom failed: %v", err)
	}

	if p.BottomTitle != "Output" {
		t.Errorf("expected title 'Output', got %q", p.BottomTitle)
	}
	if p.BottomRenderFunc == nil {
		t.Error("expected bottom render function to be set")
	}
	if p.BottomEventFunc == nil {
		t.Error("expected bottom event function to be set")
	}
}

func TestSandboxRegisterBottomWithoutPermission(t *testing.T) {
	L := NewSandbox()
	defer L.Close()

	p := &Plugin{Granted: PermissionSet{PanelSidebar: true}}
	setupTTTModule(L, p)

	err := L.DoString(`
		local ttt = require("ttt")
		ttt.register({
			bottom = {
				title = "Output",
				render = function(panel) end,
			},
		})
	`)
	if err == nil {
		t.Fatal("expected error when panel.bottom not granted")
	}

	if p.BottomTitle != "" {
		t.Error("bottom title should not be set without permission")
	}
}

func TestPluginDirExposed(t *testing.T) {
	p := &Plugin{Name: "test", Dir: "/some/plugin/dir"}
	p.State = NewSandbox()
	defer p.State.Close()
	setupTTTModule(p.State, p)

	err := p.State.DoString(`
		local ttt = require("ttt")
		result = ttt.plugin_dir()
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.State.GetGlobal("result").String()
	if got != "/some/plugin/dir" {
		t.Errorf("expected /some/plugin/dir, got %q", got)
	}
}

func TestPlatformArchVersionExposed(t *testing.T) {
	p := &Plugin{Name: "test", AppVersion: "1.2.3"}
	p.State = NewSandbox()
	defer p.State.Close()
	setupTTTModule(p.State, p)

	err := p.State.DoString(`
		local ttt = require("ttt")
		platform = ttt.platform()
		arch = ttt.arch()
		version = ttt.version()
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := p.State.GetGlobal("platform").String(); got != runtime.GOOS {
		t.Errorf("expected platform %q, got %q", runtime.GOOS, got)
	}
	if got := p.State.GetGlobal("arch").String(); got != runtime.GOARCH {
		t.Errorf("expected arch %q, got %q", runtime.GOARCH, got)
	}
	if got := p.State.GetGlobal("version").String(); got != "1.2.3" {
		t.Errorf("expected version %q, got %q", "1.2.3", got)
	}
}
