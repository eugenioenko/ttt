package app

import (
	"context"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"
	"weak"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
)

type debuggerState struct {
	picker                 *asynctask.Task[[]dap.Process]
	lastFolder, lastTarget string
	panel                  debugSidebar
	run                    *debugRun
	closedSources          map[string]os.FileInfo
	sources                map[string]debugSourceVersion
	points                 map[string]map[int]*debugBreakpoint
	stoppedFile            string
	stoppedLine            int
}

func (a *App) wireDebuggerGutter(g *ui.EditorGroupWidget) {
	g.OnGutterClick = func(path string, line int) bool {
		pluginHandlesClick := a.PluginManager != nil && a.PluginManager.HasEventListeners("gutter.click")
		debugView := a.Sidebar.Visible && a.Sidebar.ActivePanel == "debug"
		if (debugView || !pluginHandlesClick) && a.hasDebugAdapter(path) && g.ActiveBuffer() != nil && !g.IsActiveVirtual() && g.ActiveDiffWidget() == nil && filepath.IsAbs(path) {
			a.toggleEditorBreakpoint(g, line+1)
			return true
		}
		if pluginHandlesClick {
			a.PluginManager.DispatchEvent("gutter.click", path, line+1)
			return true
		}
		return false
	}
	g.Editor.BookmarkOverlay = func(line int) (ui.Bookmark, bool) {
		path := g.ActiveFilePath()
		if path == a.debugger.stoppedFile && line+1 == a.debugger.stoppedLine {
			return ui.Bookmark{Icon: icons.Rune(icons.DebugExecution), Style: term.StyleDebugExecution}, true
		}
		if point, ok := a.debugger.points[path][line+1]; ok {
			return ui.Bookmark{Icon: icons.Rune(a.breakpointIcon(point)), Style: term.StyleDebugBreakpoint}, true
		}
		return ui.Bookmark{}, false
	}
}

func (a *App) breakpointIcon(point *debugBreakpoint) icons.Name {
	if !point.confirm && (a.debugger.run == nil || point.binding.Verified) {
		return icons.Breakpoint
	}
	return icons.PendingBreakpoint
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
	if p := points[line]; p != nil && p.confirm {
		p.confirm = false
	} else if p != nil {
		delete(points, line)
	} else {
		points[line] = &debugBreakpoint{}
	}
	if r := a.debugger.run; r != nil && a.debugOwnsSource(r, path) {
		if p := points[line]; p != nil && !r.edited[path] {
			p.runtimeLine = line
		}
		r.revision[path]++
		a.syncDebugBreakpoints(path)
	}
	a.refreshDebugPanel()
}

func (a *App) ClearDebugBreakpoints() {
	for path := range a.debugger.points {
		delete(a.debugger.points, path)
		if r := a.debugger.run; r != nil && a.debugOwnsSource(r, path) {
			r.revision[path]++
			a.syncDebugBreakpoints(path)
		}
	}
	clear(a.debugger.sources)
	clear(a.debugger.closedSources)
	a.refreshDebugPanel()
}

func (a *App) syncDebugBreakpoints(path string) {
	r := a.debugger.run
	if r == nil || !r.ready || r.sending[path] || r.ctx.Err() != nil {
		return
	}
	r.sending[path] = true
	revision := r.revision[path]
	lines := a.debugLines(r, path)
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
	if !a.hasDebugAdapter(g.ActiveFilePath()) || g.ActiveBuffer() == nil || g.IsActiveVirtual() || !filepath.IsAbs(g.ActiveFilePath()) {
		return
	}
	a.toggleEditorBreakpoint(g, g.Editor.Cursor.Line+1)
}

type debugBreakpoint struct {
	runtimeLine int
	confirm     bool
	binding     dap.Breakpoint
}

func (a *App) debugOwnsSource(r *debugRun, path string) bool {
	folder := a.Workspace.FolderForFile(path)
	return folder != nil && folder.Path == r.folder && slices.Contains(r.languages, a.debugLanguage(path))
}

func (a *App) applyDebugBindings(path string, lines []int, bindings []dap.Breakpoint) {
	byLine := a.debugger.points[path]
	if r := a.debugger.run; r != nil && r.edited[path] {
		byLine = make(map[int]*debugBreakpoint, len(byLine))
		for _, p := range a.debugger.points[path] {
			if p.runtimeLine > 0 {
				byLine[p.runtimeLine] = p
			}
		}
	}
	for i, line := range lines {
		if p := byLine[line]; p != nil {
			p.binding = dap.Breakpoint{}
			if i < len(bindings) {
				p.binding = bindings[i]
				p.binding.Source.Path = a.debugSourcePath(p.binding.Source.Path)
			}
		}
	}
	a.refreshDebugPanel()
}

