package app

import (
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) editorGroupMenu() []ui.ContextMenuItem {
	var items []ui.ContextMenuItem
	if len(a.editorGroups()) < ui.MaxEditorGroups {
		items = append(items,
			ui.ContextMenuItem{Label: i18n.Text("menu.splitRight"), Command: "editor.split"},
			ui.ContextMenuItem{Label: i18n.Text("menu.splitDown"), Command: "editor.splitDown"})
	}
	if len(a.editorGroups()) > 1 {
		items = append(items,
			ui.ContextMenuItem{Label: i18n.Text("menu.closeSplit"), Command: "editor.closeSplit"})
	}
	return items
}
