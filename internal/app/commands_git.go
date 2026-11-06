package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func (a *App) DiscardSelected() {
	if _, _, _, isFolder := a.Changes.SelectedFolder(); isFolder {
		a.Changes.confirmDiscardFolder(a.Changes.Tree.Selected())
		return
	}
	dir, status, ok := a.Changes.SelectedFile()
	if !ok || status.Staged {
		return
	}
	msg := i18n.Format("git.discardChangesToThisIsIrreversible", i18n.Args{"path": status.Path})
	if status.Status == "?" {
		msg = i18n.Format("git.deleteUntrackedFileThisIsIrreversible", i18n.Args{"path": status.Path})
	}
	a.ShowConfirmDialogEx(i18n.Text("dialog.discardChanges"), msg,
		[]string{i18n.Text("general.cancel"), i18n.Text("general.discard")},
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
	a.ShowFolderPicker(i18n.Text("general.openFolder"), i18n.Text("general.open"), "", func(path string) {
		abs, err := filepath.Abs(workspace.ExpandPath(path))
		if err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
			return
		}
		a.openFolderPath(abs)
	})
}

func (a *App) openFolderPath(abs string) {
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		a.StatusError(i18n.Format("status.notADirectory", i18n.Args{"abs": abs}))
		return
	}
	a.Workspace.Folders = nil
	a.Workspace.FilePath = ""
	a.Workspace.AddFolder(abs)
	a.refreshWorkspaceWidgets()
}

func (a *App) AddWorkspaceFolder() {
	a.ShowFolderPicker(i18n.Text("general.addFolder"), i18n.Text("general.add"), "", func(path string) {
		if path == "" {
			return
		}
		if err := a.addWorkspaceFolders([]string{path}); err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
		}
	})
}

func (a *App) addWorkspaceFolders(paths []string) error {
	var errs []error
	count := len(a.Workspace.Folders)
	for _, path := range paths {
		abs, err := filepath.Abs(workspace.ExpandPath(path))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			errs = append(errs, fmt.Errorf("not a directory: %s", abs))
			continue
		}
		a.Workspace.AddFolder(abs)
	}
	if len(a.Workspace.Folders) != count {
		a.refreshWorkspaceWidgets()
	}
	return errors.Join(errs...)
}

func (a *App) RemoveWorkspaceFolder() {
	paths := a.Workspace.Paths()
	if len(paths) == 0 {
		a.StatusWarn(i18n.Text("status.noFolderOpen"))
		return
	}
	items := make([]widgets.SelectItem, len(paths))
	for i, p := range paths {
		items[i] = widgets.SelectItem{ID: p, Label: filepath.Base(p)}
	}
	a.ShowSelectDialog(i18n.Text("git.removeFolder"), items, func(path string) {
		a.Workspace.RemoveFolder(path)
		a.refreshWorkspaceWidgets()
	}, nil)
}

func (a *App) CloseWorkspace() {
	if len(a.Workspace.Paths()) == 0 {
		return
	}
	a.Workspace.Folders = nil
	a.Workspace.FilePath = ""
	a.refreshWorkspaceWidgets()
}

func (a *App) OpenWorkspace() {
	a.ShowInputDialogEx(i18n.Text("general.openWorkspace"), i18n.Text("git.pathToTttFile"), "", i18n.Text("general.open"), func(path string) {
		abs, err := filepath.Abs(workspace.ExpandPath(path))
		if err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
			return
		}
		ws, err := workspace.LoadFile(abs)
		if err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
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
	a.ShowInputDialog(i18n.Text("general.saveWorkspace"), i18n.Text("general.filename"), initial, func(path string) {
		if path == "" {
			return
		}
		abs, err := filepath.Abs(workspace.ExpandPath(path))
		if err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
			return
		}
		if err := a.Workspace.SaveFile(abs); err != nil {
			a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
		} else {
			a.Workspace.FilePath = abs
			a.invalidateRepositoryPath(abs, RepositoryWorktree)
			a.StatusNotify(i18n.Format("status.workspaceSaved", i18n.Args{"abs": abs}))
		}
	})
}

