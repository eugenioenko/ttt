package app

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/ui"
)

type debuggerState struct {
	lastFolder, lastTarget string
	panel                  debugSidebar
	run                    *debugRun
	points                 map[string]map[int]*debugBreakpoint
	stoppedFile            string
	stoppedLine            int
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
		if point, ok := a.debugger.points[path][line+1]; ok {
			if point.binding.Verified {
				return icons.Rune(icons.Breakpoint)
			}
			return icons.Rune(icons.PendingBreakpoint)
		}
		return 0
	}
}

func (a *App) toggleBreakpoint(path string, line int) {
	if a.debugger.points == nil {
		a.debugger.points = make(map[string]map[int]*debugBreakpoint)
	}
	points := a.debugger.points[path]
	if points == nil {
		points = make(map[int]*debugBreakpoint)
		a.debugger.points[path] = points
	}
	if _, ok := points[line]; ok {
		delete(points, line)
	} else {
		points[line] = &debugBreakpoint{}
	}
	if r := a.debugger.run; r != nil && a.debugOwnsSource(r, path) {
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
			a.applyDebugBindings(path, lines, points)
		})
	}()
}

func (a *App) ToggleDebugBreakpoint() {
	g := a.EditorGroup
	if g.ActiveBuffer() == nil || g.IsActiveVirtual() || !filepath.IsAbs(g.ActiveFilePath()) {
		return
	}
	for _, group := range a.editorGroups() {
		a.wireDebuggerGutter(group)
	}
	a.toggleBreakpoint(g.ActiveFilePath(), g.Editor.Cursor.Line+1)
}

type debugBreakpoint struct {
	binding dap.Breakpoint
}

func (a *App) debugOwnsSource(r *debugRun, path string) bool {
	folder := a.Workspace.FolderForFile(path)
	return folder != nil && folder.Path == r.folder
}

func (a *App) applyDebugBindings(path string, lines []int, bindings []dap.Breakpoint) {
	for i, line := range lines {
		if p := a.debugger.points[path][line]; p != nil {
			p.binding = dap.Breakpoint{}
			if i < len(bindings) {
				p.binding = bindings[i]
			}
		}
	}
	a.refreshDebugPanel()
}

func (a *App) debugBindingEvent(r *debugRun, b dap.Breakpoint, reason string) {
	var match *debugBreakpoint
	for path, points := range a.debugger.points {
		if !a.debugOwnsSource(r, path) {
			continue
		}
		for line, p := range points {
			if b.ID != nil && p.binding.ID != nil {
				if *p.binding.ID == *b.ID {
					match = p
					break
				}
			} else if path == b.Source.Path && line == b.Line {
				match = p
			}
		}
		if match != nil {
			break
		}
	}
	if match == nil {
		return
	}
	if reason == "removed" {
		b.Verified = false
	}
	match.binding = b
	a.refreshDebugPanel()
}
