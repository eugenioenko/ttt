package app

import (
	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) updateAutoHiddenTabs(x, y int) bool {
	changed := false
	for _, group := range a.editorGroups() {
		if group.TabBar.UpdateAutoHide(x, y) {
			changed = true
		}
	}
	return changed
}

func (a *App) wireEditorTabAutoHide(group *ui.EditorGroupWidget) {
	group.TabBar.AutoShowTask = NewTask[bool](a, asynctask.TaskOptions{Delay: ui.TabBarAutoShowDelay})
	group.TabBar.AutoHideTask = NewTask[bool](a, asynctask.TaskOptions{Delay: ui.TabBarAutoHideDelay})
	group.TabBar.AutoHideBlocked = func() bool { return a.Root.HasOverlay() || a.Root.PointerCaptureActive() }
	group.TabBar.OnAutoHideChange = a.DismissHover
}
