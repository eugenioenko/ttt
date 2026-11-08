package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
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
	appsSearchDelay    = 150 * time.Millisecond
)

type AppsPanel struct {
	SearchInput *widgets.InputWidget
	Tree        *widgets.TreeWidget
	Adapter     *ui.WidgetAdapter
	OnLaunch    func(name string)
	OnAdd       func()
	OnSearch    func(string)
	apps        []config.Application
	mode        string
	content     *widgets.VStackWidget
	addButton   *widgets.ButtonWidget
}

func NewAppsPanel() *AppsPanel {
	p := &AppsPanel{}
	p.SearchInput = widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: i18n.Text("apps.search"),
		OnChange: func(query string) {
			if p.OnSearch != nil {
				p.OnSearch(query)
			}
		},
		OnSubmit: func(query string) {
			p.Filter(query)
			p.Tree.ActivateSelected()
		},
	})
	p.Tree = widgets.NewTreeWidget(widgets.TreeConfig{
		EmptyText: i18n.Text("apps.noMatches"),
		Inset:     1,
		OnCommand: func(cmd string, node *widgets.TreeNode) {
			if cmd == "activate" && node != nil && p.OnLaunch != nil {
				p.OnLaunch(node.ID)
			}
		},
	})
	p.addButton = widgets.NewButtonWidget(widgets.ButtonConfig{Label: i18n.Text("apps.add"), OnClick: func() {
		if p.OnAdd != nil {
			p.OnAdd()
		}
	}})
	p.content = widgets.NewVStackWidget(p.SearchInput, widgets.NewDividerWidget(widgets.DividerConfig{}), p.Tree, p.addButton)
	p.Adapter = ui.NewWidgetAdapter(p.content)
	return p
}

func (p *AppsPanel) SetApps(apps []config.Application) {
	p.apps = apps
	p.Filter(p.SearchInput.Text())
	p.content.Children = p.content.Children[:3]
	if len(apps) == 0 {
		p.content.Children = append(p.content.Children, p.addButton)
	}
	p.Adapter.RebuildFocus()
}

func appIcon(app config.Application, mode string) string {
	if app.Icon != "" {
		return app.Icon
	}
	return icons.Get(mode, icons.PanelApps)
}

func (p *AppsPanel) Filter(query string) {
	p.setFilteredApps(filterApps(p.apps, query))
}

func filterApps(apps []config.Application, query string) []config.Application {
	query = strings.ToLower(strings.TrimSpace(query))
	var matches []config.Application
	for _, app := range apps {
		if strings.Contains(strings.ToLower(app.Name), query) {
			matches = append(matches, app)
		}
	}
	return matches
}

func (p *AppsPanel) setFilteredApps(apps []config.Application) {
	nodes := make([]*widgets.TreeNode, 0, len(apps))
	for _, app := range apps {
		nodes = append(nodes, &widgets.TreeNode{ID: app.Name, Label: app.Name, Icon: appIcon(app, p.mode)})
	}
	p.Tree.SetItems(nodes)
}

