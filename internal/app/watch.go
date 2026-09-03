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

// ExplorerDirChangedResult is posted to the event loop when the contents of a
// directory shown in the workspace explorer change on disk.
type ExplorerDirChangedResult struct {
	Dir string
}

// StartWatcher creates the file watcher. The callbacks run on the watcher's
// goroutine, so they only post an event; the reconciliation happens on the main
// loop in HandleFileChanged / HandleExplorerDirChanged. a.Screen is read at
// call time so it is safe even if the screen is wired up after Init.
func (a *App) StartWatcher() {
	w, err := watcher.New(
		func(path string) {
			if a.Screen != nil {
				a.Screen.PostEvent(tcell.NewEventInterrupt(&FileChangedResult{Path: path}))
			}
		},
		func(dir string) {
			if a.Screen != nil {
				a.Screen.PostEvent(tcell.NewEventInterrupt(&ExplorerDirChangedResult{Dir: dir}))
			}
		},
	)
	if err != nil {
		return
	}
	a.Watcher = w
}

// SyncWatched updates the watcher's tracked set to the currently open files and
// the directories currently visible in the explorer. It is cheap to call
// frequently — the watcher ignores paths it already tracks.
func (a *App) SyncWatched() {
	if a.Watcher == nil {
		return
	}
	a.Watcher.Sync(a.EditorGroup.OpenFilePaths())
	if a.Explorer != nil {
		a.Watcher.SyncDirs(a.Explorer.WatchedDirs())
	}
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

// HandleExplorerDirChanged re-reads the explorer tree after its backing
// filesystem changed. Reload preserves expansion state and selection.
func (a *App) HandleExplorerDirChanged(dir string) {
	a.invalidateRepositoryPath(dir, RepositoryWorktree)
	if a.Explorer != nil {
		a.Explorer.Reload()
	}
}
