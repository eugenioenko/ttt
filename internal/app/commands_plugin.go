package app

import (
	"log/slog"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func registerPluginCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID:       "plugin.list",
		Title:    i18n.Text("command.pluginsListInstalled"),
		Keywords: []string{"plugin", "extension"},
		Handler:  func() { app.showPluginList() },
	})

	reg.Register(command.Command{
		ID:       "plugin.install",
		Title:    i18n.Text("command.pluginsInstallFromURL"),
		Keywords: []string{"plugin", "extension", "install"},
		Handler:  func() { app.pluginInstall() },
	})

	reg.Register(command.Command{
		ID:       "plugin.uninstall",
		Title:    i18n.Text("command.pluginsUninstall"),
		Keywords: []string{"plugin", "extension", "remove"},
		Handler:  func() { app.pluginUninstall() },
	})

	reg.Register(command.Command{
		ID:       "plugin.update",
		Title:    i18n.Text("command.pluginsUpdate"),
		Keywords: []string{"plugin", "extension", "upgrade"},
		Handler:  func() { app.pluginUpdate() },
	})

	reg.Register(command.Command{
		ID:       "plugin.showPanel",
		Title:    i18n.Text("command.pluginsShowPanel"),
		Keywords: []string{"plugin", "extension", "panel"},
		Handler: func() {
			if app.PluginsPanel != nil {
				app.ShowPanel("plugins", app.PluginsPanel.Adapter)
			} else {
				app.Sidebar.SetActivePanel("plugins")
				app.SplitPanel.ShowLeft = true
			}
		},
	})

	reg.Register(command.Command{
		ID:       "plugin.refresh",
		Title:    i18n.Text("command.pluginsRefresh"),
		Keywords: []string{"plugin", "refresh", "registry", "available"},
		Handler:  func() { app.PluginRefreshRegistry() },
	})

	reg.Register(command.Command{
		ID:       "plugin.reload",
		Title:    i18n.Text("command.pluginsReload"),
		Keywords: []string{"plugin", "reload", "refresh"},
		Handler:  func() { app.pluginReload() },
	})

	reg.Register(command.Command{
		ID:       "plugin.reloadAll",
		Title:    i18n.Text("command.pluginsReloadAll"),
		Keywords: []string{"plugin", "reload", "refresh", "all"},
		Handler:  func() { app.pluginReloadAll() },
	})

}

func (a *App) showPluginList() {
	plugins := a.PluginManager.Plugins()
	if len(plugins) == 0 {
		a.ShowConfirmDialogEx(i18n.Text("plugins.plugins"), i18n.Text("plugins.noPluginsInstalled"), []string{i18n.Text("general.close")}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}

	var entries []widgets.KeyValueEntry
	for _, p := range plugins {
		status := i18n.Text("plugins.disabled")
		if p.Enabled {
			status = i18n.Text("plugins.enabled")
		}
		if p.LastError != nil {
			status = i18n.Text("plugins.error")
		}
		entries = append(entries, widgets.KeyValueEntry{
			Key:   p.Manifest.Title(),
			Value: status + " v" + p.Manifest.Version,
		})
	}

	a.ShowInfoDialog(i18n.Text("plugins.installedPlugins"), entries)
}

func (a *App) pluginInstall() {
	a.ShowInputDialogEx(i18n.Text("plugins.installPlugin"), i18n.Text("plugins.repositoryURL"), "", i18n.Text("general.install"), func(repoURL string) {
		go func() {
			p, err := a.PluginManager.Install(repoURL, "")
			if p != nil {
				p.InstalledFromURL = true
			}
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&pluginInstallResult{
				plugin: p,
				err:    err,
			}))
		}()
	})
}

type pluginInstallResult struct {
	plugin *plugin.Plugin
	name   string
	err    error
}

