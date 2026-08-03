package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
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
	a.ShowInputDialog("New File", "Filename", "", func(name string) {
		newPath := filepath.Join(parentDir, name)
		if _, err := os.Stat(newPath); err == nil {
			a.StatusError(name + " already exists")
			return
		}
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
		if err := os.WriteFile(newPath, []byte{}, 0644); err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
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
	a.ShowInputDialog("New Folder", "Folder name", "", func(name string) {
		newPath := filepath.Join(parentDir, name)
		if err := os.MkdirAll(newPath, 0755); err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
		reload()
	})
}

func (a *App) FileOpRename(path string, reload func()) {
	if path == "" {
		return
	}
	a.ShowInputDialog("Rename", "New name", filepath.Base(path), func(newName string) {
		newPath := filepath.Join(filepath.Dir(path), newName)
		if newPath == path {
			return
		}
		if newInfo, err := os.Stat(newPath); err == nil {
			oldInfo, oldErr := os.Stat(path)
			if oldErr != nil || !os.SameFile(newInfo, oldInfo) {
				a.StatusError(newName + " already exists")
				return
			}
		}
		if err := os.Rename(path, newPath); err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
		a.EditorGroup.RenamePath(path, newPath)
		reload()
	})
}

func (a *App) FileOpDelete(path string, reload func()) {
	if path == "" {
		return
	}
	name := filepath.Base(path)
	a.ShowConfirmDialog("Delete "+name+"?",
		[]string{"No", "Yes"},
		[]func(){
			func() { a.DismissDialog() },
			func() {
				a.DismissDialog()
				if err := os.RemoveAll(path); err != nil {
					a.StatusError("Error: " + err.Error())
					return
				}
				reload()
			},
		},
	)
}

func (a *App) FileOpCopyAbsolutePath(path string) {
	if path == "" {
		a.StatusWarn("No file selected")
		return
	}
	clipboard.Set(path)
	a.StatusNotify("Absolute path copied to clipboard")
}

func (a *App) FileOpReveal(path string) {
	if path == "" {
		a.StatusWarn("No file selected")
		return
	}
	if err := revealInFileManager(path); err != nil {
		a.StatusError("Could not reveal in file manager")
		return
	}
	a.StatusNotify("Revealed in file manager")
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
		uri := "file://" + path
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
		a.StatusWarn("No file selected")
		return
	}
	rel := a.relativePath(path)
	clipboard.Set(rel)
	a.StatusNotify("Relative path copied to clipboard")
}

func (a *App) FileOpRemoveRoot(path string) {
	if path == "" {
		return
	}
	if len(a.Workspace.Paths()) <= 1 {
		a.StatusWarn("Cannot remove the last folder")
		return
	}
	a.Workspace.RemoveFolder(path)
	a.refreshWorkspaceWidgets()
}
