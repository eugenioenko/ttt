package app

import (
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/watcher"

	"github.com/gdamore/tcell/v3"
)

// FileChangedResult is posted to the event loop when a watched file changes on
// disk. It carries the path as tracked by the editor.
type FileChangedResult struct {
	Path string
}

// StartWatcher creates the file watcher. onChange runs on the watcher's
// goroutine, so it only posts an event; the reconciliation happens on the main
// loop in HandleFileChanged. a.Screen is read at call time so it is safe even
// if the screen is wired up after Init.
func (a *App) StartWatcher() {
	w, err := watcher.New(func(path string) {
		if a.Screen != nil {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&FileChangedResult{Path: path}))
		}
	})
	if err != nil {
		return
	}
	a.Watcher = w
}

// SyncWatched updates the watcher's tracked set to the currently open files.
// It is cheap to call frequently — the watcher ignores paths it already tracks.
func (a *App) SyncWatched() {
	if a.Watcher == nil {
		return
	}
	a.Watcher.Sync(a.EditorGroup.OpenFilePaths())
}

// HandleFileChanged reconciles an open buffer with a change detected on disk.
// A clean buffer is reloaded silently; a buffer with unsaved edits is left
// untouched (the save path still guards against clobbering) and the user is
// warned. The recorded disk state of a dirty buffer is deliberately not
// updated, so the save-time conflict check keeps working.
func (a *App) HandleFileChanged(path string) {
	a.invalidateRepositoryPath(path, RepositoryWorktree)
	buf := a.EditorGroup.BufferForPath(path)
	if buf == nil {
		return
	}
	if !buf.DiskChanged(path) {
		// Our own save, or a change that doesn't affect the bytes we hold.
		return
	}
	name := filepath.Base(path)
	if _, err := os.Stat(path); err != nil {
		a.StatusWarn(name + " was deleted on disk")
		return
	}
	if buf.Dirty {
		a.StatusWarn(name + " changed on disk; you have unsaved changes")
		return
	}
	// Reload silently: a clean buffer picking up disk changes is the routine
	// case (e.g. tailing a live log), and a notification on every change would
	// be noise. Only the cases needing attention (conflict, deletion) warn.
	a.EditorGroup.ReloadFile(path)
	a.RequestGitGutterForActiveFile()
	a.RefreshSymbols()
}
