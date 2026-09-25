package app

import (
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/i18n"
)

func (a *App) explorerReload() {
	a.Explorer.Reload()
}

func (a *App) ExplorerOpen() {
	context := a.ExplorerContextNode
	a.ExplorerContextNode = nil
	if context != nil {
		a.Explorer.ActivatePath(context.ID)
		return
	}
	a.Explorer.Tree.ActivateSelected()
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
		a.StatusWarn(i18n.Text("status.noFileOpen"))
		return
	}
	clipboard.Set(path)
	a.StatusNotify(i18n.Text("status.absolutePathCopiedToClipboard"))
}

func (a *App) CopyRelativePath() {
	path := a.activeFilePath()
	if path == "" {
		a.StatusWarn(i18n.Text("status.noFileOpen"))
		return
	}
	rel := a.relativePath(path)
	clipboard.Set(rel)
	a.StatusNotify(i18n.Text("status.relativePathCopiedToClipboard"))
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

func (a *App) ExplorerRevealActiveFile() {
	path := a.revealableActiveFile()
	if path == "" {
		a.StatusWarn(i18n.Text("status.noFileOpen"))
		return
	}
	a.Sidebar.SetActivePanel("explorer")
	if !a.Sidebar.Visible {
		a.ShowSidebar()
	}
	if !a.Explorer.RevealPath(path) {
		a.StatusWarn(i18n.Text("status.fileIsNotInTheExplorer"))
	}
}

func (a *App) autoRevealActiveFile() {
	if !a.Settings.Explorer.AutoReveal {
		return
	}
	if path := a.revealableActiveFile(); path != "" {
		a.Explorer.RevealPath(path)
	}
}

func (a *App) revealableActiveFile() string {
	if a.EditorGroup.ActiveDiffWidget() != nil {
		return ""
	}
	return a.activeFilePath()
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
		ID: "explorer.refresh", Title: i18n.Text("command.explorerRefresh"),
		Keywords: []string{"view", "file", "reload"},
		Handler:  func() { app.Explorer.Reload() },
	})

	reg.Register(command.Command{
		ID: "explorer.expandAll", Title: i18n.Text("command.explorerExpandAll"),
		Keywords: []string{"view", "file", "tree", "folder", "expand"},
		Handler:  app.Explorer.ExpandAll,
	})

	reg.Register(command.Command{
		ID: "explorer.collapseAll", Title: i18n.Text("command.explorerCollapseAll"),
		Keywords: []string{"view", "file", "tree", "folder", "collapse"},
		Handler:  app.Explorer.CollapseAll,
	})

	reg.Register(command.Command{
		ID: "explorer.open", Title: i18n.Text("command.explorerToggleNode"),
		Keywords: []string{"view", "file"},
		Handler:  app.ExplorerOpen,
	})

	reg.Register(command.Command{
		ID: "explorer.newFile", Title: i18n.Text("command.explorerNewFile"),
		Keywords: []string{"view", "file", "create"},
		Handler:  app.ExplorerNewFile,
	})

	reg.Register(command.Command{
		ID: "explorer.newFolder", Title: i18n.Text("command.explorerNewFolder"),
		Keywords: []string{"view", "file", "create", "directory"},
		Handler:  app.ExplorerNewFolder,
	})

	reg.Register(command.Command{
		ID: "explorer.rename", Title: i18n.Text("command.explorerRename"),
		Keywords: []string{"view", "file"},
		Handler:  app.ExplorerRename,
	})

	reg.Register(command.Command{
		ID: "explorer.delete", Title: i18n.Text("command.explorerDelete"),
		Keywords: []string{"view", "file", "remove"},
		Handler:  app.ExplorerDelete,
	})

	reg.Register(command.Command{
		ID: "file.copyAbsolutePath", Title: i18n.Text("command.fileCopyAbsolutePath"),
		Keywords: []string{"file", "path", "copy", "clipboard", "absolute"},
		Handler:  app.CopyAbsolutePath,
	})

	reg.Register(command.Command{
		ID: "file.copyRelativePath", Title: i18n.Text("command.fileCopyRelativePath"),
		Keywords: []string{"file", "path", "copy", "clipboard", "relative"},
		Handler:  app.CopyRelativePath,
	})

	reg.Register(command.Command{
		ID: "explorer.reveal", Title: i18n.Text("command.explorerRevealInFileManager"),
		Keywords: []string{"explorer", "file", "reveal", "manager", "folder", "finder", "open"},
		Handler:  app.ExplorerReveal,
	})

	reg.Register(command.Command{
		ID: "explorer.revealActiveFile", Title: i18n.Text("command.explorerRevealActiveFile"),
		Keywords: []string{"explorer", "file", "reveal", "locate", "sync", "tree", "select"},
		Handler:  app.ExplorerRevealActiveFile,
	})

	reg.Register(command.Command{
		ID: "explorer.copyAbsolutePath", Title: i18n.Text("command.explorerCopyAbsolutePath"),
		Keywords: []string{"explorer", "file", "path", "copy", "clipboard", "absolute"},
		Handler:  app.ExplorerCopyAbsolutePath,
	})

	reg.Register(command.Command{
		ID: "explorer.copyRelativePath", Title: i18n.Text("command.explorerCopyRelativePath"),
		Keywords: []string{"explorer", "file", "path", "copy", "clipboard", "relative"},
		Handler:  app.ExplorerCopyRelativePath,
	})

	reg.Register(command.Command{
		ID: "explorer.insertPathInTerminal", Title: i18n.Text("command.explorerInsertPathInTerminal"),
		Keywords: []string{"explorer", "file", "path", "terminal", "insert", "paste", "drag", "drop"},
		Handler:  app.InsertExplorerPathInTerminal,
	})

	reg.Register(command.Command{
		ID: "explorer.removeRoot", Title: i18n.Text("command.explorerRemoveFolderFromWorkspace"),
		Keywords: []string{"explorer", "workspace", "folder", "remove"},
		Handler:  app.ExplorerRemoveRoot,
	})
}
