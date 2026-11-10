package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) editorLayoutKey() string {
	if a.Workspace == nil {
		return ""
	}
	if a.Workspace.FilePath != "" {
		return a.Workspace.FilePath
	}
	return strings.Join(a.Workspace.Paths(), "\n")
}

func (a *App) snapshotEditorLayout(node *ui.EditorLayoutWidget) *config.EditorLayoutState {
	if node.Group != nil {
		return &config.EditorLayoutState{Views: node.Group.SessionViews(), Focused: node.Group == a.EditorGroup}
	}
	direction := "right"
	if node.Split.Position == ui.SplitBottom {
		direction = "down"
	}
	return &config.EditorLayoutState{Direction: direction, Ratio: node.Split.BottomRatio,
		First: a.snapshotEditorLayout(node.First), Second: a.snapshotEditorLayout(node.Second)}
}

func (a *App) persistEditorLayout() {
	if a.restoringEditorLayout || a.EditorSplit == nil {
		return
	}
	key := a.editorLayoutKey()
	if key == "" {
		return
	}
	if len(a.editorGroups()) == 1 && a.State.EditorLayouts[key] == nil {
		return
	}
	if a.State.EditorLayouts == nil {
		a.State.EditorLayouts = make(map[string]*config.EditorLayoutState)
	}
	a.State.EditorLayouts[key] = a.snapshotEditorLayout(a.EditorSplit)
	if err := config.SaveState(a.State); err != nil {
		a.StatusError(i18n.Format("status.failedToSaveEditorLayout", i18n.Args{"error": err.Error()}))
	}
}

func validEditorLayout(state *config.EditorLayoutState, depth int) int {
	if state == nil || depth > ui.MaxEditorGroups {
		return 0
	}
	if state.First == nil && state.Second == nil {
		if state.Direction != "" {
			return 0
		}
		return 1
	}
	if state.Direction != "right" && state.Direction != "down" {
		return 0
	}
	first, second := validEditorLayout(state.First, depth+1), validEditorLayout(state.Second, depth+1)
	if first == 0 || second == 0 {
		return 0
	}
	return first + second
}

func (a *App) restoreEditorGroups() {
	a.EditorSplit.OnChanged = a.persistEditorLayout
	state := a.State.EditorLayouts[a.editorLayoutKey()]
	if !a.restoreEditorLayout || validEditorLayout(state, 0) == 0 || validEditorLayout(state, 0) > ui.MaxEditorGroups {
		return
	}
	a.restoringEditorLayout = true
	var focus *ui.EditorGroupWidget
	var restore func(*ui.EditorLayoutWidget, *config.EditorLayoutState)
	restore = func(node *ui.EditorLayoutWidget, saved *config.EditorLayoutState) {
		if saved.First != nil {
			position := ui.SplitRight
			if saved.Direction == "down" {
				position = ui.SplitBottom
			}
			next := a.newEditorGroupLike(node.Group)
			a.EditorSplit.SplitGroup(node.Group, next, position)
			node.Split.BottomRatio = saved.Ratio
			if node.Split.BottomRatio < 0.05 || node.Split.BottomRatio > 0.95 {
				node.Split.BottomRatio = 0.5
			}
			restore(node.First, saved.First)
			restore(node.Second, saved.Second)
			return
		}
		group, active := node.Group, ""
		for _, view := range saved.Views {
			if !filepath.IsAbs(view.Path) {
				continue
			}
			if info, err := os.Stat(view.Path); err != nil || !info.Mode().IsRegular() {
				continue
			}
			if err := group.OpenFile(view.Path); err != nil {
				continue
			}
			group.CommitActiveTab()
			group.GoToLineCol(max(view.Line, 0)+1, max(view.Col, 0)+1)
			if buf := group.ActiveBuffer(); buf != nil {
				group.Editor.Viewport.TopLine = max(0, min(view.TopLine, len(buf.Lines)-1))
				group.Editor.Viewport.LeftCol = max(view.LeftCol, 0)
			}
			if view.Pinned {
				group.TogglePinTab()
			}
			if view.Active {
				active = view.Path
			}
		}
		if active != "" {
			group.SwitchToTabByPath(active)
		}
		if saved.Focused {
			focus = group
		}
	}
	restore(a.EditorSplit, state)
	if focus != nil {
		a.Root.SetFocus(focus)
	}
	a.restoringEditorLayout = false
}