func (a *App) scheduleAppsSearch(query string) {
	if a.appsSearch == nil {
		a.appsSearch = NewTask[[]config.Application](a, asynctask.TaskOptions{Delay: appsSearchDelay})
	}
	apps := a.apps
	a.appsSearch.Schedule(func(context.Context) ([]config.Application, error) { return filterApps(apps, query), nil }, func(apps []config.Application, err error) {
		if err == nil {
			a.Apps.setFilteredApps(apps)
		}
	})
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

func (a *App) syncApps() {
	if a.Apps == nil {
		a.Apps = NewAppsPanel()
		a.Apps.OnLaunch = a.LaunchApp
		a.Apps.OnAdd = a.AddApp
		a.Apps.OnSearch = a.scheduleAppsSearch
	}
	if a.appsSearch != nil {
		a.appsSearch.Stop()
	}
	apps := a.validApps()
	a.apps = apps
	a.Apps.mode = a.Settings.Appearance.Icons
	a.Apps.SetApps(apps)

	shortcuts := make(map[string]string, len(a.appCommandIDs))
	for _, id := range a.appCommandIDs {
		if cmd, ok := a.Reg.Get(id); ok {
			shortcuts[id] = cmd.Shortcut
		}
		a.Reg.Unregister(id)
	}
	a.appCommandIDs = a.appCommandIDs[:0]
	for _, app := range apps {
		name := app.Name
		title := strings.TrimSpace(app.CommandText)
		if title == "" {
			title = name
		}
		id := appCommandIDPrefix + name
		a.Reg.Register(command.Command{
			ID:       id,
			Title:    title,
			Shortcut: shortcuts[id],
			Keywords: []string{"apps", "run", "launch", "open", name},
			Handler:  func() { a.LaunchApp(name) },
		})
		a.appCommandIDs = append(a.appCommandIDs, id)
		if app.OpenWith {
			id = "apps.openWith:" + name
			a.Reg.Register(command.Command{ID: id, Title: i18n.Format("command.appsOpenWith", i18n.Args{"name": name}), Shortcut: shortcuts[id], Hidden: true, Handler: func() { a.ExplorerOpenWith(name) }})
			a.appCommandIDs = append(a.appCommandIDs, id)
		}
	}

	if !a.Sidebar.HasPanel(appsPanelID) {
		a.Sidebar.AddPanel(appsPanelID, i18n.Text("panel.apps"), a.Apps.Adapter)
		a.Sidebar.SetPanelIcon(appsPanelID, string(icons.Rune(icons.PanelApps)))
	}
}

func (a *App) ShowApps() {
	a.Sidebar.SetActivePanel(appsPanelID)
	if !a.Sidebar.Visible {
		a.ShowSidebar()
	}
	if len(a.apps) == 0 {
		a.AddApp()
	}
	if !a.Root.HasOverlay() {
		a.Root.SetFocus(a.Apps.Adapter)
	}
}

func (a *App) LaunchApp(name string) {
	idx := slices.IndexFunc(a.apps, func(app config.Application) bool { return app.Name == name })
	if idx >= 0 {
		file := a.EditorGroup.RecentDocumentPath()
		if a.EditorGroup.IsEditorActive() && a.EditorGroup.IsActiveVirtual() {
			file = ""
		}
		a.launchApp(name, file, appUsesFile(a.apps[idx]))
	}
}

type appLaunchKey struct{ name, path string }

func (a *App) OpenCurrentFileWith() {
	if a.Root.HasOverlay() {
		return
	}
	file := a.EditorGroup.RecentDocumentPath()
	if a.EditorGroup.IsEditorActive() && a.EditorGroup.IsActiveVirtual() {
		file = ""
	}
	var items []widgets.SelectItem
	for _, app := range a.apps {
		items = append(items, widgets.SelectItem{ID: app.Name, Label: appIcon(app, a.Settings.Appearance.Icons) + " " + app.Name})
	}
	if len(items) == 0 {
		a.AddOpenWithApp()
		return
	}
	a.ShowSelectDialog(i18n.Text("apps.openCurrentFileWith"), items, func(name string) { a.launchApp(name, file, true) }, nil)
}

func appUsesFile(app config.Application) bool {
	if strings.Contains(app.Command, "{file}") || strings.Contains(app.Cwd, "{file}") {
		return true
	}
	return slices.ContainsFunc(app.Args, func(arg string) bool { return strings.Contains(arg, "{file}") })
}

func (a *App) ExplorerOpenWith(name string) {
	path := a.explorerNodePath()
	a.ExplorerContextNode = nil
	if path == "" {
		a.StatusWarn(i18n.Text("status.noFileSelected"))
		return
	}
	a.launchApp(name, path, true)
}

func (a *App) forgetAppTerminal(id string) {
	for key, terminalID := range a.appTerminals {
		if terminalID == id {
			delete(a.appTerminals, key)
		}
	}
}

func (a *App) launchApp(name, file string, explicit bool) {
	idx := slices.IndexFunc(a.apps, func(app config.Application) bool { return app.Name == name })
	if idx < 0 {
		return
	}
	app := a.apps[idx]
	if file != "" {
		if abs, err := filepath.Abs(file); err == nil {
			file = abs
		}
	}
	key := appLaunchKey{name: name}
	if explicit {
		key.path = file
	}
	if id, ok := a.appTerminals[key]; ok && a.terminalByID(id) != nil {
		a.FocusTerminal(id)
		return
	}
	workspace := a.Workspace.Primary()
	if folder := a.Workspace.FolderForFile(file); folder != nil {
		workspace = folder.Path
	}
	if slices.Contains(a.Workspace.Paths(), file) {
		workspace = file
	}
	dir := workspace
	if file != "" {
		dir = filepath.Dir(file)
		if info, err := os.Stat(file); err == nil && info.IsDir() {
			dir = file
		}
	}
	spec := appTerminalSpec(app, file, dir, workspace, runtime.GOOS)
	if strings.TrimSpace(spec.Shell) == "" {
		a.StatusError(i18n.Text("apps.invalidEntry"))
		return
	}
	id, err := a.SpawnTerminalSpec(spec)
	if err != nil {
		return
	}
	if a.appTerminals == nil {
		a.appTerminals = map[appLaunchKey]string{}
	}
	a.appTerminals[key] = id
	if tt := a.terminalByID(id); tt != nil && tt.Tab != nil {
		tt.Tab.Icon = app.Icon
	}
}
