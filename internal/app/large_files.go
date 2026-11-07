package app

import (
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
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
		ID: "reload", Side: "left", Priority: 170, Text: string(icons.Rune(icons.Reload)) + " " + i18n.Text("general.reload"), OnClick: a.ReloadActiveFile,
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
		a.ShowConfirmDialog(i18n.Format("largeFiles.discardUnsavedChangesTo", i18n.Args{"filename": filepath.Base(path)}), []string{i18n.Text("general.reload"), i18n.Text("general.cancel")}, []func(){
			func() { a.DismissDialog(); a.reloadFile(path) },
			a.DismissDialog,
		})
		return
	}
	a.reloadFile(path)
}
