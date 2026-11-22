package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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
		for _, group := range a.editorGroups() {
			group.RenamePath(path, newPath)
		}
		reload()
	})
}

func (a *App) FileOpDuplicate(path string, reload func()) {
	if path == "" {
		return
	}
	info, err := os.Lstat(path)
	if err != nil {
		a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
		return
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		a.StatusWarn(i18n.Text("status.cannotDuplicateItem"))
		return
	}
	newPath, err := nextDuplicatePath(path)
	if err != nil {
		a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
		return
	}
	if info.IsDir() {
		err = copyTree(path, newPath)
	} else {
		err = copyRegularFile(path, newPath, info.Mode())
	}
	if err != nil {
		a.StatusError(fileOpDuplicateMessage(err))
		return
	}
	a.invalidateRepositoryPath(newPath, RepositoryWorktree)
	reload()
	a.StatusNotify(i18n.Format("files.duplicated", i18n.Args{"name": filepath.Base(newPath)}))
}

type unsupportedFileTypeError struct{ path string }

func (e unsupportedFileTypeError) Error() string {
	return "unsupported file type: " + e.path
}

func fileOpDuplicateMessage(err error) string {
	var unsupported unsupportedFileTypeError
	if !errors.As(err, &unsupported) {
		return i18n.Format("status.error", i18n.Args{"error": err.Error()})
	}
	msg := i18n.Format("files.unsupportedType", i18n.Args{"path": unsupported.path})
	for _, part := range flattenJoinedErrors(err) {
		if _, ok := part.(unsupportedFileTypeError); ok {
			continue
		}
		msg += "\n" + part.Error()
	}
	return msg
}

func flattenJoinedErrors(err error) []error {
	if err == nil {
		return nil
	}
	if joiner, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, e := range joiner.Unwrap() {
			out = append(out, flattenJoinedErrors(e)...)
		}
		return out
	}
	return []error{err}
}

// copyTree duplicates a directory recursively. Symlinks are recreated as
// symlinks rather than followed, so a link cannot pull files from outside the
// source tree into the duplicate.
func copyTree(src, dst string) (err error) {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	// A read-only source would otherwise produce a destination its children cannot be copied into.
	if err := os.Mkdir(dst, info.Mode().Perm()|0o700); err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		if cleanupErr := restoreDirWriteAccess(dst); cleanupErr != nil {
			err = errors.Join(err, errors.New(i18n.Format("files.duplicateRestoreFailed", i18n.Args{
				"path":  dst,
				"error": cleanupErr.Error(),
			})))
		}
		if cleanupErr := os.RemoveAll(dst); cleanupErr != nil {
			err = errors.Join(err, errors.New(i18n.Format("files.duplicateCleanupFailed", i18n.Args{
				"path":  dst,
				"error": cleanupErr.Error(),
			})))
		}
	}()
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return err
			}
		case entry.IsDir():
			if err := copyTree(srcPath, dstPath); err != nil {
				return err
			}
		case entry.Type().IsRegular():
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := copyRegularFile(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		default:
			return unsupportedFileTypeError{path: srcPath}
		}
	}
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return err
	}
	return nil
}

func restoreDirWriteAccess(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.Chmod(root, info.Mode().Perm()|0o700); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := restoreDirWriteAccess(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// nextDuplicatePath mirrors the editor's "file copy.txt", "file copy 2.txt"
// naming, including dotfiles where filepath.Ext returns the whole basename.
func nextDuplicatePath(path string) (string, error) {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem == "" {
		stem, ext = name, ""
	}
	for n := 1; ; n++ {
		suffix := " copy"
		if n > 1 {
			suffix = fmt.Sprintf(" copy %d", n)
		}
		candidate := filepath.Join(dir, stem+suffix+ext)
		_, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		if n == 1000 {
			return "", fmt.Errorf("no free duplicate name for %q", name)
		}
	}
}

func copyRegularFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	if err := os.Chmod(dst, mode.Perm()); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
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
	if !useTrash {
		// RemoveAll cannot be cancelled; finish before the event loop can quit.
		a.completeFileDelete(path, false, reload, os.RemoveAll(path))
		return
	}
	task := NewTask[struct{}](a, asynctask.TaskOptions{})
	task.Schedule(func(ctx context.Context) (struct{}, error) {
		if err := ctx.Err(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, moveToTrash(ctx, path)
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
