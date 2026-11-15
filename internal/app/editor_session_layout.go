package app

import (
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) snapshotEditorSessionLayout(node *ui.EditorLayoutWidget) *config.EditorLayoutState {
	if node.Group != nil {
		var tabs []config.EditorTabState
		for _, tab := range node.Group.SessionTabs() {
			path, err := filepath.Abs(tab.Path)
			if err != nil {
				continue
			}
			if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
				continue
			}
			tab.Path = path
			tabs = append(tabs, tab)
		}
		return &config.EditorLayoutState{Tabs: tabs, Focused: node.Group == a.EditorGroup}
	}
	direction := "right"
	if node.Split.Position == ui.SplitBottom {
		direction = "down"
	}
	return &config.EditorLayoutState{Direction: direction, Ratio: node.Split.BottomRatio,
		First: a.snapshotEditorSessionLayout(node.First), Second: a.snapshotEditorSessionLayout(node.Second)}
}

func (a *App) restoreEditorSessionLayout(saved *config.EditorLayoutState) {
	focus := a.EditorGroup
	var restore func(*ui.EditorLayoutWidget, *config.EditorLayoutState)
	restore = func(node *ui.EditorLayoutWidget, state *config.EditorLayoutState) {
		if state.First != nil {
			position := ui.SplitRight
			if state.Direction == "down" {
				position = ui.SplitBottom
			}
			next := a.newEditorGroupLike(node.Group)
			a.EditorSplit.SplitGroup(node.Group, next, position)
			node.Split.BottomRatio = state.Ratio
			if state.Ratio < 0.05 || state.Ratio > 0.95 {
				node.Split.BottomRatio = 0.5
			}
			restore(node.First, state.First)
			restore(node.Second, state.Second)
			return
		}
		a.activateEditorGroup(node.Group)
		a.restoreEditorGroupTabs(node.Group, state.Tabs)
		if state.Focused {
			focus = node.Group
		}
	}
	restore(a.EditorSplit, saved)
	a.activateEditorGroup(focus)
	a.Root.SetFocus(focus)
}

func (a *App) restoreEditorGroupTabs(group *ui.EditorGroupWidget, tabs []config.EditorTabState) {
	active := ""
	seen := map[string]bool{}
	for _, tab := range tabs {
		if !filepath.IsAbs(tab.Path) || seen[tab.Path] {
			continue
		}
		seen[tab.Path] = true
		if info, err := os.Stat(tab.Path); err != nil || !info.Mode().IsRegular() {
			continue
		}
		if err := group.OpenFile(tab.Path); err != nil {
			continue
		}
		group.CommitActiveTab()
		group.GoToLineCol(max(tab.Line, 0)+1, max(tab.Col, 0)+1)
		if buf := group.ActiveBuffer(); buf != nil {
			group.Editor.Viewport.TopLine = max(0, min(tab.TopLine, len(buf.Lines)-1))
			group.Editor.Viewport.LeftCol = max(tab.LeftCol, 0)
		}
		if tab.Pinned != group.IsActiveTabPinned() {
			group.TogglePinTab()
		}
		if tab.Active {
			active = tab.Path
		}
	}
	if active != "" {
		group.SwitchToTabByPath(active)
	}
}
