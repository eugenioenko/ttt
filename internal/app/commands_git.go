package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func (a *App) DiscardSelected() {
	dir, status, ok := a.Changes.SelectedFile()
	if !ok || status.Staged {
		return
	}
	msg := fmt.Sprintf("Discard changes to %s? This is irreversible.", status.Path)
	if status.Status == "?" {
		msg = fmt.Sprintf("Delete untracked file %s? This is irreversible.", status.Path)
	}
	a.ShowConfirmDialogEx("Discard Changes?", msg,
		[]string{"Cancel", "Discard"},
		[]func(){
			func() {
				a.DismissDialog()
			},
			func() {
				a.DismissDialog()
				if status.Status == "?" {
					a.Changes.applied(git.DiscardUntracked(dir, status.Path))
				} else {
					a.Changes.applied(git.Discard(dir, status.Path))
				}
			},
		},
	)
}

func (a *App) OpenFolder() {
	a.ShowInputDialogEx("Open Folder", "Folder path", "", "Open", func(path string) {
		abs, err := filepath.Abs(workspace.ExpandPath(path))
		if err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			a.StatusError("Not a directory: " + abs)
			return
		}
		a.Workspace.Folders = nil
		a.Workspace.FilePath = ""
		a.Workspace.AddFolder(abs)
		a.refreshWorkspaceWidgets()
	})
}

func (a *App) AddWorkspaceFolder() {
	a.ShowInputDialog("Add Folder", "Folder path", "", func(path string) {
		if path == "" {
			return
		}
		abs, err := filepath.Abs(workspace.ExpandPath(path))
		if err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			a.StatusError("Not a directory: " + abs)
			return
		}
		a.Workspace.AddFolder(abs)
		a.refreshWorkspaceWidgets()
	})
}

func (a *App) RemoveWorkspaceFolder() {
	paths := a.Workspace.Paths()
	if len(paths) <= 1 {
		a.StatusWarn("Cannot remove the last folder")
		return
	}
	items := make([]widgets.SelectItem, len(paths))
	for i, p := range paths {
		items[i] = widgets.SelectItem{ID: p, Label: filepath.Base(p)}
	}
	a.ShowSelectDialog("Remove Folder", items, func(path string) {
		a.Workspace.RemoveFolder(path)
		a.refreshWorkspaceWidgets()
	}, nil)
}

func (a *App) OpenWorkspace() {
	a.ShowInputDialogEx("Open Workspace", "Path to .ttt file", "", "Open", func(path string) {
		abs, err := filepath.Abs(workspace.ExpandPath(path))
		if err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
		ws, err := workspace.LoadFile(abs)
		if err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
		a.Workspace.Folders = ws.Folders
		a.Workspace.FilePath = ws.FilePath
		a.refreshWorkspaceWidgets()
	})
}

func (a *App) SaveWorkspace() {
	initial := "workspace.ttt"
	if a.Workspace.FilePath != "" {
		initial = a.Workspace.FilePath
	}
	a.ShowInputDialog("Save Workspace", "Filename", initial, func(path string) {
		if path == "" {
			return
		}
		abs, err := filepath.Abs(workspace.ExpandPath(path))
		if err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
		if err := a.Workspace.SaveFile(abs); err != nil {
			a.StatusError("Error: " + err.Error())
		} else {
			a.Workspace.FilePath = abs
			a.StatusNotify("Workspace saved: " + abs)
		}
	})
}

func (a *App) OpenPullRequestDialog() {
	if !github.IsGHInstalled() {
		a.StatusError("GitHub CLI (gh) is required. Install from https://cli.github.com/")
		return
	}
	a.ShowInputDialogEx("Open PR Diff", "https://github.com/owner/repo/pull/123", "", "Open", func(url string) {
		a.FetchAndOpenPR(url)
	})
}

