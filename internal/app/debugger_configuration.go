package app

import (
	"maps"
	"path/filepath"
	"slices"

	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type debugIdleAction struct{ button, command, file string }

func (a *App) refreshDebugConfiguration() {
	p := &a.debugger.panel
	if p.toolbar == nil {
		return
	}
	action := debugIdleAction{button: i18n.Text("debug.createConfiguration"), command: "debug.configure"}
	var targets []dap.Target
	for _, folder := range a.Workspace.Folders {
		path, list, err := dap.ReadLaunchFile(folder.Path)
		if path != "" && action.file == "" {
			action.file = path
		}
		if err != nil {
			targets = nil
			action.file = path
			break
		}
		targets = append(targets, list...)
	}
	if len(targets) > 0 {
		action.command = "debug.start"
		action.button = i18n.Text("debug.runAndDebug")
	}
	p.idle = action
	a.refreshDebugControls()
}

func (a *App) currentFileDebugTarget() (dap.Target, bool) {
	g := a.EditorGroup
	if g == nil || g.ActiveBuffer() == nil || g.IsActiveVirtual() || g.ActiveDiffWidget() != nil {
		return dap.Target{}, false
	}
	path := g.ActiveFilePath()
	if !filepath.IsAbs(path) {
		return dap.Target{}, false
	}
	language := highlight.DetectLanguageID(path, g.ActiveBuffer().Lines)
	folder := filepath.Dir(path)
	if owner := a.Workspace.FolderForFile(path); owner != nil {
		folder = owner.Path
	}
	for _, name := range slices.Sorted(maps.Keys(a.Settings.Debug.Adapters)) {
		entry := a.Settings.Debug.Adapters[name]
		if (entry.Provider != "" && entry.Provider != "generic") || (entry.Transport != "" && entry.Transport != "stdio" && entry.Transport != "tcp") || !slices.Contains(entry.Languages, language) {
			continue
		}
		if target, ok := dap.Adapter(entry).FileTarget(folder, path, name); ok {
			return target, true
		}
	}
	return dap.Target{}, false
}

func (a *App) debugFile() {
	if a.debugger.run != nil || a.Root.HasModalOverlay() {
		return
	}
	target, ok := a.currentFileDebugTarget()
	if !ok {
		a.StatusWarn(i18n.Text("debug.fileUnavailable"))
		return
	}
	if a.EditorGroup.IsDirtyPath(a.EditorGroup.ActiveFilePath()) {
		a.ShowConfirmDialog(i18n.Format("debug.saveFile", i18n.Args{"filename": target.Name}),
			[]string{i18n.Text("general.cancel"), i18n.Text("general.save")},
			[]func(){a.DismissDialog, func() { a.DismissDialog(); a.saveFile(func() { a.prepareDebugTarget(target) }) }})
		return
	}
	a.prepareDebugTarget(target)
}

func (a *App) configureDebugger() {
	a.refreshDebugConfiguration()
	if path := a.debugger.panel.idle.file; path != "" {
		a.openDebugConfiguration(path)
		return
	}
	paths := a.Workspace.Paths()
	switch {
	case len(paths) == 0:
		a.StatusWarn(i18n.Text("status.noFolderOpen"))
	case len(paths) == 1:
		a.createDebugConfiguration(paths[0])
	default:
		if owner := a.Workspace.FolderForFile(a.EditorGroup.ActiveFilePath()); owner != nil {
			a.createDebugConfiguration(owner.Path)
			return
		}
		items := make([]widgets.SelectItem, 0, len(paths))
		for _, path := range paths {
			items = append(items, widgets.SelectItem{ID: path, Label: path})
		}
		a.ShowSelectDialog(i18n.Text("debug.chooseFolder"), items, a.createDebugConfiguration, nil)
	}
}

func (a *App) createDebugConfiguration(folder string) {
	if path, _, _ := dap.ReadLaunchFile(folder); path != "" {
		a.openDebugConfiguration(path)
		return
	}
	source := a.EditorGroup.ActiveFilePath()
	if owner := a.Workspace.FolderForFile(source); owner == nil || owner.Path != folder || a.EditorGroup.ActiveBuffer() == nil || a.EditorGroup.IsActiveVirtual() {
		source = ""
	}
	adapter := ""
	language := highlight.DetectLanguageID(source, nil)
	for _, name := range slices.Sorted(maps.Keys(a.Settings.Debug.Adapters)) {
		if slices.Contains(a.Settings.Debug.Adapters[name].Languages, language) {
			adapter = name
			break
		}
	}
	path, err := dap.CreateLaunchFile(folder, source, adapter, dap.Adapter(a.Settings.Debug.Adapters[adapter]))
	if err != nil {
		a.StatusError(err.Error())
		return
	}
	a.openDebugConfiguration(path)
}

func (a *App) openDebugConfiguration(path string) {
	if err := a.EditorGroup.OpenFile(path); err != nil {
		a.StatusError(err.Error())
		return
	}
	a.FocusEditor()
	a.refreshDebugConfiguration()
}
