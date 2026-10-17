package app

import (
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type largeFileState struct{ stale map[string]bool }

func (a *App) SyncReloadSegment() {
	for path := range a.largeFiles.stale {
		buf := a.EditorGroup.BufferForPath(path)
		if buf == nil || !buf.DiskChanged(path) {
			delete(a.largeFiles.stale, path)
		}
	}
	if !a.largeFiles.stale[a.EditorGroup.ActiveFilePath()] {
		a.Status.RemoveSegment("reload")
		return
	}
	a.Status.SetSegment(view.StatusSegment{ID: "reload", Side: "left", Priority: 160, Text: "↻ Reload", OnClick: a.ShowStaleFileActions})
}
func (a *App) ShowStaleFileActions() {
	if a.Root.HasOverlay() {
		return
	}
	path := a.EditorGroup.ActiveFilePath()
	a.ShowSelectDialog("File changed on disk", []widgets.SelectItem{{ID: "reload", Label: "Reload"}, {ID: "log", Label: "Open in Log Mode"}}, func(id string) {
		if id == "log" {
			a.openLogView(path)
		} else if a.EditorGroup.ActiveFilePath() == path {
			a.ReloadActiveFile()
		}
	}, nil)
}
func (a *App) ReloadActiveFile() {
	if a.Root.HasOverlay() {
		return
	}
	if a.activeLogView() != nil {
		return
	}
	path := a.EditorGroup.ActiveFilePath()
	buf := a.EditorGroup.ActiveBuffer()
	if buf == nil || a.EditorGroup.IsActiveVirtual() {
		return
	}
	if buf.Dirty {
		a.ShowConfirmDialog("Discard unsaved changes to "+filepath.Base(path)+"?", []string{"Reload", "Cancel"}, []func(){func() { a.DismissDialog(); a.reloadFile(path) }, a.DismissDialog})
	} else {
		a.reloadFile(path)
	}
}
