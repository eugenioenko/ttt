package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/trash"
)

func (a *App) FileOpNewFile(path string, reload func()) {
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	parentDir := path
	if err != nil || !info.IsDir() {
		parentDir = filepath.Dir(path)
	}
	a.ShowInputDialog(i18n.Text("general.newFile"), i18n.Text("general.filename"), "", func(name string) {
		newPath := filepath.Join(parentDir, name)
		if _, err := os.Stat(newPath); err == nil {
			a.StatusError(i18n.Format("status.fileAlreadyExists", i18n.Args{"name": name}))
			return
		}
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
			return
		}
		if err := os.WriteFile(newPath, []byte{}, 0644); err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
			return
		}
		a.invalidateRepositoryPath(newPath, RepositoryWorktree)
		reload()
		a.EditorGroup.OpenFile(newPath)
		a.FocusEditor()
	})
}

func (a *App) FileOpNewFolder(path string, reload func()) {
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	parentDir := path
	if err != nil || !info.IsDir() {
		parentDir = filepath.Dir(path)
	}
	a.ShowInputDialog(i18n.Text("general.newFolder"), i18n.Text("general.folderName"), "", func(name string) {
		newPath := filepath.Join(parentDir, name)
		if err := os.MkdirAll(newPath, 0755); err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
			return
		}
		reload()
	})
}

func (a *App) FileOpRename(path string, reload func()) {
	if path == "" {
		return
	}
	a.ShowInputDialog(i18n.Text("general.rename"), i18n.Text("general.newName"), filepath.Base(path), func(newName string) {
		newPath := filepath.Join(filepath.Dir(path), newName)
		if newPath == path {
			return
		}
		if newInfo, err := os.Stat(newPath); err == nil {
			oldInfo, oldErr := os.Stat(path)
			if oldErr != nil || !os.SameFile(newInfo, oldInfo) {
				a.StatusError(i18n.Format("status.renamedFileAlreadyExists", i18n.Args{"newName": newName}))
				return
			}
		}
		if err := os.Rename(path, newPath); err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
			return
		}
		a.invalidateRepositoryPath(path, RepositoryWorktree)
		a.invalidateRepositoryPath(newPath, RepositoryWorktree)
		a.EditorGroup.RenamePath(path, newPath)
		reload()
	})
}

func (a *App) FileOpDelete(path string, reload func()) {
	a.confirmFileDelete(path, reload, trash.MoveContext)
}

func (a *App) confirmFileDelete(path string, reload func(), moveToTrash func(context.Context, string) error) {
	if path == "" {
		return
	}
	name := filepath.Base(path)
	useTrash := a.Settings.Explorer.UseTrash
	message := i18n.Format("files.delete", i18n.Args{"name": name})
	if useTrash {
		message = i18n.Format("files.moveToTrash", i18n.Args{"name": name})
	}
	a.ShowConfirmDialog(message,
		[]string{i18n.Text("general.no"), i18n.Text("general.yes")},
		[]func(){
			a.DismissDialog,
			func() {
				a.DismissDialog()
				a.startFileDelete(path, useTrash, reload, moveToTrash)
			},
		},
	)
}

func (a *App) startFileDelete(path string, useTrash bool, reload func(), moveToTrash func(context.Context, string) error) {
	task := NewTask[struct{}](a, asynctask.TaskOptions{})
	task.Schedule(func(ctx context.Context) (struct{}, error) {
		if err := ctx.Err(); err != nil {
			return struct{}{}, err
		}
		if useTrash {
			return struct{}{}, moveToTrash(ctx, path)
		}
		return struct{}{}, os.RemoveAll(path)
	}, func(_ struct{}, err error) {
		defer a.releaseTask(task)
		a.completeFileDelete(path, useTrash, reload, err)
	})
}

func (a *App) completeFileDelete(path string, useTrash bool, reload func(), err error) {
	name := filepath.Base(path)
	if err != nil {
		message := i18n.Format("status.error", i18n.Args{"error": err.Error()})
		if useTrash {
			message = i18n.Format("files.trashFailed", i18n.Args{"name": name, "error": err.Error()})
		}
		a.StatusError(message)
		return
	}
	a.invalidateRepositoryPath(path, RepositoryWorktree)
	reload()
	message := i18n.Format("files.deletedPermanently", i18n.Args{"name": name})
	if useTrash {
		message = i18n.Format("files.movedToTrash", i18n.Args{"name": name})
	}
	a.StatusNotify(message)
}

func (a *App) FileOpCopyAbsolutePath(path string) {
	if path == "" {
		a.StatusWarn(i18n.Text("status.noFileSelected"))
		return
	}
	clipboard.Set(path)
	a.StatusNotify(i18n.Text("status.absolutePathCopiedToClipboard"))
}

func (a *App) FileOpReveal(path string) {
	if path == "" {
		a.StatusWarn(i18n.Text("status.noFileSelected"))
		return
	}
	if err := revealInFileManager(path); err != nil {
		a.StatusError(i18n.Text("status.couldNotRevealInFileManager"))
		return
	}
	a.StatusNotify(i18n.Text("status.revealedInFileManager"))
}

// revealInFileManager opens the OS file manager showing path, highlighting the
// item where the platform supports it. On Linux it falls back to opening the
// containing folder (no highlight) when the FileManager1 D-Bus service is
// unavailable.
func revealInFileManager(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer", "/select,"+path).Start()
	default:
		uri := FileURI(path)
		dbus := exec.Command("dbus-send", "--session",
			"--dest=org.freedesktop.FileManager1", "--type=method_call",
			"/org/freedesktop/FileManager1",
			"org.freedesktop.FileManager1.ShowItems",
			"array:string:"+uri, "string:")
		if err := dbus.Run(); err == nil {
			return nil
		}
		return exec.Command("xdg-open", containingFolder(path)).Start()
	}
}

// containingFolder returns the folder to open for a fallback reveal: the path
// itself if it is a directory, otherwise its parent directory.
func containingFolder(path string) string {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

func (a *App) FileOpCopyRelativePath(path string) {
	if path == "" {
		a.StatusWarn(i18n.Text("status.noFileSelected"))
		return
	}
	rel := a.relativePath(path)
	clipboard.Set(rel)
	a.StatusNotify(i18n.Text("status.relativePathCopiedToClipboard"))
}

func (a *App) FileOpRemoveRoot(path string) {
	if path == "" {
		return
	}
	a.Workspace.RemoveFolder(path)
	a.refreshWorkspaceWidgets()
}
