package app

import (
	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type debugIdleAction struct{ label, button, command, file string }

func (a *App) refreshDebugConfiguration() {
	p := &a.debugger.panel
	if p.toolbar == nil {
		return
	}
	action := debugIdleAction{label: i18n.Text("debug.setup"), button: i18n.Text("debug.createConfiguration"), command: "debug.configure"}
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
		action = debugIdleAction{command: "debug.start", button: i18n.Text("debug.runAndDebug"), label: i18n.Text("debug.chooseTarget")}
		if len(targets) == 1 {
			action.label = targets[0].Name
		}
	} else if action.file != "" {
		action.button = i18n.Text("debug.editConfiguration")
	}
	p.idle = action
	a.refreshDebugControls()
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
	path, err := dap.CreateLaunchFile(folder, source)
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
