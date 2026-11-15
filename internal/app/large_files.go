package app

import (
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type largeFileState struct {
	stale map[string]bool
}

func (a *App) SyncReloadSegment() {
	for path := range a.largeFiles.stale {
		buf := a.editorBufferForPath(path)
		if buf == nil || !buf.DiskChanged(path) {
			delete(a.largeFiles.stale, path)
		}
	}
	if !a.largeFiles.stale[a.EditorGroup.ActiveFilePath()] {
		a.Status.RemoveSegment("reload")
		return
	}
	a.Status.SetSegment(view.StatusSegment{
		ID: "reload", Side: "left", Priority: 170, Text: string(icons.Rune(icons.Reload)) + " " + i18n.Text("general.reload"), OnClick: a.ShowStaleFileActions,
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

func (a *App) ShowStaleFileActions() {
	if a.Root.HasOverlay() {
		return
	}
	path := a.EditorGroup.ActiveFilePath()
	a.ShowSelectDialog(i18n.Text("logMode.staleActions"), []widgets.SelectItem{
		{ID: "reload", Label: i18n.Text("general.reload")},
		{ID: "log", Label: i18n.Text("logMode.open")},
	}, func(id string) {
		if a.EditorGroup.ActiveFilePath() != path {
			return
		}
		if id == "log" {
			a.OpenActiveInLogMode()
		} else {
			a.ReloadActiveFile()
		}
	}, nil)
}
