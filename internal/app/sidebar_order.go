package app

import (
	"slices"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) persistSidebarPanelOrder(ids []string) {
	a.State.SidebarPanelOrder = slices.Clone(ids)
	if err := config.SaveState(a.State); err != nil {
		a.StatusError("Failed to save sidebar order: " + err.Error())
	}
}

func (a *App) MoveActiveSidebarPanel(dir int) {
	a.Sidebar.MoveActivePanel(dir)
}

func (a *App) sidebarMoveMenuItems() []ui.ContextMenuItem {
	var items []ui.ContextMenuItem
	if a.Sidebar.CanMoveActivePanel(-1) {
		items = append(items, ui.ContextMenuItem{Label: "Move Panel Left", Command: "sidebar.movePanelLeft"})
	}
	if a.Sidebar.CanMoveActivePanel(1) {
		items = append(items, ui.ContextMenuItem{Label: "Move Panel Right", Command: "sidebar.movePanelRight"})
	}
	return items
}
