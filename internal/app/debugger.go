package app

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/ui"
)

type debuggerState struct {
	panel       debugSidebar
	run         *debugRun
	points      map[string]map[int]bool
	stoppedFile string
	stoppedLine int
}

func (a *App) initDebuggerGutter() {
	if a.Workspace == nil {
		return
	}
	for _, folder := range a.Workspace.Folders {
		for _, dir := range []string{".ttt", ".vscode"} {
			if _, err := os.Stat(filepath.Join(folder.Path, dir, "launch.json")); err == nil {
				a.ensureDebugPanel()
				a.wireDebuggerGutter(a.EditorGroup)
				return
			}
		}
	}
}

func (a *App) wireDebuggerGutter(g *ui.EditorGroupWidget) {
	g.Editor.OnDebugClick = func(line int) {
		if !g.IsActiveVirtual() && filepath.IsAbs(g.ActiveFilePath()) {
			a.toggleBreakpoint(g.ActiveFilePath(), line+1)
		}
	}
	g.Editor.DebugMarker = func(line int) rune {
		path := g.ActiveFilePath()
		if path == a.debugger.stoppedFile && line+1 == a.debugger.stoppedLine {
			return icons.Rune(icons.ArrowRight)
		}
		if verified, ok := a.debugger.points[path][line+1]; ok {
			if verified {
				return icons.Rune(icons.Breakpoint)
			}
			return icons.Rune(icons.PendingBreakpoint)
		}
		return 0
	}
}

func (a *App) toggleBreakpoint(path string, line int) {
	if a.debugger.points == nil {
		a.debugger.points = make(map[string]map[int]bool)
	}
	points := a.debugger.points[path]
	if points == nil {
		points = make(map[int]bool)
		a.debugger.points[path] = points
	}
	if _, ok := points[line]; ok {
		delete(points, line)
	} else {
		points[line] = false
	}
	if r := a.debugger.run; r != nil {
		r.revision[path]++
		a.syncDebugBreakpoints(path)
	}
	a.refreshDebugPanel()
}

func (a *App) syncDebugBreakpoints(path string) {
	r := a.debugger.run
	if r == nil || !r.ready || r.sending[path] || r.ctx.Err() != nil {
		return
	}
	r.sending[path] = true
	revision := r.revision[path]
	lines := slices.Sorted(maps.Keys(a.debugger.points[path]))
	post := a.tasks.post
	go func() {
		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
		defer cancel()
		points, err := r.session.SetBreakpoints(ctx, path, lines)
		_ = post(func() {
			if a.debugger.run != r || r.ctx.Err() != nil {
				return
			}
			delete(r.sending, path)
			if revision != r.revision[path] {
				a.syncDebugBreakpoints(path)
				return
			}
			if err != nil {
				a.StatusError(err.Error())
				return
			}
			for i, bp := range points {
				if i < len(lines) {
					a.debugger.points[path][lines[i]] = bp.Verified
				}
			}
		})
	}()
}