func (a *App) OpenPullRequestDialog() {
	if !github.IsGHInstalled() {
		a.StatusError(i18n.Text("status.gitHubCLIGhIsRequiredInstallFromHttpsCliGithubCom"))
		return
	}
	a.ShowInputDialogEx(i18n.Text("git.openPRDiff"), "https://github.com/owner/repo/pull/123", "", i18n.Text("general.open"), func(url string) {
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
		a.StatusNotify(i18n.Text("status.noChangedFiles"))
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

func (a *App) RefreshChanges() {
	if a.Repository != nil {
		a.Repository.RefreshNow(RepositoryWorktree | RepositoryHistory)
		return
	}
	a.Changes.Refresh()
}

func (a *App) NewBranchDialog() {
	groups := a.Changes.Groups()
	dirs := make([]string, 0, len(groups))
	seen := make(map[string]bool)
	for _, group := range groups {
		if group.IsPR {
			continue
		}
		dir := git.RepoRoot(group.Dir)
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	if len(dirs) == 0 {
		a.StatusWarn(i18n.Text("status.noGitRepositoriesInChangesPanel"))
		return
	}

	a.ShowInputDialogEx(i18n.Text("git.newBranch"), i18n.Text("git.branchName"), "", i18n.Text("general.create"), func(name string) {
		a.RunRepoTask(RepoTask{
			Progress: i18n.Text("git.creatingBranch"),
			Done:     i18n.Format("git.createdBranch", i18n.Args{"name": name}),
			Dirs:     dirs,
			Ops:      []RepoOp{OpCreateBranch(name)},
		})
	})
}

func registerGitCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "changes.nextFile", Title: i18n.Text("command.gitNextChangedFile"),
		Keywords: []string{"git", "changes", "file", "navigate", "next"},
		Handler:  app.ChangesNextFile,
	})

	reg.Register(command.Command{
		ID: "changes.prevFile", Title: i18n.Text("command.gitPreviousChangedFile"),
		Keywords: []string{"git", "changes", "file", "navigate", "previous"},
		Handler:  app.ChangesPrevFile,
	})

	reg.Register(command.Command{
		ID: "changes.openDiff", Title: i18n.Text("command.gitOpenChanges"),
		Keywords: []string{"git", "changes", "diff", "compare"},
		Handler: func() {
			app.openSelectedDiff(false)
		},
	})

	reg.Register(command.Command{
		ID: "changes.openExtendedDiff", Title: i18n.Text("command.gitOpenFullDiff"),
		Keywords: []string{"git", "changes", "diff", "compare"},
		Handler: func() {
			app.openSelectedDiff(true)
		},
	})

	reg.Register(command.Command{
		ID: "changes.openFile", Title: i18n.Text("command.gitOpenFile"),
		Keywords: []string{"git", "changes"},
		Handler: func() {
			app.Changes.ActivateSelected()
		},
	})

	reg.Register(command.Command{
		ID:       "changes.viewAll",
		Title:    i18n.Text("command.gitOpenCurrentChanges"),
		Keywords: []string{"git", "changes", "diff", "all", "working tree"},
		Handler:  app.OpenCurrentChanges,
	})

	reg.Register(command.Command{
		ID: "changes.refresh", Title: i18n.Text("command.gitRefreshChanges"),
		Keywords: []string{"git", "changes", "reload"},
		Handler:  app.RefreshChanges,
	})

	reg.Register(command.Command{
		ID: "changes.expandAll", Title: i18n.Text("command.gitExpandAllFileTrees"),
		Keywords: []string{"git", "changes", "history", "detail", "tree", "folder", "expand"},
		Handler:  app.ExpandAllGitFiles,
	})

	reg.Register(command.Command{
		ID: "changes.collapseAll", Title: i18n.Text("command.gitCollapseAllFileTrees"),
		Keywords: []string{"git", "changes", "history", "detail", "tree", "folder", "collapse"},
		Handler:  app.CollapseAllGitFiles,
	})

	reg.Register(command.Command{
		ID: "changes.expandAllWorkingTree", Title: i18n.Text("command.gitExpandAllChangesFiles"),
		Keywords: []string{"git", "changes", "working", "tree", "folder", "expand"},
		Handler:  app.ExpandAllChangesFiles,
	})

	reg.Register(command.Command{
		ID: "changes.collapseAllWorkingTree", Title: i18n.Text("command.gitCollapseAllChangesFiles"),
		Keywords: []string{"git", "changes", "working", "tree", "folder", "collapse"},
		Handler:  app.CollapseAllChangesFiles,
	})

	reg.Register(command.Command{
		ID: "changes.expandAllCommitDetail", Title: i18n.Text("command.gitExpandAllCommitDetailFiles"),
		Keywords: []string{"git", "commit", "detail", "file", "expand"},
		Handler:  app.ExpandAllCommitDetailFiles,
	})

	reg.Register(command.Command{
		ID: "changes.collapseAllCommitDetail", Title: i18n.Text("command.gitCollapseAllCommitDetailFiles"),
		Keywords: []string{"git", "commit", "detail", "file", "collapse"},
		Handler:  app.CollapseAllCommitDetailFiles,
	})

	reg.Register(command.Command{
		ID: "changes.stage", Title: i18n.Text("command.gitStageFile"),
		Keywords: []string{"git", "changes", "add"},
		Handler: func() {
			dir, status, ok := app.Changes.SelectedFile()
			if ok && !status.Staged {
				app.Changes.applied(git.Stage(dir, status.Path))
			}
		},
	})

	reg.Register(command.Command{
		ID: "changes.unstage", Title: i18n.Text("command.gitUnstageFile"),
		Keywords: []string{"git", "changes", "remove"},
		Handler: func() {
			dir, status, ok := app.Changes.SelectedFile()
			if ok && status.Staged {
				app.Changes.applied(git.Unstage(dir, status.Path))
			}
		},
	})

	reg.Register(command.Command{
		ID: "changes.discard", Title: i18n.Text("command.gitDiscardChanges"),
		Keywords: []string{"git", "changes", "revert", "undo"},
		Handler:  app.DiscardSelected,
	})

	reg.Register(command.Command{
		ID: "changes.stageFolder", Title: i18n.Text("command.gitStageFolder"),
		Keywords: []string{"git", "changes", "add", "folder", "directory"},
		Handler: func() {
			app.Changes.stageFolder(app.Changes.Tree.Selected())
		},
	})

	reg.Register(command.Command{
		ID: "changes.unstageFolder", Title: i18n.Text("command.gitUnstageFolder"),
		Keywords: []string{"git", "changes", "remove", "folder", "directory"},
		Handler: func() {
			app.Changes.unstageFolder(app.Changes.Tree.Selected())
		},
	})

	reg.Register(command.Command{
		ID: "changes.discardFolder", Title: i18n.Text("command.gitDiscardFolderChanges"),
		Keywords: []string{"git", "changes", "revert", "undo", "folder", "directory"},
		Handler: func() {
			app.Changes.confirmDiscardFolder(app.Changes.Tree.Selected())
		},
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
	registerGitCmd("git.pull", i18n.Text("command.gitPull"), []string{"git", "fetch", "download"}, []RepoOp{OpPull}, i18n.Text("git.pulling"), i18n.Text("git.pulledSuccessfully"))
	registerGitCmd("git.push", i18n.Text("command.gitPush"), []string{"git", "upload", "publish"}, []RepoOp{OpPush}, i18n.Text("git.pushing"), i18n.Text("git.pushedSuccessfully"))
	registerGitCmd("git.sync", i18n.Text("command.gitSync"), []string{"git", "fetch", "upload"}, []RepoOp{OpPull, OpPush}, i18n.Text("git.syncing"), i18n.Text("git.syncedSuccessfully"))
	reg.Register(command.Command{
		ID:       "git.newBranch",
		Title:    i18n.Text("command.gitNewBranch"),
		Keywords: []string{"git", "branch", "create"},
		Handler:  app.NewBranchDialog,
	})

	reg.Register(command.Command{
		ID: "git.copyPermalink", Title: i18n.Text("command.gitCopyGitHubPermalink"),
		Keywords: []string{"git", "github", "link", "url", "permalink", "copy"},
		Handler: func() {
			filePath := app.EditorGroup.ActiveFilePath()
			if filePath == "" || app.EditorGroup.IsActiveVirtual() {
				app.StatusWarn(i18n.Text("status.noFileOpen"))
				return
			}
			repoDir := git.RepoRoot(filepath.Dir(filePath))
			if repoDir == "" {
				app.StatusWarn(i18n.Text("status.notAGitRepository"))
				return
			}
			line, _ := app.EditorGroup.ActiveCursor()
			link := git.Permalink(repoDir, filePath, line)
			if link == "" {
				app.StatusWarn(i18n.Text("status.couldNotGeneratePermalinkNoRemoteFound"))
				return
			}
			clipboard.Set(link)
			app.StatusNotify(i18n.Text("status.permalinkCopiedToClipboard"))
		},
	})
}

func registerWorkspaceCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "workspace.openFolder", Title: i18n.Text("general.openFolder"),
		Keywords: []string{"file", "directory", "project"},
		Handler:  app.OpenFolder,
	})

	reg.Register(command.Command{
		ID: "workspace.addFolder", Title: i18n.Text("general.addFolder"),
		Keywords: []string{"file", "directory", "project"},
		Handler:  app.AddWorkspaceFolder,
	})

	reg.Register(command.Command{
		ID: "workspace.removeFolder", Title: i18n.Text("command.removeFolder"),
		Keywords: []string{"file", "directory", "project"},
		Handler:  app.RemoveWorkspaceFolder,
	})

	reg.Register(command.Command{
		ID: "workspace.close", Title: i18n.Text("general.closeWorkspace"),
		Keywords: []string{"workspace", "folder", "close", "project"},
		Handler:  app.CloseWorkspace,
	})

	reg.Register(command.Command{
		ID: "welcome.addFavorite", Title: i18n.Text("command.welcomeAddFolderToFavorites"),
		Keywords: []string{"welcome", "favorite", "bookmark", "folder", "project"},
		Handler:  app.AddFavorite,
	})

	reg.Register(command.Command{
		ID: "welcome.removeFavorite", Title: i18n.Text("command.welcomeRemoveFolderFromFavorites"),
		Keywords: []string{"welcome", "favorite", "bookmark", "folder", "project"},
		Handler:  app.RemoveFavorite,
	})

	reg.Register(command.Command{
		ID: "workspace.clone", Title: i18n.Text("command.gitCloneRepository"),
		Keywords: []string{"git", "github", "clone", "repository", "repo", "download"},
		Handler:  app.CloneRepository,
	})

	reg.Register(command.Command{
		ID: "help.welcome", Title: i18n.Text("command.helpWelcome"),
		Keywords: []string{"welcome", "start", "getting started", "home"},
		Handler:  app.ShowWelcome,
	})

	reg.Register(command.Command{
		ID: "workspace.open", Title: i18n.Text("general.openWorkspace"),
		Keywords: []string{"file", "project"},
		Handler:  app.OpenWorkspace,
	})

	reg.Register(command.Command{
		ID: "workspace.save", Title: i18n.Text("general.saveWorkspace"),
		Keywords: []string{"file", "project"},
		Handler:  app.SaveWorkspace,
	})
}

func registerPRCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "pr.openDiff", Title: i18n.Text("command.gitOpenPRDiff"),
		Keywords: []string{"git", "pull request", "github"},
		Handler:  app.OpenPullRequestDialog,
	})
	reg.Register(command.Command{
		ID: "pr.close", Title: i18n.Text("command.gitClosePR"),
		Keywords: []string{"git", "pull request", "github"},
		Handler: func() {
			app.Changes.RemovePRGroups()
		},
	})
}
