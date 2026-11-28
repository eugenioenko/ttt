package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) BuildChangesPanelMenu() []ui.ContextMenuItem {
	return []ui.ContextMenuItem{
		{Label: i18n.Text("menu.openCurrentChanges"), Command: "changes.viewAll"},
		ui.MenuSep(),
		{Label: i18n.Text("general.refresh"), Command: "changes.refresh"},
		{Label: i18n.Text("menu.gitFiles"), Submenu: a.BuildChangesGitFileOptions()},
		{Label: i18n.Text("menu.diffViews"), Submenu: a.BuildDiffViewOptions()},
		ui.MenuSep(),
		{Label: i18n.Text("menu.pull"), Command: "git.pull"},
		{Label: i18n.Text("menu.push"), Command: "git.push"},
		{Label: i18n.Text("menu.sync"), Command: "git.sync"},
		{Label: i18n.Text("menu.newBranch"), Command: "git.newBranch"},
		ui.MenuSep(),
		{Label: i18n.Text("menu.openPRDiff"), Command: "pr.openDiff"},
		ui.MenuSep(),
		{Label: i18n.Text("general.help"), Command: "changes.help"},
	}
}

func (a *App) BuildChangesContextMenu() []ui.ContextMenuItem {
	return []ui.ContextMenuItem{
		{Label: i18n.Text("menu.openCurrentChanges"), Command: "changes.viewAll"},
		ui.MenuSep(),
		{Label: i18n.Text("general.refresh"), Command: "changes.refresh"},
		{Label: i18n.Text("menu.gitFiles"), Submenu: a.BuildChangesGitFileOptions()},
		{Label: i18n.Text("menu.diffViews"), Submenu: a.BuildDiffViewOptions()},
	}
}

func (a *App) ShowChangesContextMenu(sx, sy int) {
	openContextMenu(a, a.BuildChangesContextMenu(), sx, sy)
}

func (a *App) ShowChangesFileContextMenu(dir string, status scm.Change, sx, sy int) {
	var items []ui.ContextMenuItem
	if status.Group == scm.Staged {
		items = append(items, changesContextMenuStaged...)
	} else {
		items = append(items, changesContextMenuUnstaged...)
	}
	if !a.Changes.supportsStaging(dir) {
		filtered := items[:0]
		for _, item := range items {
			if item.Command != "changes.stage" && item.Command != "changes.unstage" {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	items = append(items, ui.MenuSep())
	items = append(items, a.BuildChangesContextMenu()...)
	openContextMenu(a, items, sx, sy)
}

func (a *App) ShowChangesFolderContextMenu(dir, _ string, staged bool, sx, sy int) {
	var items []ui.ContextMenuItem
	if staged {
		items = append(items, changesFolderContextMenuStaged...)
	} else {
		items = append(items, changesFolderContextMenuUnstaged...)
	}
	if !a.Changes.supportsStaging(dir) {
		filtered := items[:0]
		for _, item := range items {
			if item.Command != "changes.stageFolder" && item.Command != "changes.unstageFolder" {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	if len(items) > 0 {
		items = append(items, ui.MenuSep())
	}
	items = append(items, a.BuildChangesContextMenu()...)
	openContextMenu(a, items, sx, sy)
}

func (a *App) ShowGroupMenu(dir string, sx, sy int) {
	reg := a.Reg
	items := []ui.ContextMenuItem{
		{Label: i18n.Text("menu.pull"), Command: "git.pull." + dir},
		{Label: i18n.Text("menu.push"), Command: "git.push." + dir},
		{Label: i18n.Text("menu.sync"), Command: "git.sync." + dir},
	}
	registerDirGitCmd := func(id, title string, ops []RepoOp, progress, done string) {
		reg.Register(command.Command{
			ID: id, Title: title,
			Handler: func() {
				a.RunRepoTask(RepoTask{
					Progress: progress, Done: done,
					Dirs: []string{dir}, Ops: ops,
				})
			},
		})
	}
	registerDirGitCmd("git.pull."+dir, i18n.Text("git.pull"), []RepoOp{OpPull}, i18n.Text("git.pulling"), i18n.Text("git.pulledSuccessfully"))
	registerDirGitCmd("git.push."+dir, i18n.Text("git.push"), []RepoOp{OpPush}, i18n.Text("git.pushing"), i18n.Text("git.pushedSuccessfully"))
	registerDirGitCmd("git.sync."+dir, i18n.Text("git.sync"), []RepoOp{OpPull, OpPush}, i18n.Text("git.syncing"), i18n.Text("git.syncedSuccessfully"))
	openContextMenu(a, items, sx, sy)
}