// changedFilePaths returns the ordered, de-duplicated list of absolute paths for
// every file that currently appears in the Changes panel (staged and unstaged).
func (a *App) changedFilePaths() []string {
	var paths []string
	seen := make(map[string]bool)
	add := func(dir, rel string) {
		p := filepath.Join(dir, rel)
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, g := range a.Changes.Groups() {
		for _, s := range g.Staged {
			add(g.Dir, s.Path)
		}
		for _, s := range g.Unstaged {
			add(g.Dir, s.Path)
		}
	}
	return paths
}

func (a *App) changesNavFile(dir int) {
	paths := a.changedFilePaths()
	if len(paths) == 0 {
		a.StatusNotify("No changed files")
		return
	}
	current := a.EditorGroup.ActiveFilePath()
	idx := -1
	for i, p := range paths {
		if p == current {
			idx = i
			break
		}
	}
	target := paths[0]
	if idx >= 0 {
		n := (idx + dir) % len(paths)
		if n < 0 {
			n += len(paths)
		}
		target = paths[n]
	}
	a.EditorGroup.OpenFile(target)
	a.FocusEditorIfEnabled()
}

// ChangesNextFile opens the next changed file relative to the active file,
// wrapping around at the end of the list.
func (a *App) ChangesNextFile() { a.changesNavFile(1) }

// ChangesPrevFile opens the previous changed file relative to the active file,
// wrapping around at the start of the list.
func (a *App) ChangesPrevFile() { a.changesNavFile(-1) }

func registerGitCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "changes.nextFile", Title: "Git: Next Changed File",
		Keywords: []string{"git", "changes", "file", "navigate", "next"},
		Handler:  app.ChangesNextFile,
	})

	reg.Register(command.Command{
		ID: "changes.prevFile", Title: "Git: Previous Changed File",
		Keywords: []string{"git", "changes", "file", "navigate", "previous"},
		Handler:  app.ChangesPrevFile,
	})

	reg.Register(command.Command{
		ID: "changes.openDiff", Title: "Git: Open Compact Diff",
		Keywords: []string{"git", "changes", "diff", "compare"},
		Handler: func() {
			app.openSelectedDiff(false)
		},
	})

	reg.Register(command.Command{
		ID: "changes.openExtendedDiff", Title: "Git: Open Extended Diff",
		Keywords: []string{"git", "changes", "diff", "compare"},
		Handler: func() {
			app.openSelectedDiff(true)
		},
	})

	reg.Register(command.Command{
		ID: "changes.openFile", Title: "Git: Open File",
		Keywords: []string{"git", "changes"},
		Handler: func() {
			app.Changes.ActivateSelected()
		},
	})

	reg.Register(command.Command{
		ID: "changes.refresh", Title: "Git: Refresh Changes",
		Keywords: []string{"git", "changes", "reload"},
		Handler: func() {
			app.Changes.Refresh()
		},
	})

	reg.Register(command.Command{
		ID: "changes.expandAll", Title: "Git: Expand All File Trees",
		Keywords: []string{"git", "changes", "history", "detail", "tree", "folder", "expand"},
		Handler:  app.ExpandAllGitFiles,
	})

	reg.Register(command.Command{
		ID: "changes.collapseAll", Title: "Git: Collapse All File Trees",
		Keywords: []string{"git", "changes", "history", "detail", "tree", "folder", "collapse"},
		Handler:  app.CollapseAllGitFiles,
	})

	reg.Register(command.Command{
		ID: "changes.expandAllWorkingTree", Title: "Git: Expand All Changes Files",
		Keywords: []string{"git", "changes", "working", "tree", "folder", "expand"},
		Handler:  app.ExpandAllChangesFiles,
	})

	reg.Register(command.Command{
		ID: "changes.collapseAllWorkingTree", Title: "Git: Collapse All Changes Files",
		Keywords: []string{"git", "changes", "working", "tree", "folder", "collapse"},
		Handler:  app.CollapseAllChangesFiles,
	})

	reg.Register(command.Command{
		ID: "changes.expandAllCommitDetail", Title: "Git: Expand All Commit Detail Files",
		Keywords: []string{"git", "commit", "detail", "file", "expand"},
		Handler:  app.ExpandAllCommitDetailFiles,
	})

	reg.Register(command.Command{
		ID: "changes.collapseAllCommitDetail", Title: "Git: Collapse All Commit Detail Files",
		Keywords: []string{"git", "commit", "detail", "file", "collapse"},
		Handler:  app.CollapseAllCommitDetailFiles,
	})

	reg.Register(command.Command{
		ID: "changes.stage", Title: "Git: Stage File",
		Keywords: []string{"git", "changes", "add"},
		Handler: func() {
			dir, status, ok := app.Changes.SelectedFile()
			if ok && !status.Staged {
				app.Changes.applied(git.Stage(dir, status.Path))
			}
		},
	})

	reg.Register(command.Command{
		ID: "changes.unstage", Title: "Git: Unstage File",
		Keywords: []string{"git", "changes", "remove"},
		Handler: func() {
			dir, status, ok := app.Changes.SelectedFile()
			if ok && status.Staged {
				app.Changes.applied(git.Unstage(dir, status.Path))
			}
		},
	})

	reg.Register(command.Command{
		ID: "changes.discard", Title: "Git: Discard Changes",
		Keywords: []string{"git", "changes", "revert", "undo"},
		Handler:  app.DiscardSelected,
	})

	registerGitCmd := func(id, title string, keywords []string, ops []RepoOp, progress, done string) {
		reg.Register(command.Command{
			ID: id, Title: title,
			Keywords: keywords,
			Handler: func() {
				app.RunRepoTask(RepoTask{
					Progress: progress, Done: done,
					Dirs: app.Changes.Dirs, Ops: ops,
				})
			},
		})
	}
	registerGitCmd("git.pull", "Git: Pull", []string{"git", "fetch", "download"}, []RepoOp{OpPull}, "Pulling", "Pulled successfully")
	registerGitCmd("git.push", "Git: Push", []string{"git", "upload", "publish"}, []RepoOp{OpPush}, "Pushing", "Pushed successfully")
	registerGitCmd("git.sync", "Git: Sync", []string{"git", "fetch", "upload"}, []RepoOp{OpPull, OpPush}, "Syncing", "Synced successfully")

	reg.Register(command.Command{
		ID: "git.copyPermalink", Title: "Git: Copy GitHub Permalink",
		Keywords: []string{"git", "github", "link", "url", "permalink", "copy"},
		Handler: func() {
			filePath := app.EditorGroup.ActiveFilePath()
			if filePath == "" || app.EditorGroup.IsActiveVirtual() {
				app.StatusWarn("No file open")
				return
			}
			repoDir := git.RepoRoot(filepath.Dir(filePath))
			if repoDir == "" {
				app.StatusWarn("Not a git repository")
				return
			}
			line, _ := app.EditorGroup.ActiveCursor()
			link := git.Permalink(repoDir, filePath, line)
			if link == "" {
				app.StatusWarn("Could not generate permalink — no remote found")
				return
			}
			clipboard.Set(link)
			app.StatusNotify("Permalink copied to clipboard")
		},
	})
}

func registerWorkspaceCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "workspace.openFolder", Title: "Open Folder",
		Keywords: []string{"file", "directory", "project"},
		Handler:  app.OpenFolder,
	})

	reg.Register(command.Command{
		ID: "workspace.addFolder", Title: "Add Folder",
		Keywords: []string{"file", "directory", "project"},
		Handler:  app.AddWorkspaceFolder,
	})

	reg.Register(command.Command{
		ID: "workspace.removeFolder", Title: "Remove Folder",
		Keywords: []string{"file", "directory", "project"},
		Handler:  app.RemoveWorkspaceFolder,
	})

	reg.Register(command.Command{
		ID: "workspace.open", Title: "Open Workspace",
		Keywords: []string{"file", "project"},
		Handler:  app.OpenWorkspace,
	})

	reg.Register(command.Command{
		ID: "workspace.save", Title: "Save Workspace",
		Keywords: []string{"file", "project"},
		Handler:  app.SaveWorkspace,
	})
}

func registerPRCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "pr.openDiff", Title: "Git: Open PR Diff",
		Keywords: []string{"git", "pull request", "github"},
		Handler:  app.OpenPullRequestDialog,
	})
	reg.Register(command.Command{
		ID: "pr.close", Title: "Git: Close PR",
		Keywords: []string{"git", "pull request", "github"},
		Handler: func() {
			app.Changes.RemovePRGroups()
		},
	})
}
