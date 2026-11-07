package app

import (
	"slices"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) persistSidebarPanelOrder(ids []string) {
	a.State.SidebarPanelOrder = slices.Clone(ids)
	if err := config.SaveState(a.State); err != nil {
		a.StatusError(i18n.Format("status.failedToSaveSidebarOrder", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) MoveActiveSidebarPanel(dir int) {
	a.Sidebar.MoveActivePanel(dir)
}

func (a *App) sidebarMoveMenuItems() []ui.ContextMenuItem {
	var items []ui.ContextMenuItem
	if a.Sidebar.CanMoveActivePanel(-1) {
		items = append(items, ui.ContextMenuItem{Label: i18n.Text("menu.movePanelLeft"), Command: "sidebar.movePanelLeft"})
	}
	if a.Sidebar.CanMoveActivePanel(1) {
		items = append(items, ui.ContextMenuItem{Label: i18n.Text("menu.movePanelRight"), Command: "sidebar.movePanelRight"})
	}
	return items
}
