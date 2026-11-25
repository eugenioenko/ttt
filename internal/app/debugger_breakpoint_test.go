package app

import (
	"os"
	"path/filepath"
	"testing"
	"weak"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/config/debugadapter"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestDebugBindingUsesRequestedIdentityAndWorkspace(t *testing.T) {
	a := &App{Workspace: workspace.New([]string{"/project", "/project/nested"})}
	r := &debugRun{languages: []string{"go"}, folder: "/project"}
	path := "/project/a.go"
	other := "/project/nested/a.go"
	if a.debugOwnsSource(r, "/project/module.py") {
		t.Fatal("Go session accepted Python breakpoints")
	}
	a.toggleBreakpoint(path, 3)
	a.toggleBreakpoint(other, 3)
	id := 0
	bp := dap.Breakpoint{ID: &id, Line: 5, Verified: true}
	a.applyDebugBindings(path, []int{3}, []dap.Breakpoint{bp})
	a.applyDebugBindings(other, []int{3}, []dap.Breakpoint{bp})
	bp.Message = "unavailable"
	a.debugBindingEvent(r, bp, "removed")
	p := a.debugger.points[path][3]
	if p.binding.Verified || p.binding.Line != 5 || p.binding.Message != "unavailable" || !a.debugger.points[other][3].binding.Verified {
		t.Fatal("relocated breakpoint event lost identity or crossed workspace boundary")
	}
	a.toggleBreakpoint(path, 3)
	a.applyDebugBindings(path, []int{3}, []dap.Breakpoint{bp})
	if len(a.debugger.points[path]) != 0 {
		t.Fatal("late reply restored deleted breakpoint")
	}
}

func TestDebugSourceRebasesOnceAndRequiresConfirmationOnReplacement(t *testing.T) {
	a := &App{}
	path := "/project/a.go"
	b := &buffer.Buffer{}
	b.SetText("one\ntwo\nthree")
	a.toggleBreakpoint(path, 2)
	a.debugger.sources = map[string]debugSourceVersion{path: {weak.Make(b), b.Version()}}
	p := a.debugger.points[path][2]
	b.Splice(0, 0, "zero")
	a.updateDebugSource(path, b)
	a.updateDebugSource(path, b)
	if a.debugger.points[path][3] != p || p.confirm {
		t.Fatal("split observers did not preserve the requested line")
	}
	b.Splice(0, 1)
	a.updateDebugSource(path, b)
	if a.debugger.points[path][2] != p {
		t.Fatal("undo did not restore original line")
	}
	b.SetText("replacement")
	a.updateDebugSource(path, b)
	if len(a.debugLines(&debugRun{}, path)) != 0 || !p.confirm {
		t.Fatal("replacement automatically bound ambiguous source")
	}
	a.toggleBreakpoint(path, 1)
	if p.confirm || len(a.debugLines(&debugRun{}, path)) != 1 {
		t.Fatal("could not confirm new location")
	}
}

func TestClearBreakpointsAcrossFoldersDuringStartup(t *testing.T) {
	a := &App{Workspace: workspace.New([]string{"/project", "/project/nested"})}
	first, second, other := "/project/a.go", "/project/b.go", "/project/nested/c.go"
	for _, path := range []string{first, second, other} {
		a.toggleBreakpoint(path, 1)
	}
	a.debugger.points[other][1].confirm = true
	r := &debugRun{languages: []string{"go"}, folder: "/project", revision: make(map[string]int)}
	a.debugger.run = r
	a.ClearDebugBreakpoints()
	if len(a.debugger.points) != 0 || len(r.revision) != 2 || r.revision[first] != 1 || r.revision[second] != 1 {
		t.Fatalf("points after clear = %v, pending adapter updates = %v", a.debugger.points, r.revision)
	}
}

func TestDebugEditPreservesRuntimeLocationAndClampsDeletedLastLine(t *testing.T) {
	path := "/project/main.go"
	a := &App{Workspace: workspace.New([]string{"/project"})}
	b := &buffer.Buffer{}
	b.SetText("one\ntwo\nthree")
	a.toggleBreakpoint(path, 1)
	a.toggleBreakpoint(path, 3)
	a.debugger.sources = map[string]debugSourceVersion{path: {weak.Make(b), b.Version()}}
	r := &debugRun{languages: []string{"go"}, folder: "/project", edited: map[string]bool{}, revision: map[string]int{}}
	a.debugger.run = r
	for n, p := range a.debugger.points[path] {
		p.runtimeLine = n
		p.binding.Verified = true
	}
	b.Splice(0, 0, "new")
	a.updateDebugSource(path, b)
	if p := a.debugger.points[path][2]; p == nil || !p.binding.Verified || p.runtimeLine != 1 {
		t.Fatal("lost unchanged breakpoint")
	}
	b.Splice(3, 1)
	a.updateDebugSource(path, b)
	if p := a.debugger.points[path][3]; p == nil || !p.confirm {
		t.Fatal("deleted last line cannot be confirmed")
	}
	if lines := a.debugLines(r, path); len(lines) != 1 || lines[0] != 1 {
		t.Fatalf("runtime breakpoints %v", lines)
	}
}

func TestGutterListenerDuringDebugSession(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Debug.Adapters = map[string]debugadapter.Config{"go": {Languages: []string{"go"}}}
	a := buildTestApp(t, settings)
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	a.Sidebar.ActivePanel = "explorer"
	a.Sidebar.Visible = true
	a.wireDebuggerGutter(a.EditorGroup)
	listener := &plugin.Plugin{Name: "listener", Enabled: true, Granted: plugin.PermissionSet{EditorBookmarks: true}}
	if err := listener.InitFromSource(`clicks = 0
		require("ttt.events").on("gutter.click", function() clicks = clicks + 1 end)`); err != nil {
		t.Fatal(err)
	}
	defer listener.Destroy()
	a.PluginManager = plugin.NewManager(dir, filepath.Join(dir, "plugins.json"))
	a.PluginManager.RegisterDebugPlugin(listener)
	click := func(wantBreakpoint bool, wantClicks string) {
		t.Helper()
		if !a.EditorGroup.OnGutterClick(path, 0) {
			t.Fatal("unhandled gutter click")
		}
		if got := a.debugger.points[path][1] != nil; got != wantBreakpoint {
			t.Fatalf("breakpoint = %v, want %v", got, wantBreakpoint)
		}
		if got := listener.State.GetGlobal("clicks").String(); got != wantClicks {
			t.Fatalf("plugin received %s clicks, want %s", got, wantClicks)
		}
	}
	click(false, "1")
	a.debugger.run = &debugRun{}
	click(true, "1")
	thread := 1
	a.debugger.run.thread = &thread
	click(false, "1")
	a.debugger.run = nil
	click(false, "2")
}