func (a *App) handlePluginInstallResult(result *pluginInstallResult) {
	if result.err != nil {
		a.resetPluginDetailButton(result.name)
		a.ShowConfirmDialogEx(i18n.Text("plugins.installFailed"), result.err.Error(), []string{i18n.Text("general.close")}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}
	a.updatePluginDetailButtons()
	a.PendingPluginApprovals = append(a.PendingPluginApprovals, result.plugin)
	if len(a.PendingPluginApprovals) == 1 {
		a.ShowPluginApprovalDialog(result.plugin)
	}
}

func (a *App) resetPluginDetailButton(name string) {
	if state, ok := a.pluginDetailWidgets[name]; ok {
		state.installBtn.SetLabel(i18n.Text("general.install"))
		state.installBtn.Disabled = false
	}
}

func (a *App) pluginUninstall() {
	names := a.PluginManager.InstalledPluginNames()
	if len(names) == 0 {
		a.ShowConfirmDialogEx(i18n.Text("general.uninstall"), i18n.Text("plugins.noPluginsInstalled"), []string{i18n.Text("general.close")}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}

	var items []widgets.SelectItem
	for _, name := range names {
		items = append(items, widgets.SelectItem{ID: name, Label: name})
	}

	a.ShowSelectDialog(i18n.Text("plugins.uninstallPlugin"), items, func(name string) {
		a.ShowConfirmDialogEx(i18n.Text("plugins.confirmUninstall"), i18n.Format("plugins.removePlugin", i18n.Args{"name": name}), []string{i18n.Text("general.cancel"), i18n.Text("general.uninstall")}, []func(){
			func() { a.DismissDialog() },
			func() {
				a.DismissDialog()
				a.doPluginUninstall(name)
			},
		})
	}, nil)
}

// ClearPluginSources removes the diagnostics, decorations and status items plugin name set.
func (a *App) ClearPluginSources(name string) {
	for _, group := range a.editorGroups() {
		group.ClearDiagnosticsSource("plugin:" + name)
		group.ClearDecorationsSource("plugin:" + name)
	}
	a.Status.RemoveSegmentsByOwner("plugin:" + name)
}

func (a *App) doPluginUninstall(name string) {
	if a.PluginManager.IsSystemPlugin(name) {
		a.StatusWarn(i18n.Text("plugins.systemManaged"))
		return
	}
	if err := a.PluginManager.RunUninstallHook(name); err != nil {
		a.ShowConfirmDialogEx(i18n.Text("plugins.uninstallFailed"), i18n.Format("plugins.cleanupFailed", i18n.Args{"name": name, "error": err.Error()}), []string{i18n.Text("general.cancel"), i18n.Text("plugins.removeAnyway")}, []func(){
			func() { a.DismissDialog() },
			func() {
				a.DismissDialog()
				a.removePlugin(name)
			},
		})
		return
	}
	a.removePlugin(name)
}

func (a *App) removePlugin(name string) {
	a.RemovePluginPanels(name)

	if err := a.PluginManager.Uninstall(name); err != nil {
		slog.Error("plugin uninstall", "error", err)
	}
	// After the uninstall hook: its on_uninstall callback can still publish and open tabs.
	a.ClearPluginSources(name)
	a.closePluginDrawerAndTabs(name)
	a.uninstallPluginGrammars(name)
	a.uninstallPluginThemes(name)
	a.uninstallPluginLanguages(name)

	a.resetPluginDetailButton(name)
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
}

func (a *App) pluginUpdate() {
	names := a.PluginManager.InstalledPluginNames()
	if len(names) == 0 {
		a.ShowConfirmDialogEx(i18n.Text("general.update"), i18n.Text("plugins.noPluginsInstalled"), []string{i18n.Text("general.close")}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}

	var items []widgets.SelectItem
	for _, name := range names {
		items = append(items, widgets.SelectItem{ID: name, Label: name})
	}

	a.ShowSelectDialog(i18n.Text("plugins.updatePlugin"), items, a.PluginUpdateByName, nil)
}

type PluginUpdateResult struct {
	update *plugin.PluginUpdate
	err    error
}

// HandlePluginUpdateResult swaps the plugin's Lua state, so it runs on the UI goroutine.
func (a *App) HandlePluginUpdateResult(result *PluginUpdateResult) {
	var p *plugin.Plugin
	var needsApproval bool
	err := result.err
	if err == nil {
		old := a.PluginManager.FindPlugin(result.update.Name())
		p, needsApproval, err = a.PluginManager.ApplyUpdate(result.update)
		if old != nil && !old.Loaded() {
			a.ClearPluginSources(old.Name)
			if p != nil && !needsApproval {
				a.closePluginDrawerAndTabs(old.Name)
			} else {
				a.RemovePluginPanels(old.Name)
			}
		}
	}
	if err != nil {
		a.ShowConfirmDialogEx(i18n.Text("plugins.updateFailed"), err.Error(), []string{i18n.Text("general.close")}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}
	if needsApproval && p != nil {
		a.PendingPluginApprovals = append(a.PendingPluginApprovals, p)
		if len(a.PendingPluginApprovals) == 1 {
			a.ShowPluginApprovalDialog(p)
		}
		return
	}
	if !needsApproval && p != nil {
		if p.Loaded() {
			a.WirePlugin(p)
			a.installPluginGrammars(p)
		} else {
			a.installPluginThemes(p)
			a.installPluginLanguages(p)
		}
	}
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
}

func (a *App) pluginReload() {
	plugins := a.PluginManager.Plugins()
	if len(plugins) == 0 {
		a.ShowConfirmDialogEx(i18n.Text("general.reload"), i18n.Text("plugins.noPluginsLoaded"), []string{i18n.Text("general.close")}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}

	if len(plugins) == 1 {
		a.doPluginReload(plugins[0].Name)
		return
	}

	var items []widgets.SelectItem
	for _, p := range plugins {
		items = append(items, widgets.SelectItem{ID: p.Name, Label: p.Name})
	}
	a.ShowSelectDialog(i18n.Text("plugins.reloadPlugin"), items, func(name string) {
		a.doPluginReload(name)
	}, nil)
}

func (a *App) doPluginReload(name string) {
	a.RemovePluginPanels(name)
	a.ClearPluginSources(name)

	p, err := a.PluginManager.Reload(name)
	if err != nil {
		a.Output.AddLine(ui.OutputLine{
			Time:       time.Now().Format("15:04:05"),
			PluginName: name,
			Level:      "error",
			Message:    i18n.Format("plugins.reloadFailed", i18n.Args{"error": err.Error()}),
		})
		if a.PluginsPanel != nil {
			a.PluginsPanel.Refresh()
		}
		return
	}

	a.WirePlugin(p)
	a.installPluginGrammars(p)

	a.Output.AddLine(ui.OutputLine{
		Time:       time.Now().Format("15:04:05"),
		PluginName: name,
		Level:      "info",
		Message:    i18n.Text("plugins.reloadedSuccessfully"),
	})
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
}

func (a *App) pluginReloadAll() {
	plugins := a.PluginManager.Plugins()
	if len(plugins) == 0 {
		return
	}

	var names []string
	for _, p := range plugins {
		names = append(names, p.Name)
	}
	for _, name := range names {
		a.doPluginReload(name)
	}
}

type RemoteRegistryResult struct {
	Entries []plugin.RemoteRegistryEntry
	Err     error
}

func (a *App) PluginInstallFromURL(repoURL, repoPath, name string) {
	go func() {
		p, err := a.PluginManager.Install(repoURL, repoPath)
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&pluginInstallResult{
			plugin: p,
			name:   name,
			err:    err,
		}))
	}()
}

func (a *App) PluginUninstallByName(name string) {
	a.ShowConfirmDialogEx(i18n.Text("plugins.confirmUninstall"), i18n.Format("plugins.removePlugin", i18n.Args{"name": name}), []string{i18n.Text("general.cancel"), i18n.Text("general.uninstall")}, []func(){
		func() { a.DismissDialog() },
		func() {
			a.DismissDialog()
			a.doPluginUninstall(name)
		},
	})
}

func (a *App) PluginToggleByName(name string, enabled bool) {
	if !enabled {
		a.RemovePluginPanels(name)
		a.ClearPluginSources(name)
	}
	p, err := a.PluginManager.SetEnabled(name, enabled)
	if err != nil {
		slog.Error("toggle plugin", "error", err)
	}
	if p != nil && p.Loaded() {
		a.WirePlugin(p)
		a.installPluginGrammars(p)
	}
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
	if p != nil && !p.Loaded() {
		a.RemovePluginPanels(name)
		a.ClearPluginSources(name)
		a.requestPluginApproval(p)
	}
}

func (a *App) PluginUpdateByName(name string) {
	update := a.PluginManager.PrepareUpdate(name)
	go func() {
		err := update.Fetch()
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&PluginUpdateResult{update: update, err: err}))
	}()
}

func (a *App) HandleRemoteRegistryResult(result *RemoteRegistryResult) {
	if a.PluginsPanel == nil {
		return
	}
	if result.Err != nil {
		a.PluginsPanel.setRegistryStatus(i18n.Text("plugins.couldNotLoadRegistry"))
		return
	}
	a.PluginsPanel.SetAvailable(result.Entries)
}

func (a *App) PluginRefreshRegistry() {
	a.registryRequested = true
	if a.PluginsPanel != nil {
		a.PluginsPanel.setRegistryStatus(i18n.Text("plugins.loadingPlugins"))
	}
	fetch := a.FetchRegistry
	if fetch == nil {
		fetch = func() ([]plugin.RemoteRegistryEntry, error) {
			return plugin.FetchRemoteRegistry(plugin.DefaultRegistryURL)
		}
	}
	go func() {
		entries, err := fetch()
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&RemoteRegistryResult{Entries: entries, Err: err}))
	}()
}
