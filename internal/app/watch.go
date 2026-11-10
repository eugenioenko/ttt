package app

import (
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/watcher"

	"github.com/gdamore/tcell/v3"
)

// FileChangedResult is posted to the event loop when a watched file changes on
// disk. It carries the path as tracked by the editor.
type FileChangedResult struct {
	Path string
}

// ExplorerDirChangedResult is posted to the event loop when a directory shown
// in the workspace explorer gains, loses or renames an entry on disk.
type ExplorerDirChangedResult struct {
	Dir string
}

// StartWatcher creates the file watcher. The callbacks run on the watcher's
// goroutine, so they only post an event; reconciliation happens on the main
// loop. a.Screen is read at call time so it is safe even if the screen is
// wired up after Init.
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

// SyncWatched points the watcher at the currently open files and the
// directories currently shown in the explorer. Cheap to call frequently.
func (a *App) SyncWatched() {
	if a.Watcher == nil {
		return
	}
	var paths []string
	for _, group := range a.editorGroups() {
		paths = append(paths, group.OpenFilePaths()...)
	}
	a.Watcher.Sync(paths)
	if a.Explorer != nil {
		a.Watcher.SyncDirs(a.Explorer.WatchedDirs())
	}
}

// HandleFileChanged reconciles an open buffer with a change detected on disk.
// A clean buffer is reloaded silently up to the size limit; a buffer with unsaved edits is left
// untouched (the save path still guards against clobbering) and the user is
// warned. The recorded disk state of a dirty buffer is deliberately not
// updated, so the save-time conflict check keeps working.
// A missing file is reported without changing the buffer contents.
func (a *App) HandleFileChanged(path string) {
	a.handleFileChanged(path, os.Stat)
}

func (a *App) handleFileChanged(path string, stat func(string) (os.FileInfo, error)) {
	a.invalidateRepositoryPath(path, RepositoryWorktree)
	var buf *buffer.Buffer
	for _, group := range a.editorGroups() {
		if buf = group.BufferForPath(path); buf != nil {
			break
		}
	}
	if buf == nil {
		return
	}
	name := filepath.Base(path)
	info, err := stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			a.StatusWarn(i18n.Format("status.wasDeletedOnDisk", i18n.Args{"name": name}))
		} else {
			a.StatusWarn(i18n.Format("status.failedToCheck", i18n.Args{"name": name, "error": err}))
		}
		return
	}
	if !buf.DiskChanged(path) {
		// Our own save, or a change that doesn't affect the bytes we hold.
		return
	}
	if buf.Dirty {
		a.StatusWarn(i18n.Format("status.changedOnDiskYouHaveUnsavedChanges", i18n.Args{"name": name}))
		return
	}
	if limit := a.Settings.Editor.AutoReloadMaxBytes(); limit > 0 && info.Size() > limit {
		if a.largeFiles.stale == nil {
			a.largeFiles.stale = make(map[string]bool)
		}
		a.largeFiles.stale[path] = true
		a.SyncReloadSegment()
		return
	}
	a.reloadFile(path)
}

func (a *App) reloadFile(path string) {
	reloaded := false
	for _, group := range a.editorGroups() {
		reloaded = group.ReloadFile(path) || reloaded
	}
	if !reloaded {
		return
	}
	delete(a.largeFiles.stale, path)
	a.SyncReloadSegment()
	a.RequestGitGutterForActiveFile()
	a.RefreshSymbols()
}

// HandleExplorerDirChanged re-reads the explorer tree after its backing
// filesystem changed. Reload preserves expansion state and selection.
func (a *App) HandleExplorerDirChanged(dir string) {
	a.invalidateRepositoryPath(dir, RepositoryWorktree)
	if a.Explorer != nil {
		a.Explorer.Reload()
		a.SyncWatched()
	}
}
