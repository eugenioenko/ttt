package app

import (
	"testing"

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
