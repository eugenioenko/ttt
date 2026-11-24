package app

import (
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"testing"
	"weak"

	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestDebugBindingUsesRequestedIdentityAndWorkspace(t *testing.T) {
	a := &App{Workspace: workspace.New([]string{"/project", "/project/nested"})}
	r := &debugRun{folder: "/project"}
	path := "/project/a.go"
	other := "/project/nested/a.go"
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
