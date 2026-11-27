package app

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type debuggerState struct {
	points map[string][]int
	run    *debugRun
	file   string
	line   int
}

func registerDebuggerCommands(a *App) {
	a.Reg.Register(command.Command{ID: "debug.start", Title: i18n.Text("debug.start"), Handler: a.StartDebugging})
	a.Reg.Register(command.Command{ID: "debug.stop", Title: i18n.Text("debug.stop"), Handler: a.StopDebugging})
	a.Reg.Register(command.Command{ID: "debug.toggleBreakpoint", Title: i18n.Text("debug.toggleBreakpoint"), Handler: a.ToggleDebugBreakpoint})
}

func (a *App) wireDebuggerGutter(g *ui.EditorGroupWidget) {
	g.OnGutterClick = func(path string, line int) bool {
		plugin := a.PluginManager != nil && a.PluginManager.HasEventListeners("gutter.click")
		if (!plugin || a.debugger.run != nil) && a.toggleEditorBreakpoint(g, line+1) {
			return true
		}
		if plugin {
			a.PluginManager.DispatchEvent("gutter.click", path, line+1)
		}
		return plugin
	}
	g.Editor.BookmarkOverlay = func(line int) (ui.Bookmark, bool) {
		path := g.ActiveFilePath()
		if path != "" && path == a.debugger.file && line+1 == a.debugger.line {
			return ui.Bookmark{Icon: icons.Rune(icons.DebugExecution), Style: term.StyleDebugExecution}, true
		}
		if slices.Contains(a.debugger.points[path], line+1) {
			return ui.Bookmark{Icon: icons.Rune(icons.Breakpoint), Style: term.StyleDebugBreakpoint}, true
		}
		return ui.Bookmark{}, false
	}
}

func (a *App) ToggleDebugBreakpoint() {
	g := a.EditorGroup
	a.toggleEditorBreakpoint(g, g.Editor.Cursor.Line+1)
}
func (a *App) toggleEditorBreakpoint(g *ui.EditorGroupWidget, line int) bool {
	path := g.ActiveFilePath()
	buf := g.ActiveBuffer()
	if buf == nil || g.IsActiveVirtual() || g.ActiveDiffWidget() != nil || !filepath.IsAbs(path) || line < 1 || line > len(buf.Lines) {
		return false
	}
	lang := highlight.DetectLanguageID(path, buf.Lines)
	for _, adapter := range a.Settings.Debug.Adapters {
		if !slices.Contains(adapter.Languages, lang) {
			continue
		}
		if a.debugger.points == nil {
			a.debugger.points = make(map[string][]int)
		}
		points := a.debugger.points[path]
		i, found := slices.BinarySearch(points, line)
		if found {
			points = slices.Delete(points, i, i+1)
		} else {
			points = slices.Insert(points, i, line)
		}
		if len(points) == 0 {
			delete(a.debugger.points, path)
		} else {
			a.debugger.points[path] = points
		}
		if r := a.debugger.run; r != nil {
			a.queueDebugPoints(r)
		}
		return true
	}
	return false
}

func (a *App) queueDebugPoints(r *debugRun) {
	points := make(map[string][]int)
	for path, lines := range a.debugger.points {
		owner := a.Workspace.FolderForFile(path)
		if owner == nil || owner.Path != r.folder {
			continue
		}
		lang := highlight.DetectLanguageID(path, nil)
		for _, g := range a.editorGroups() {
			if b := g.BufferForPath(path); b != nil {
				lang = highlight.DetectLanguageID(path, b.Lines)
				break
			}
		}
		if slices.Contains(r.languages, lang) {
			points[path] = slices.Clone(lines)
		}
	}
	select {
	case <-r.points:
	default:
	}
	r.points <- points
}

func (a *App) StopDebugging() {
	if r := a.debugger.run; r != nil {
		r.cancel()
	}
}
func (a *App) StartDebugging() {
	if a.debugger.run != nil || a.Root.HasModalOverlay() {
		return
	}
	for _, g := range a.editorGroups() {
		for _, folder := range a.Workspace.Folders {
			for _, dir := range []string{".ttt", ".vscode"} {
				path := filepath.Join(folder.Path, dir, "launch.json")
				if g.IsDirtyPath(path) {
					a.activateEditorGroup(g)
					g.SwitchToTabByPath(path)
					a.saveFile(a.StartDebugging)
					return
				}
			}
		}
	}

	var targets []dap.Target
	for _, folder := range a.Workspace.Folders {
		list, err := dap.LoadTargets(folder.Path)
		if err != nil {
			a.StatusError(err.Error())
			return
		}
		targets = append(targets, list...)
	}
	switch len(targets) {
	case 0:
		a.StatusError(i18n.Text("debug.noTargets"))
	case 1:
		a.prepareDebugTarget(targets[0])
	default:
		items := make([]widgets.SelectItem, 0, len(targets))
		for i, t := range targets {
			items = append(items, widgets.SelectItem{ID: strconv.Itoa(i), Label: t.Name + " (" + t.Folder + ")"})
		}
		a.showSelectDialog(i18n.Text("debug.start"), items, "", func(id string) {
			i, err := strconv.Atoi(id)
			if err == nil && i >= 0 && i < len(targets) {
				a.prepareDebugTarget(targets[i])
			}
		}, nil)
	}
}

func (a *App) prepareDebugTarget(t dap.Target) {
	if a.debugger.run != nil || a.tasks.post == nil {
		return
	}
	adapter, err := dap.LookupAdapter(a.Settings.Debug.Adapters, t.Type)
	if err != nil {
		a.StatusError(err.Error())
		return
	}
	if t.Request != "launch" || adapter.Provider != "generic" || (adapter.Transport != "" && adapter.Transport != "stdio") {
		a.StatusError(i18n.Text("debug.unsupportedLaunch"))
		return
	}
	args, err := adapter.Arguments(t)
	if err != nil {
		a.StatusError(err.Error())
		return
	}
	cmd, err := adapter.Process(t.Folder)
	if err != nil {
		a.StatusError(err.Error())
		return
	}
	for _, g := range a.editorGroups() {
		for _, path := range g.OpenFilePaths() {
			owner := a.Workspace.FolderForFile(path)
			if owner != nil && owner.Path == t.Folder && g.IsDirtyPath(path) {
				a.activateEditorGroup(g)
				g.SwitchToTabByPath(path)
				a.saveFile(func() { a.prepareDebugTarget(t) })
				return
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &debugRun{cancel: cancel, folder: t.Folder, languages: adapter.Languages, points: make(chan map[string][]int, 1), closed: make(chan struct{})}
	a.debugger.run = r
	a.queueDebugPoints(r)
	post := a.tasks.post
	deliver := func(f func()) {
		_ = post(func() {
			if a.debugger.run == r {
				f()
			}
		})
	}
	go func() {
		defer close(r.closed)
		err := r.launch(ctx, cmd, t.Type, args, func(path string, line int) {
			deliver(func() {
				a.debugger.file, a.debugger.line = path, line
				a.navigateToLocation(path, line-1, 0)
				a.StatusNotify(i18n.Format("debug.stoppedAt", i18n.Args{"file": filepath.Base(path), "line": line}))
			})
		})
		canceled := ctx.Err() != nil
		cancel()
		deliver(func() {
			a.debugger.run = nil
			a.debugger.file = ""
			a.debugger.line = 0
			if err != nil && !canceled {
				a.StatusError(err.Error())
			} else {
				a.StatusNotify(i18n.Text("debug.ended"))
			}
		})
	}()
}
