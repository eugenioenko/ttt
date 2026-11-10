package app

import (
	"os"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) editorSideGroup() *ui.EditorGroupWidget {
	if len(a.editorGroups()) == 1 {
		g := a.newEditorGroupLike(a.EditorGroup)
		a.EditorSplit.SplitGroup(a.EditorGroup, g, ui.SplitRight)
		return g
	}
	if right := a.EditorSplit.Neighbor(a.EditorGroup, 1, 0); right != nil {
		return right
	}
	groups := a.editorGroups()
	for i, group := range groups {
		if group == a.EditorGroup {
			return groups[(i+1)%len(groups)]
		}
	}
	return nil
}

func (a *App) OpenEditorToSide() {
	if a.Root.HasOverlay() {
		return
	}
	a.FlushEditorOnChange()
	source := a.EditorGroup
	target := a.editorSideGroup()
	if target != nil && source.CloneActiveTabTo(target) {
		a.Root.SetFocus(target)
	}
}

func (a *App) ExplorerOpenToSide() {
	path := a.explorerNodePath()
	a.ExplorerContextNode = nil
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return
	}
	a.FlushEditorOnChange()
	if target := a.editorSideGroup(); target != nil {
		a.Root.SetFocus(target)
		if err := target.OpenFile(path); err != nil {
			a.StatusError(err.Error())
		}
	}
}

func (a *App) MoveEditorToOtherGroup() {
	if a.Root.HasOverlay() || len(a.editorGroups()) < 2 {
		return
	}
	a.FlushEditorOnChange()
	source := a.EditorGroup
	if target := a.editorSideGroup(); target != nil {
		a.moveEditorTab(source, target)
	}
}

func (a *App) moveEditorTab(source, target *ui.EditorGroupWidget) {
	a.activateEditorGroup(source)
	a.FlushEditorOnChange()
	source.MoveActiveTabTo(target)
	a.Root.SetFocus(target)
}

func (a *App) wireEditorTabDrag(group *ui.EditorGroupWidget) {
	group.TabBar.OnTabDrag = func(_ int, x, y int) {
		for _, other := range a.editorGroups() {
			other.DropTarget = other != group && other.GetRect().Contains(x, y)
		}
	}
	group.TabBar.OnTabDragCancel = a.clearEditorDropTargets
	group.TabBar.OnTabDrop = func(index, x, y int) bool {
		a.clearEditorDropTargets()
		for _, other := range a.editorGroups() {
			if other != group && other.GetRect().Contains(x, y) {
				dropIndex := other.TabBar.DropIndexAt(x, y)
				group.SwitchTab(index)
				a.moveEditorTab(group, other)
				if dropIndex >= 0 {
					other.MoveTab(other.ActiveTabIndex(), dropIndex)
				}
				return true
			}
		}
		return !group.TabBar.GetRect().Contains(x, y)
	}
}

func (a *App) clearEditorDropTargets() {
	for _, g := range a.editorGroups() {
		g.DropTarget = false
	}
}

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

func registerEditorGroupCommands(a *App) {
	a.Reg.Register(command.Command{ID: "editor.openToSide", Title: i18n.Text("command.openEditorToSide"), Handler: a.OpenEditorToSide})
	a.Reg.Register(command.Command{ID: "editor.moveToOtherGroup", Title: i18n.Text("command.moveEditorToOtherGroup"), Handler: a.MoveEditorToOtherGroup})
	a.Reg.Register(command.Command{ID: "explorer.openToSide", Title: i18n.Text("command.explorerOpenToSide"), Handler: a.ExplorerOpenToSide})
}
