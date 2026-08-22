package app

import (
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
)

func (a *App) explorerReload() {
	a.Explorer.Reload()
}

func (a *App) ExplorerNewFile() {
	a.FileOpNewFile(a.explorerNodePath(), a.explorerReload)
}

func (a *App) ExplorerNewFolder() {
	a.FileOpNewFolder(a.explorerNodePath(), a.explorerReload)
}

func (a *App) ExplorerRename() {
	a.FileOpRename(a.explorerNodePath(), a.explorerReload)
}

func (a *App) ExplorerDelete() {
	a.FileOpDelete(a.explorerNodePath(), a.explorerReload)
}

func (a *App) CopyAbsolutePath() {
	path := a.activeFilePath()
	if path == "" {
		a.StatusWarn("No file open")
		return
	}
	clipboard.Set(path)
	a.StatusNotify("Absolute path copied to clipboard")
}

func (a *App) CopyRelativePath() {
	path := a.activeFilePath()
	if path == "" {
		a.StatusWarn("No file open")
		return
	}
	rel := a.relativePath(path)
	clipboard.Set(rel)
	a.StatusNotify("Relative path copied to clipboard")
}

func (a *App) ExplorerCopyAbsolutePath() {
	path := a.explorerNodePath()
	a.ExplorerContextNode = nil
	a.FileOpCopyAbsolutePath(path)
}

func (a *App) ExplorerCopyRelativePath() {
	path := a.explorerNodePath()
	a.ExplorerContextNode = nil
	a.FileOpCopyRelativePath(path)
}

func (a *App) ExplorerReveal() {
	path := a.explorerNodePath()
	a.ExplorerContextNode = nil
	a.FileOpReveal(path)
}

func (a *App) ExplorerRemoveRoot() {
	path := a.explorerNodePath()
	a.ExplorerContextNode = nil
	a.FileOpRemoveRoot(path)
}

func (a *App) activeFilePath() string {
	path := a.EditorGroup.ActiveFilePath()
	if a.EditorGroup.IsActiveVirtual() {
		return ""
	}
	path = strings.TrimSuffix(path, " (diff)")
	if !filepath.IsAbs(path) {
		if len(a.Workspace.Folders) > 0 {
			path = filepath.Join(a.Workspace.Folders[0].Path, path)
		}
	}
	return path
}

func (a *App) explorerNodePath() string {
	if a.ExplorerContextNode != nil {
		return a.ExplorerContextNode.ID
	}
	if node := a.Explorer.Tree.Selected(); node != nil {
		return node.ID
	}
	return ""
}

func (a *App) relativePath(absPath string) string {
	if folder := a.Workspace.FolderForFile(absPath); folder != nil {
		if rel, err := filepath.Rel(folder.Path, absPath); err == nil {
			return rel
		}
	}
	primary := a.Workspace.Primary()
	if primary != "" {
		if rel, err := filepath.Rel(primary, absPath); err == nil {
			return rel
		}
	}
	return absPath
}

func registerExplorerCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "explorer.refresh", Title: "Explorer: Refresh",
		Keywords: []string{"view", "file", "reload"},
		Handler:  func() { app.Explorer.Reload() },
	})

	reg.Register(command.Command{
		ID: "explorer.expandAll", Title: "Explorer: Expand All",
		Keywords: []string{"view", "file", "tree", "folder", "expand"},
		Handler:  app.Explorer.ExpandAll,
	})

	reg.Register(command.Command{
		ID: "explorer.collapseAll", Title: "Explorer: Collapse All",
		Keywords: []string{"view", "file", "tree", "folder", "collapse"},
		Handler:  app.Explorer.CollapseAll,
	})

	reg.Register(command.Command{
		ID: "explorer.open", Title: "Explorer: Toggle Node",
		Keywords: []string{"view", "file"},
		Handler:  func() { app.Explorer.Tree.ActivateSelected() },
	})

	reg.Register(command.Command{
		ID: "explorer.newFile", Title: "Explorer: New File",
		Keywords: []string{"view", "file", "create"},
		Handler:  app.ExplorerNewFile,
	})

	reg.Register(command.Command{
		ID: "explorer.newFolder", Title: "Explorer: New Folder",
		Keywords: []string{"view", "file", "create", "directory"},
		Handler:  app.ExplorerNewFolder,
	})

	reg.Register(command.Command{
		ID: "explorer.rename", Title: "Explorer: Rename",
		Keywords: []string{"view", "file"},
		Handler:  app.ExplorerRename,
	})

	reg.Register(command.Command{
		ID: "explorer.delete", Title: "Explorer: Delete",
		Keywords: []string{"view", "file", "remove"},
		Handler:  app.ExplorerDelete,
	})

	reg.Register(command.Command{
		ID: "file.copyAbsolutePath", Title: "File: Copy Absolute Path",
		Keywords: []string{"file", "path", "copy", "clipboard", "absolute"},
		Handler:  app.CopyAbsolutePath,
	})

	reg.Register(command.Command{
		ID: "file.copyRelativePath", Title: "File: Copy Relative Path",
		Keywords: []string{"file", "path", "copy", "clipboard", "relative"},
		Handler:  app.CopyRelativePath,
	})

	reg.Register(command.Command{
		ID: "explorer.reveal", Title: "Explorer: Reveal in File Manager",
		Keywords: []string{"explorer", "file", "reveal", "manager", "folder", "finder", "open"},
		Handler:  app.ExplorerReveal,
	})

	reg.Register(command.Command{
		ID: "explorer.copyAbsolutePath", Title: "Explorer: Copy Absolute Path",
		Keywords: []string{"explorer", "file", "path", "copy", "clipboard", "absolute"},
		Handler:  app.ExplorerCopyAbsolutePath,
	})

	reg.Register(command.Command{
		ID: "explorer.copyRelativePath", Title: "Explorer: Copy Relative Path",
		Keywords: []string{"explorer", "file", "path", "copy", "clipboard", "relative"},
		Handler:  app.ExplorerCopyRelativePath,
	})

	reg.Register(command.Command{
		ID: "explorer.removeRoot", Title: "Explorer: Remove Folder from Workspace",
		Keywords: []string{"explorer", "workspace", "folder", "remove"},
		Handler:  app.ExplorerRemoveRoot,
	})
}
