package app

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"weak"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/icons"
)

type debugBinding struct {
	ID       *int
	Verified bool
	Line     int
	Source   struct{ Path string }
	Message  string
}
type debugBreakpoint struct {
	runtimeLine int
	confirm     bool
}
type debugSourceVersion struct {
	buf     weak.Pointer[buffer.Buffer]
	version uint64
}

func (a *App) trackDebugSource(path string, b *buffer.Buffer) {
	if a.debugger.sources == nil {
		a.debugger.sources = make(map[string]debugSourceVersion)
	}
	a.debugger.sources[path] = debugSourceVersion{weak.Make(b), b.Version()}
}

func (a *App) bindingFor(path string, p *debugBreakpoint) debugBinding {
	if peer := a.activeDebugPeer(); peer != nil {
		return peer.bindings[path][p.runtimeLine]
	}
	return debugBinding{}
}

func (a *App) breakpointIcon(path string, p *debugBreakpoint) icons.Name {
	if p.confirm || a.debugger.run != nil && !a.bindingFor(path, p).Verified {
		return icons.PendingBreakpoint
	}
	return icons.Breakpoint
}

func (a *App) debugBindingEvent(peer *debugRun, reason string, b debugBinding) {
	var fallback map[int]debugBinding
	var fallbackLine int
	if reason == "removed" {
		b.Verified = false
	}
	for path, lines := range peer.bindings {
		for line, old := range lines {
			if old.ID != nil && b.ID != nil {
				if *old.ID == *b.ID {
					lines[line] = b
					a.refreshDebugPanel()
					return
				}
				continue
			}
			if path == b.Source.Path && line == b.Line {
				fallback = lines
				fallbackLine = line
			}
		}
	}
	if fallback != nil {
		if b.ID == nil {
			b.ID = fallback[fallbackLine].ID
		}
		fallback[fallbackLine] = b
		a.refreshDebugPanel()
	}
}

func (a *App) restoreBreakpoints() {
	if a.Workspace == nil {
		return
	}
	var roots []string
	for _, f := range a.Workspace.Folders {
		roots = append(roots, f.Path)
	}
	slices.Sort(roots)
	key := a.Workspace.FilePath
	if key == "" && len(roots) > 0 {
		data, _ := json.Marshal(roots)
		key = string(data)
	}
	if key == a.debugger.workspaceKey {
		return
	}
	a.debugger.workspaceKey = key
	a.debugger.points = make(map[string]map[int]*debugBreakpoint)
	a.debugger.sources = nil
	a.debugger.closedSources = nil
	for path, lines := range a.State.Breakpoints[key] {
		points := make(map[int]*debugBreakpoint)
		for _, saved := range lines {
			if saved.Line > 0 {
				points[saved.Line] = &debugBreakpoint{runtimeLine: saved.Line, confirm: saved.NeedsConfirmation}
			}
		}
		a.debugger.points[path] = points
		if a.EditorGroup != nil {
			if buf := a.editorBufferForPath(path); buf != nil {
				a.trackDebugSource(path, buf)
			}
		}
	}
	a.refreshDebugPanel()
}

func (a *App) persistBreakpoints() {
	key := a.debugger.workspaceKey
	if key == "" {
		return
	}
	state, err := config.ReadState()
	if err != nil {
		a.StatusError(err.Error())
		return
	}
	if state.Breakpoints == nil {
		state.Breakpoints = make(map[string]map[string][]config.SavedBreakpoint)
	}
	snapshot := make(map[string][]config.SavedBreakpoint)
	for path, points := range a.debugger.points {
		snapshot[path] = savedDebugPoints(points)
	}
	if len(snapshot) == 0 {
		delete(state.Breakpoints, key)
	} else {
		state.Breakpoints[key] = snapshot
	}
	a.State.Breakpoints = state.Breakpoints
	if err := config.SaveState(state); err != nil {
		a.StatusError(err.Error())
	}
}

func savedDebugPoints(points map[int]*debugBreakpoint) []config.SavedBreakpoint {
	saved := make([]config.SavedBreakpoint, 0, len(points))
	for _, line := range slices.Sorted(maps.Keys(points)) {
		saved = append(saved, config.SavedBreakpoint{Line: line, NeedsConfirmation: points[line].confirm})
	}
	return saved
}

func (a *App) updateDebugSource(path string, b *buffer.Buffer) {
	prev, ok := a.debugger.sources[path]
	if b == nil || len(a.debugger.points[path]) == 0 {
		return
	}
	if !ok {
		a.trackDebugSource(path, b)
		return
	}
	now := debugSourceVersion{weak.Make(b), b.Version()}
	if now == prev {
		return
	}
	points := a.debugger.points[path]
	previousLines := savedDebugPoints(points)
	resync := false
	replayed := prev.buf == now.buf && b.ChangesSince(prev.version, func(c buffer.Change) {
		shifted := make(map[int]*debugBreakpoint, len(points))
		for _, line := range slices.Sorted(maps.Keys(points)) {
			p := points[line]
			switch {
			case line <= c.Start:
			case line > c.Start+c.Removed:
				line += c.Added - c.Removed
			default:
				if p.runtimeLine != 0 {
					resync = true
				}
				p.runtimeLine = 0

				p.confirm = true
				line = c.Start + max(1, min(line-c.Start, c.Added))
			}
			if existing := shifted[line]; existing == nil || existing.confirm {
				shifted[line] = p
			}
		}
		points = shifted
	})
	for line, p := range points {
		if !replayed {
			p.confirm = true
			p.runtimeLine = 0

			resync = true
		}
		if line > len(b.Lines) {
			delete(points, line)
			last := max(1, len(b.Lines))
			if points[last] == nil {
				points[last] = p
			}
		}
	}
	a.debugger.points[path] = points
	a.debugger.sources[path] = now
	if r := a.debugger.run; r != nil {
		a.debugger.edited[path] = true
		if resync {
			a.queueDebugPoints(r)
		}
	}
	if !slices.Equal(previousLines, savedDebugPoints(points)) {
		a.persistBreakpoints()
	}
	a.refreshDebugPanel()
}

func (a *App) rememberClosedDebugSource(path string) {
	if len(a.debugger.points[path]) == 0 {
		return
	}
	if a.debugger.closedSources == nil {
		a.debugger.closedSources = make(map[string]os.FileInfo)
	}
	info, _ := os.Stat(path)
	a.debugger.closedSources[path] = info
	delete(a.debugger.sources, path)
}

func (a *App) checkDebugSource(path string) {
	info, err := os.Stat(path)
	last, closed := a.debugger.closedSources[path]
	if err != nil || !info.Mode().IsRegular() || closed && (last == nil || !os.SameFile(last, info) || last.Size() != info.Size() || !last.ModTime().Equal(info.ModTime())) {
		changed := false
		for _, p := range a.debugger.points[path] {
			changed = changed || !p.confirm
			p.confirm = true
		}
		if changed {
			a.persistBreakpoints()
		}
	}
	if b := a.editorBufferForPath(path); b != nil {
		a.updateDebugSource(path, b)
		delete(a.debugger.closedSources, path)
	}
}
