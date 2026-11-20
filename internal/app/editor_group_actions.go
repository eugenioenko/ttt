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
	if !source.IsEditorActive() {
		return
	}
	target := a.editorSideGroup()
	if target != nil && source.CloneActiveTabTo(target) {
		a.Root.SetFocus(target)
	}
}

func (a *App) ExplorerOpenToSide() {
	if a.Root.HasOverlay() {
		return
	}
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

func (a *App) editorDropTargetAt(source *ui.EditorGroupWidget, x, y int) (*ui.EditorGroupWidget, ui.EditorDropZone) {
	groups := a.editorGroups()
	for _, target := range groups {
		if !target.GetRect().Contains(x, y) {
			continue
		}
		sameGroup := target == source
		canSplit := len(groups) < ui.MaxEditorGroups && !(sameGroup && source.TabCount() <= 1)
		return target, target.DropZoneAt(x, y, sameGroup, canSplit)
	}
	return nil, ui.EditorDropNone
}

func (a *App) wireEditorTabDrag(group *ui.EditorGroupWidget) {
	group.TabBar.OnTabDrag = func(_ int, x, y int) {
		target, zone := a.editorDropTargetAt(group, x, y)
		for _, other := range a.editorGroups() {
			if other == target {
				other.SetDropZone(zone)
			} else {
				other.SetDropZone(ui.EditorDropNone)
			}
		}
	}
	group.TabBar.OnTabDragCancel = a.clearEditorDropTargets
	group.TabBar.OnTabDrop = func(index, x, y int) bool {
		a.clearEditorDropTargets()
		target, zone := a.editorDropTargetAt(group, x, y)
		if target != nil && zone != ui.EditorDropNone {
			if zone.IsSplit() {
				if len(a.editorGroups()) < ui.MaxEditorGroups {
					next := a.newEditorGroupLike(target)
					if a.EditorSplit.SplitGroupAt(target, next, zone) {
						group.SwitchTab(index)
						a.moveEditorTab(group, next)
					}
				}
				return true
			}
			if target != group {
				dropIndex := target.TabBar.DropIndexAt(x, y)
				before := target.TabCount()
				group.SwitchTab(index)
				a.moveEditorTab(group, target)
				if dropIndex >= 0 {
					if target.TabCount() == before && target.ActiveTabIndex() < dropIndex {
						dropIndex--
					}
					target.MoveTab(target.ActiveTabIndex(), min(dropIndex, target.TabCount()-1))
				}
				return true
			}
		}
		tabs := group.TabBar.GetRect()
		return y < tabs.Y || y >= tabs.Y+tabs.H
	}
}

func (a *App) clearEditorDropTargets() {
	for _, g := range a.editorGroups() {
		g.SetDropZone(ui.EditorDropNone)
	}
}

func registerEditorGroupCommands(a *App) {
	a.Reg.Register(command.Command{ID: "editor.openToSide", Title: i18n.Text("command.openEditorToSide"), Handler: a.OpenEditorToSide})
	a.Reg.Register(command.Command{ID: "editor.moveToOtherGroup", Title: i18n.Text("command.moveEditorToOtherGroup"), Handler: a.MoveEditorToOtherGroup})
	a.Reg.Register(command.Command{ID: "explorer.openToSide", Title: i18n.Text("command.explorerOpenToSide"), Handler: a.ExplorerOpenToSide})
}