func (a *App) debugBindingEvent(r *debugRun, b dap.Breakpoint, reason string) {
	b.Source.Path = a.debugSourcePath(b.Source.Path)
	var match *debugBreakpoint
	for path, points := range a.debugger.points {
		if !a.debugOwnsSource(r, path) {
			continue
		}
		for line, p := range points {
			if p.confirm {
				continue
			}
			if b.ID != nil && p.binding.ID != nil {
				if *p.binding.ID == *b.ID {
					match = p
					break
				}
			} else if path == b.Source.Path && ((!r.edited[path] && line == b.Line) || (r.edited[path] && p.runtimeLine == b.Line)) {
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

type debugSourceVersion struct {
	buf     weak.Pointer[buffer.Buffer]
	version uint64
}

func (a *App) toggleEditorBreakpoint(g *ui.EditorGroupWidget, line int) {
	path := g.ActiveFilePath()
	buf := g.ActiveBuffer()
	if buf == nil {
		return
	}
	a.updateDebugSource(path, buf)
	delete(a.debugger.closedSources, path)
	if r := a.debugger.run; r != nil && buf.Dirty && a.debugOwnsSource(r, path) {
		r.edited[path] = true
	}
	a.toggleBreakpoint(path, line)
	if a.debugger.sources == nil {
		a.debugger.sources = make(map[string]debugSourceVersion)
	}
	if len(a.debugger.points[path]) == 0 {
		delete(a.debugger.sources, path)
	} else {
		a.debugger.sources[path] = debugSourceVersion{weak.Make(buf), buf.Version()}
	}
}

func (a *App) debugLines(r *debugRun, path string) []int {

	var lines []int
	for line, p := range a.debugger.points[path] {
		if r.edited[path] {
			line = p.runtimeLine
		}
		if !p.confirm && line > 0 {
			lines = append(lines, line)
		}
	}
	slices.Sort(lines)
	return lines
}

func (a *App) updateDebugSource(path string, b *buffer.Buffer) {
	prev, ok := a.debugger.sources[path]
	if !ok || b == nil {
		return
	}
	now := debugSourceVersion{weak.Make(b), b.Version()}
	if now == prev {
		return
	}
	points := a.debugger.points[path]
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
				p.binding = dap.Breakpoint{}
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
			p.binding = dap.Breakpoint{}
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
	if r := a.debugger.run; r != nil && a.debugOwnsSource(r, path) {
		r.edited[path] = true
		if resync {
			r.revision[path]++
			a.syncDebugBreakpoints(path)
		}
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
}
func (a *App) checkDebugSource(path string) {
	info, err := os.Stat(path)
	last, closed := a.debugger.closedSources[path]
	if err != nil || !info.Mode().IsRegular() || closed && (last == nil || !os.SameFile(last, info) || last.Size() != info.Size() || !last.ModTime().Equal(info.ModTime())) {
		for _, p := range a.debugger.points[path] {
			p.confirm = true
		}
	}
	if b := a.editorBufferForPath(path); b != nil {
		a.updateDebugSource(path, b)
		delete(a.debugger.closedSources, path)
	}
}

func (a *App) debugSourcePath(path string) string {
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if _, ok := a.debugger.points[path]; ok {
		return path
	}
	var candidates []string
	for _, g := range a.editorGroups() {
		if g != nil {
			if g.BufferForPath(path) != nil {
				return path
			}
			candidates = append(candidates, g.OpenFilePaths()...)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return path
	}
	for _, p := range append(candidates, slices.Collect(maps.Keys(a.debugger.points))...) {
		if other, err := os.Stat(p); err == nil && os.SameFile(info, other) {
			return p
		}
	}
	return path
}

func (a *App) debugLanguage(path string) string {
	for _, g := range a.editorGroups() {
		if g != nil {
			if b := g.BufferForPath(path); b != nil {
				return highlight.DetectLanguageID(path, b.Lines)
			}
		}
	}
	return highlight.DetectLanguageID(path, nil)
}
func (a *App) hasDebugAdapter(path string) bool {
	lang := a.debugLanguage(path)
	for _, adapter := range a.Settings.Debug.Adapters {
		if slices.Contains(adapter.Languages, lang) {
			return true
		}
	}
	return false
}
