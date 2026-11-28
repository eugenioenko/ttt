package app

import (
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) ShowSidebarMoreMenu(sx, sy int) {
	var items []ui.ContextMenuItem
	switch a.Sidebar.ActivePanel {
	case "explorer":
		items = []ui.ContextMenuItem{
			{Label: i18n.Text("explorer.searchInFolders"), Command: "explorer.find"},
			ui.MenuSep(),
			{Label: i18n.Text("general.newFile"), Command: "file.new"},
			{Label: i18n.Text("general.addFolder"), Command: "workspace.addFolder"},
			{Label: i18n.Text("general.refresh"), Command: "explorer.refresh"},
			ui.MenuSep(),
			{Label: i18n.Text("general.expandAll"), Command: "explorer.expandAll"},
			{Label: i18n.Text("general.collapseAll"), Command: "explorer.collapseAll"},
			ui.MenuSep(),
			{Label: i18n.Text("general.help"), Command: "explorer.help"},
		}
	case "search":
		replaceLabel := i18n.Text("general.replace")
		if a.Search.IsReplaceMode() {
			replaceLabel = i18n.Text("general.search")
		}
		items = []ui.ContextMenuItem{
			{Label: replaceLabel, Command: "sidebar.searchReplace"},
			ui.MenuSep(),
			{Label: i18n.Text("general.expandAll"), Command: "search.expandAll"},
			{Label: i18n.Text("general.collapseAll"), Command: "search.collapseAll"},
			ui.MenuSep(),
			{Label: i18n.Text("menu.clearResults"), Command: "search.clear"},
			ui.MenuSep(),
			{Label: i18n.Text("general.help"), Command: "search.help"},
		}
	case "debug":
		items = a.debugToolbarMenu()
	case "changes":
		items = a.BuildChangesPanelMenu()
	case "outline":
		items = []ui.ContextMenuItem{
			{Label: i18n.Text("general.refresh"), Command: "sidebar.outline"},
		}
	case appsPanelID:
		items = []ui.ContextMenuItem{{Label: i18n.Text("apps.add"), Command: "apps.add"}}
	case "plugins":
		items = []ui.ContextMenuItem{
			{Label: i18n.Text("menu.installFromURL"), Command: "plugin.install"},
			{Label: i18n.Text("general.refresh"), Command: "plugin.refresh"},
			ui.MenuSep(),
			{Label: i18n.Text("general.help"), Command: "plugin.help"},
		}
	default:
		if a.PluginManager != nil {
			for _, p := range a.PluginManager.Plugins() {
				if a.Sidebar.ActivePanel == "plugin."+p.Name && len(p.SidebarMenuEntries) > 0 {
					items = contextMenuItemsFromWidgetEntries(p.SidebarMenuEntries)
					break
				}
			}
		}
	}
	moveItems := a.sidebarMoveMenuItems()
	if len(moveItems) > 0 {
		if len(items) > 0 && !items[len(items)-1].IsSep {
			items = append(items, ui.MenuSep())
		}
		items = append(items, moveItems...)
	}
	if len(items) > 0 {
		openContextMenu(a, items, sx, sy)
	}
}
