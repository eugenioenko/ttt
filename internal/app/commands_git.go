package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/ui"
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
	a.ShowConfirmDialog(msg,
		[]string{"Cancel", "Discard"},
		[]func(){
			func() {
				a.DismissDialog()
			},
			func() {
				a.DismissDialog()
				if status.Status == "?" {
					git.DiscardUntracked(dir, status.Path)
				} else {
					git.Discard(dir, status.Path)
				}
				a.Changes.Refresh()
			},
		},
	)
}

func (a *App) OpenFolder() {
	dialog := ui.NewInputDialogWidget("Open Folder", "Folder path", "")
	dialog.ConfirmLabel = "Open"
	dialog.Borders = a.Borders
	dialog.OnSubmit = func(path string) {
		a.DismissDialog()
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
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
	}
	dialog.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(dialog)
}

func (a *App) AddWorkspaceFolder() {
	a.ShowInputDialog("Add Folder", "Folder path", "", func(path string) {
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
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
	var cmds []command.Command
	for _, p := range paths {
		cmds = append(cmds, command.Command{ID: p, Title: filepath.Base(p)})
	}
	a.ShowPicker(cmds, func(path string) {
		a.Workspace.RemoveFolder(path)
		a.refreshWorkspaceWidgets()
	})
}

func (a *App) OpenWorkspace() {
	dialog := ui.NewInputDialogWidget("Open Workspace", "Path to .ttt file", "")
	dialog.ConfirmLabel = "Open"
	dialog.Borders = a.Borders
	dialog.OnSubmit = func(path string) {
		a.DismissDialog()
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
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
	}
	dialog.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(dialog)
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
		abs, err := filepath.Abs(path)
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
	dialog := ui.NewInputDialogWidget("Review PR", "https://github.com/owner/repo/pull/123", "")
	dialog.ConfirmLabel = "Review"
	dialog.Borders = a.Borders
	dialog.OnSubmit = func(url string) {
		a.DismissDialog()
		if url != "" {
			a.FetchAndOpenPR(url)
		}
	}
	dialog.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(dialog)
}

func registerGitCommands(app *App) {
	reg := app.Reg

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
			fullPath := app.Changes.SelectedFullPath()
			if fullPath != "" {
				app.EditorGroup.OpenFile(fullPath)
				app.FocusEditorIfEnabled()
			}
		},
	})

	reg.Register(command.Command{
		ID: "changes.refresh", Title: "Git: Refresh Changes",
		Keywords: []string{"git", "changes", "reload"},
		Handler:  func() { app.Changes.Refresh() },
	})

	reg.Register(command.Command{
		ID: "changes.stage", Title: "Git: Stage File",
		Keywords: []string{"git", "changes", "add"},
		Handler: func() {
			dir, status, ok := app.Changes.SelectedFile()
			if ok && !status.Staged {
				git.Stage(dir, status.Path)
				app.Changes.Refresh()
			}
		},
	})

	reg.Register(command.Command{
		ID: "changes.unstage", Title: "Git: Unstage File",
		Keywords: []string{"git", "changes", "remove"},
		Handler: func() {
			dir, status, ok := app.Changes.SelectedFile()
			if ok && status.Staged {
				git.Unstage(dir, status.Path)
				app.Changes.Refresh()
			}
		},
	})

	reg.Register(command.Command{
		ID: "changes.discard", Title: "Git: Discard Changes",
		Keywords: []string{"git", "changes", "revert", "undo"},
		Handler:  app.DiscardSelected,
	})

	registerGitCmd := func(id, title string, keywords []string, ops []func(string) error, verb string) {
		reg.Register(command.Command{
			ID: id, Title: title,
			Keywords: keywords,
			Handler: func() {
				for _, dir := range app.Changes.Dirs {
					for _, op := range ops {
						if err := op(dir); err != nil {
							app.StatusError(fmt.Sprintf("%s failed: %v", verb, err))
							return
						}
					}
				}
				app.StatusNotify(verb + " successfully")
				app.Changes.Refresh()
			},
		})
	}
	registerGitCmd("git.pull", "Git: Pull", []string{"git", "fetch", "download"}, []func(string) error{git.Pull}, "Pulled")
	registerGitCmd("git.push", "Git: Push", []string{"git", "upload", "publish"}, []func(string) error{git.Push}, "Pushed")
	registerGitCmd("git.sync", "Git: Sync", []string{"git", "fetch", "upload"}, []func(string) error{git.Pull, git.Push}, "Synced")

	reg.Register(command.Command{
		ID: "git.copyPermalink", Title: "Git: Copy GitHub Permalink",
		Keywords: []string{"git", "github", "link", "url", "permalink", "copy"},
		Handler: func() {
			filePath := app.EditorGroup.ActiveFilePath()
			if filePath == "" || filePath == "untitled" {
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
		ID: "pr.review", Title: "Git: Review PR",
		Keywords: []string{"git", "pull request", "github"},
		Handler:  app.OpenPullRequestDialog,
	})
	reg.Register(command.Command{
		ID: "pr.close", Title: "Git: Close PR",
		Keywords: []string{"git", "pull request", "github"},
		Handler:  func() { app.Changes.RemovePRGroups() },
	})
	reg.Register(command.Command{
		ID: "pr.showComments", Title: "Git: Show PR Comments",
		Keywords: []string{"git", "pull request", "github", "comments", "review"},
		Handler:  app.ShowPRCommentsDialog,
	})
}
