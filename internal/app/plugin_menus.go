package app

import (
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) ShowPluginDropdownMenu(entries []widgets.MenuEntry, x, y int) {
	a.showWidgetMenu(entries, x, y, a.handlePluginDropdownCommand)
}

func contextMenuItemFromWidgetEntry(entry widgets.MenuEntry) ui.ContextMenuItem {
	item := ui.ContextMenuItem{
		Label:   entry.Label,
		Command: entry.Command,
		IsSep:   entry.Separator,
	}
	if !entry.Separator && entry.Checked != nil {
		item.Checked = ui.MenuUnchecked
		if *entry.Checked {
			item.Checked = ui.MenuChecked
		}
	}
	return item
}

func contextMenuItemsFromWidgetEntries(entries []widgets.MenuEntry) []ui.ContextMenuItem {
	items := make([]ui.ContextMenuItem, len(entries))
	for i, entry := range entries {
		items[i] = contextMenuItemFromWidgetEntry(entry)
	}
	return items
}

func (a *App) handlePluginDropdownCommand(cmd string) {
	switch cmd {
	case "updateAll":
		names := a.PluginManager.InstalledPluginNames()
		for _, name := range names {
			a.PluginUpdateByName(name)
		}
	case "refresh":
		a.PluginRefreshRegistry()
	case "help":
		a.Reg.Execute("plugin.help")
	}
}

// ShowPluginRowMenu opens the per-plugin actions menu (enable/disable, update,
// uninstall) for one installed plugin. Selecting a row no longer toggles it;
// these actions live here and on the e/u/x shortcuts.
func (a *App) ShowPluginRowMenu(name string, enabled bool, x, y int) {
	toggleLabel := i18n.Text("general.disable")
	if !enabled {
		toggleLabel = i18n.Text("general.enable")
	}
	items := []ui.ContextMenuItem{
		{Label: toggleLabel, Command: "toggle"},
		{Label: i18n.Text("general.update"), Command: "update"},
		{Label: i18n.Text("general.uninstall"), Command: "uninstall"},
	}
	menu := ui.NewContextMenuWidget(items, x, y)
	menu.Borders = a.Borders
	menu.Icons = a.Settings.Appearance.Icons
	restoreFocus := func() {
		a.Root.PopOverlay()
		if a.PluginsPanel != nil {
			a.Root.SetFocus(a.PluginsPanel.InstalledTree)
		}
	}
	menu.OnExec = func(cmd string) {
		restoreFocus()
		if a.PluginsPanel == nil {
			return
		}
		switch cmd {
		case "toggle":
			if a.PluginsPanel.OnToggle != nil {
				a.PluginsPanel.OnToggle(name, !enabled)
			} else {
				a.PluginToggleByName(name, !enabled)
			}
		case "update":
			if a.PluginsPanel.OnUpdate != nil {
				a.PluginsPanel.OnUpdate(name)
			} else {
				a.PluginUpdateByName(name)
			}
		case "uninstall":
			if a.PluginsPanel.OnUninstall != nil {
				a.PluginsPanel.OnUninstall(name)
			} else {
				a.PluginUninstallByName(name)
			}
		}
	}
	menu.OnDismiss = func() {
		restoreFocus()
	}
	a.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	a.Root.SetFocus(menu)
}

func (a *App) showWidgetMenu(entries []widgets.MenuEntry, x, y int, onCommand func(string)) {
	items := contextMenuItemsFromWidgetEntries(entries)
	a.captureMenuFocus()
	menu := ui.NewContextMenuWidget(items, x, y)
	menu.Borders = a.Borders
	menu.Icons = a.Settings.Appearance.Icons
	menu.OnExec = func(cmd string) {
		a.Root.PopOverlay()
		a.restoreMenuFocus()
		onCommand(cmd)
	}
	menu.OnDismiss = func() {
		a.Root.PopOverlay()
		a.restoreMenuFocus()
	}
	a.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	a.Root.SetFocus(menu)
}
