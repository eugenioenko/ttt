package app

import (
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/view"
)

type largeFileState struct {
	stale map[string]bool
}

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
	a.Status.SetSegment(view.StatusSegment{
		ID: "reload", Side: "left", Priority: 170, Text: "↻ Reload", OnClick: a.ReloadActiveFile,
	})
}

func (a *App) ReloadActiveFile() {
	if a.Root.HasOverlay() {
		return
	}
	path := a.EditorGroup.ActiveFilePath()
	buf := a.EditorGroup.ActiveBuffer()
	if buf == nil || a.EditorGroup.IsActiveVirtual() {
		return
	}
	if buf.Dirty {
		a.ShowConfirmDialog("Discard unsaved changes to "+filepath.Base(path)+"?", []string{"Reload", "Cancel"}, []func(){
			func() { a.DismissDialog(); a.reloadFile(path) },
			a.DismissDialog,
		})
		return
	}
	a.reloadFile(path)
}
