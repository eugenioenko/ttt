package app

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

const (
	appsPanelID        = "apps"
	appCommandIDPrefix = "apps.run:"
)

type AppsPanel struct {
	Tree     *widgets.TreeWidget
	Adapter  *ui.WidgetAdapter
	OnLaunch func(name string)
}

func NewAppsPanel() *AppsPanel {
	p := &AppsPanel{}
	p.Tree = widgets.NewTreeWidget(widgets.TreeConfig{
		OnCommand: func(cmd string, node *widgets.TreeNode) {
			if cmd == "activate" && node != nil && p.OnLaunch != nil {
				p.OnLaunch(node.ID)
			}
		},
	})
	p.Adapter = ui.NewWidgetAdapter(p.Tree)
	return p
}

func (p *AppsPanel) SetApps(apps []config.Application) {
	nodes := make([]*widgets.TreeNode, 0, len(apps))
	for _, app := range apps {
		nodes = append(nodes, &widgets.TreeNode{ID: app.Name, Label: app.Name})
	}
	p.Tree.SetItems(nodes)
}

// validApps drops entries without a name or command and repeated names,
// reporting each one.
func (a *App) validApps() []config.Application {
	seen := map[string]bool{}
	var apps []config.Application
	for _, app := range a.Settings.Applications {
		app.Name = strings.TrimSpace(app.Name)
		switch {
		case app.Name == "" || strings.TrimSpace(app.Command) == "":
			a.LogOutput("warn", "settings", i18n.Text("apps.invalidEntry"))
		case seen[app.Name]:
			a.LogOutput("warn", "settings", i18n.Format("apps.duplicateName", i18n.Args{"name": app.Name}))
		default:
			seen[app.Name] = true
			apps = append(apps, app)
		}
	}
	return apps
}

// syncApps rebuilds the Apps tab and its run commands from the settings. The
// tab exists only while at least one app is configured.
func (a *App) syncApps() {
	if a.Apps == nil {
		a.Apps = NewAppsPanel()
		a.Apps.OnLaunch = a.LaunchApp
	}
	apps := a.validApps()
	a.apps = apps
	a.Apps.SetApps(apps)

	for _, id := range a.appCommandIDs {
		a.Reg.Unregister(id)
	}
	a.appCommandIDs = a.appCommandIDs[:0]
	for _, app := range apps {
		name := app.Name
		id := appCommandIDPrefix + name
		a.Reg.Register(command.Command{
			ID:       id,
			Title:    i18n.Format("command.appsRun", i18n.Args{"name": name}),
			Keywords: []string{"apps", "run", "launch", "open"},
			Handler:  func() { a.LaunchApp(name) },
		})
		a.appCommandIDs = append(a.appCommandIDs, id)
	}

	switch {
	case len(apps) > 0 && !a.Sidebar.HasPanel(appsPanelID):
		a.Sidebar.AddPanel(appsPanelID, i18n.Text("panel.apps"), a.Apps.Adapter)
		a.Sidebar.SetPanelIcon(appsPanelID, string(icons.Rune(icons.PanelApps)))
	case len(apps) == 0 && a.Sidebar.HasPanel(appsPanelID):
		a.Sidebar.RemovePanel(appsPanelID)
	}
}

func (a *App) ShowApps() {
	if !a.Sidebar.HasPanel(appsPanelID) {
		a.StatusNotify(i18n.Text("apps.noneConfigured"))
		return
	}
	a.ShowPanel(appsPanelID, a.Apps.Adapter)
}

// LaunchApp runs the named app, or focuses its terminal while it still runs.
func (a *App) LaunchApp(name string) {
	idx := slices.IndexFunc(a.apps, func(app config.Application) bool { return app.Name == name })
	if idx < 0 {
		return
	}
	app := a.apps[idx]
	if id, ok := a.appTerminals[name]; ok && a.terminalByID(id) != nil {
		a.FocusTerminal(id)
		return
	}

	file := a.EditorGroup.RecentDocumentPath()
	if file != "" {
		if abs, err := filepath.Abs(file); err == nil {
			file = abs
		}
	}
	if file == "" && (strings.Contains(app.Command, "{file}") || strings.Contains(app.Cwd, "{file}")) {
		a.StatusError(i18n.Format("apps.needsFile", i18n.Args{"name": name}))
		return
	}
	workspace := a.Workspace.Primary()
	dir := workspace
	if file != "" {
		dir = filepath.Dir(file)
	}
	quoted := strings.NewReplacer("{file}", shellQuote(file), "{dir}", shellQuote(dir), "{workspace}", shellQuote(workspace))
	plain := strings.NewReplacer("{file}", file, "{dir}", dir, "{workspace}", workspace)

	id, err := a.SpawnTerminalSpec(TerminalSpec{
		Shell:    "/bin/sh",
		Args:     []string{"-c", quoted.Replace(app.Command)},
		Cwd:      plain.Replace(app.Cwd),
		Label:    name,
		InEditor: app.Location != "panel",
		KeepOpen: app.KeepOpen,
	})
	if err != nil {
		return
	}
	if a.appTerminals == nil {
		a.appTerminals = map[string]string{}
	}
	a.appTerminals[name] = id
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
