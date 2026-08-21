package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func (a *App) ShowSidebarMoreMenu(sx, sy int) {
	var items []ui.ContextMenuItem
	switch a.Sidebar.ActivePanel {
	case "explorer":
		items = []ui.ContextMenuItem{
			{Label: "New File", Command: "file.new"},
			{Label: "Add Folder", Command: "workspace.addFolder"},
			{Label: "Refresh", Command: "explorer.refresh"},
			ui.MenuSep(),
			{Label: "Expand All", Command: "explorer.expandAll"},
			{Label: "Collapse All", Command: "explorer.collapseAll"},
			ui.MenuSep(),
			{Label: "Help", Command: "explorer.help"},
		}
	case "search":
		replaceLabel := "Replace"
		if a.Search.IsReplaceMode() {
			replaceLabel = "Search"
		}
		items = []ui.ContextMenuItem{
			{Label: replaceLabel, Command: "sidebar.searchReplace"},
			ui.MenuSep(),
			{Label: "Expand All", Command: "search.expandAll"},
			{Label: "Collapse All", Command: "search.collapseAll"},
			ui.MenuSep(),
			{Label: "Clear Results", Command: "search.clear"},
			ui.MenuSep(),
			{Label: "Help", Command: "search.help"},
		}
	case "changes":
		items = a.BuildChangesPanelMenu()
	case "outline":
		items = []ui.ContextMenuItem{
			{Label: "Refresh", Command: "sidebar.outline"},
		}
	case "plugins":
		items = []ui.ContextMenuItem{
			{Label: "Install from URL", Command: "plugin.install"},
			{Label: "Refresh", Command: "plugin.refresh"},
			ui.MenuSep(),
			{Label: "Help", Command: "plugin.help"},
		}
	default:
		if a.PluginManager != nil {
			for _, p := range a.PluginManager.Plugins() {
				if a.Sidebar.ActivePanel == "plugin."+p.Name && len(p.SidebarMenuEntries) > 0 {
					for _, e := range p.SidebarMenuEntries {
						items = append(items, contextMenuItemFromWidgetEntry(e))
					}
					break
				}
			}
		}
	}
	moveItems := a.sidebarMoveMenuItems()
	if len(moveItems) > 0 {
		if len(items) > 0 && !items[len(items)-1].IsSep {
			items = append(items, ui.MenuSep())
		}
		items = append(items, moveItems...)
	}
	if len(items) > 0 {
		openContextMenu(a, items, sx, sy)
	}
}

func (a *App) BuildChangesPanelMenu() []ui.ContextMenuItem {
	return []ui.ContextMenuItem{
		{Label: "View All Changes", Command: "changes.viewAll"},
		ui.MenuSep(),
		{Label: "Refresh", Command: "changes.refresh"},
		ui.MenuSep(),
		{Label: "Expand All", Command: "changes.expandAll"},
		{Label: "Collapse All", Command: "changes.collapseAll"},
		ui.MenuSep(),
		{Label: "Git Files", Submenu: a.BuildGitFileOptions()},
		{Label: "Diff View", Submenu: a.BuildDiffViewOptions()},
		ui.MenuSep(),
		{Label: "Pull", Command: "git.pull"},
		{Label: "Push", Command: "git.push"},
		{Label: "Sync", Command: "git.sync"},
		ui.MenuSep(),
		{Label: "Open PR Diff", Command: "pr.openDiff"},
		ui.MenuSep(),
		{Label: "Help", Command: "changes.help"},
	}
}

// BuildChangesContextMenu keeps the structural and presentation actions close
// to the tree while leaving network and PR operations in the panel menu.
func (a *App) BuildChangesContextMenu() []ui.ContextMenuItem {
	return []ui.ContextMenuItem{
		{Label: "View All Changes", Command: "changes.viewAll"},
		ui.MenuSep(),
		{Label: "Refresh", Command: "changes.refresh"},
		ui.MenuSep(),
		{Label: "Expand All", Command: "changes.expandAll"},
		{Label: "Collapse All", Command: "changes.collapseAll"},
		ui.MenuSep(),
		{Label: "Git Files", Submenu: a.BuildGitFileOptions()},
		{Label: "Diff View", Submenu: a.BuildDiffViewOptions()},
	}
}

func (a *App) ShowChangesContextMenu(sx, sy int) {
	openContextMenu(a, a.BuildChangesContextMenu(), sx, sy)
}

func (a *App) DiffSearchSources() []ui.DiffSearchSource {
	seen := map[string]bool{}
	sources := a.EditorGroup.DiffTabSources()
	for _, s := range sources {
		seen[s.TabName] = true
	}
	for _, g := range a.Changes.Groups() {
		if !g.IsPR {
			continue
		}
		for path, diffText := range g.PRDiffs {
			tabName := path + " (diff)"
			if seen[tabName] {
				continue
			}
			fd := diff.Parse(diffText)
			dv := ui.NewDiffViewWidget(path, fd, nil, nil, false)
			sources = append(sources, ui.DiffSearchSource{TabName: tabName, Lines: dv.CombinedLines()})
		}
	}
	return sources
}

func (a *App) NavigateToSearchMatch(path string, line, col int) {
	if strings.HasSuffix(path, " (diff)") {
		if !a.EditorGroup.SwitchToTabByPath(path) {
			filePath := strings.TrimSuffix(path, " (diff)")
			for _, g := range a.Changes.Groups() {
				if !g.IsPR {
					continue
				}
				if diffText, ok := g.PRDiffs[filePath]; ok {
					a.EditorGroup.OpenDiff(filePath, diff.Parse(diffText), nil, nil, false)
					break
				}
			}
		}
		if dv := a.EditorGroup.ActiveDiffWidget(); dv != nil {
			dv.ScrollToLine(line - 1)
			dv.ApplySearchHighlight(a.Search.Input.Text, a.Search.Options)
		}
		a.Root.SetFocus(a.EditorGroup)
		return
	}
	a.EditorGroup.OpenFile(path)
	a.EditorGroup.GoToLine(line)
	if a.Search.Input.Text != "" {
		matches, _ := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, a.Search.Input.Text, a.Search.Options)
		a.EditorGroup.SetSearch(a.Search.Input.Text, matches)
	}
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) PreviewSearchReplace(filePath string, matches []ui.SearchMatch, replacement string, opts ui.SearchOptions) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		a.StatusWarn("Cannot read file: " + err.Error())
		return
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	fd := ui.BuildReplaceDiff(filepath.Base(filePath), lines, matches, replacement, opts)
	a.EditorGroup.OpenDiff(filePath, fd, nil, nil, false)
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) ApplySearchReplace(filePath string, matches []ui.SearchMatch, replacement string, opts ui.SearchOptions) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		a.StatusWarn("Cannot read file: " + err.Error())
		return
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	newLines := ui.ApplyReplacements(lines, matches, replacement, opts)
	if err := os.WriteFile(filePath, []byte(strings.Join(newLines, "\n")+"\n"), 0644); err != nil {
		a.StatusWarn("Cannot write file: " + err.Error())
		return
	}
	a.invalidateRepositoryPath(filePath, RepositoryStatus)
	a.EditorGroup.ReloadFile(filePath)
	a.Search.Refresh()
	a.StatusNotify(fmt.Sprintf("Replaced %d matches in %s", len(matches), filepath.Base(filePath)))
}

func (a *App) ApplySearchReplaceAll(allMatches map[string][]ui.SearchMatch, replacement string, opts ui.SearchOptions) {
	totalFiles := len(allMatches)
	totalMatches := 0
	for _, m := range allMatches {
		totalMatches += len(m)
	}
	msg := fmt.Sprintf("Replace %d matches across %d files? This cannot be undone.", totalMatches, totalFiles)
	a.ShowConfirmDialog(msg, []string{"Cancel", "Replace All"}, []func(){
		func() { a.DismissDialog() },
		func() {
			a.DismissDialog()
			for filePath, matches := range allMatches {
				data, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}
				lines := strings.Split(string(data), "\n")
				if len(lines) > 0 && lines[len(lines)-1] == "" {
					lines = lines[:len(lines)-1]
				}
				newLines := ui.ApplyReplacements(lines, matches, replacement, opts)
				if err := os.WriteFile(filePath, []byte(strings.Join(newLines, "\n")+"\n"), 0644); err != nil {
					continue
				}
				a.invalidateRepositoryPath(filePath, RepositoryStatus)
				a.EditorGroup.ReloadFile(filePath)
			}
			a.Search.Refresh()
			a.StatusNotify(fmt.Sprintf("Replaced %d matches across %d files", totalMatches, totalFiles))
		},
	})
}

func (a *App) openSelectedDiff(extended bool) {
	a.Changes.OpenSelectedDiff(extended)
}

// OpenChangeDiff shows a working-tree file's changes against HEAD. The reads
// run in the background: git diff on a large file is slow enough that doing it
// from the click handler freezes the editor.
func (a *App) OpenChangeDiff(dir string, status git.FileStatus, extended bool) {
	fullPath := filepath.Join(dir, status.Path)
	if status.Status == "?" {
		// Untracked: nothing to diff against, and no git to wait for.
		a.EditorGroup.OpenFile(fullPath)
		a.FocusEditorIfEnabled()
		return
	}
	a.startDiffOpen(func() *DiffOpenResult {
		return readChangeDiff(dir, status, fullPath, extended)
	})
}

func readChangeDiff(dir string, status git.FileStatus, fullPath string, extended bool) *DiffOpenResult {
	// Falling back to the plain file is what this path has always done when
	// there is no usable diff — an unreadable one, an empty one, or one git
	// produced with no hunks in it.
	fallback := &DiffOpenResult{OpenPath: fullPath}

	var diffText string
	var err error
	if status.Status == "R" && status.OldPath != "" {
		diffText, err = git.DiffRename(dir, status.OldPath, status.Path)
	} else {
		diffText, err = git.DiffFile(dir, status.Path)
	}
	if err != nil || diffText == "" {
		return fallback
	}
	parsed := diff.Parse(diffText)
	if len(parsed.Hunks) == 0 {
		return fallback
	}

	var newLines []string
	if newData, err := os.ReadFile(fullPath); err == nil {
		newLines = splitFileLines(string(newData))
	}
	return &DiffOpenResult{
		TabName:  status.Path + " (diff)",
		Path:     status.Path,
		Diff:     parsed,
		OldLines: gitFileLines(dir, status.Path, "HEAD"),
		NewLines: newLines,
		Extended: extended,
	}
}

// OpenCommitDiff shows what one commit did to one file. Unlike OpenChangeDiff
// neither side comes from the worktree: both are read out of git at the commit
// and its first parent.
func (a *App) OpenCommitDiff(dir, ref, short string, status git.FileStatus, extended bool) {
	a.startDiffOpen(func() *DiffOpenResult {
		return readCommitDiff(dir, ref, short, status, extended)
	})
}

func readCommitDiff(dir, ref, short string, status git.FileStatus, extended bool) *DiffOpenResult {
	diffText, err := git.CommitFileDiff(dir, ref, status)
	if err != nil {
		return &DiffOpenResult{Warn: fmt.Sprintf("Could not read %s at %s", status.Path, short)}
	}
	parsed := diff.Parse(diffText)
	if len(parsed.Hunks) == 0 {
		// A pure rename or a mode change carries no hunks at all.
		return &DiffOpenResult{Warn: fmt.Sprintf("No line changes for %s in %s", status.Path, short)}
	}

	// Both reads are allowed to fail: an added file has no parent blob, a
	// deleted one has no blob at the commit, and a root commit has no parent at
	// all. OpenDiffTab renders the hunks alone in that case, exactly as PR
	// diffs do.
	oldPath := status.Path
	if status.OldPath != "" {
		oldPath = status.OldPath
	}
	return &DiffOpenResult{
		// The key uses the full hash so it stays put as the abbreviation grows;
		// the label uses the short one because that is what the panel shows.
		TabName:  fmt.Sprintf("%s:%s (diff)", ref, status.Path),
		Title:    fmt.Sprintf("%s @ %s", filepath.Base(status.Path), short),
		Path:     status.Path,
		Diff:     parsed,
		OldLines: gitFileLines(dir, oldPath, ref+"^"),
		NewLines: gitFileLines(dir, status.Path, ref),
		Extended: extended,
	}
}

func splitFileLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// gitFileLines reads a blob at a ref, returning nil when it does not exist.
func gitFileLines(dir, path, ref string) []string {
	content, err := git.ShowFile(dir, path, ref)
	if err != nil {
		return nil
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (a *App) OpenPRDiff(group *ui.ChangesGroup, status git.FileStatus, extended bool) {
	diffText, ok := group.PRDiffs[status.Path]
	if !ok || diffText == "" {
		a.StatusWarn("No diff available for " + status.Path)
		return
	}
	parsed := diff.Parse(diffText)
	if len(parsed.Hunks) == 0 {
		a.StatusWarn("Empty diff for " + status.Path)
		return
	}
	a.EditorGroup.OpenDiff(status.Path, parsed, nil, nil, false)
	if dv := a.EditorGroup.ActiveDiffWidget(); dv != nil {
		dv.OnFetchExtended = func(dv *ui.DiffViewWidget) {
			a.fetchPRFileContent(dv, group.PROwner, group.PRRepo, group.PRBaseSHA, group.PRHeadSHA, status.Path)
		}
		if extended {
			dv.SetExtended(true)
		}
	}
	a.FocusEditorIfEnabled()
}

func (a *App) fetchPRFileContent(dv *ui.DiffViewWidget, owner, repo, baseSHA, headSHA, path string) {
	if owner == "" || baseSHA == "" {
		dv.Loading = false
		return
	}
	tabName := path + " (diff)"
	go func() {
		var oldLines, newLines []string
		var fetchErr error
		if content, err := github.FetchFileContent(owner, repo, path, baseSHA); err == nil {
			oldLines = strings.Split(content, "\n")
			if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
				oldLines = oldLines[:len(oldLines)-1]
			}
		} else {
			fetchErr = err
		}
		if content, err := github.FetchFileContent(owner, repo, path, headSHA); err == nil {
			newLines = strings.Split(content, "\n")
			if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
				newLines = newLines[:len(newLines)-1]
			}
		} else if fetchErr == nil {
			fetchErr = err
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(&DiffContentResult{
			TabName:  tabName,
			OldLines: oldLines,
			NewLines: newLines,
			Err:      fetchErr,
		}))
	}()
}

func (a *App) ShowPRGroupMenu(group *ui.ChangesGroup, sx, sy int) {
	reg := a.Reg
	name := group.Name
	url := group.PRURL
	refreshID := "pr.refresh." + name
	closeID := "pr.close." + name
	reg.Register(command.Command{
		ID: refreshID, Title: "Refresh",
		Handler: func() {
			a.Changes.RemovePRGroup(name)
			a.FetchAndOpenPR(url)
		},
	})
	reg.Register(command.Command{
		ID: closeID, Title: "Close",
		Handler: func() {
			a.Changes.RemovePRGroup(name)
		},
	})
	items := []ui.ContextMenuItem{
		{Label: "Refresh PR", Command: refreshID},
		{Label: "Close", Command: closeID},
	}
	items = append(items, ui.MenuSep())
	items = append(items, a.BuildChangesContextMenu()...)
	openContextMenu(a, items, sx, sy)
}

func (a *App) ShowGroupMenu(dir string, sx, sy int) {
	reg := a.Reg
	items := []ui.ContextMenuItem{
		{Label: "Pull", Command: "git.pull." + dir},
		{Label: "Push", Command: "git.push." + dir},
		{Label: "Sync", Command: "git.sync." + dir},
	}
	items = append(items, ui.MenuSep())
	items = append(items, a.BuildChangesContextMenu()...)
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
	registerDirGitCmd("git.pull."+dir, "Pull", []RepoOp{OpPull}, "Pulling", "Pulled successfully")
	registerDirGitCmd("git.push."+dir, "Push", []RepoOp{OpPush}, "Pushing", "Pushed successfully")
	registerDirGitCmd("git.sync."+dir, "Sync", []RepoOp{OpPull, OpPush}, "Syncing", "Synced successfully")
	openContextMenu(a, items, sx, sy)
}

func (a *App) CommitChanges(dir string, message string) {
	a.RunRepoTask(RepoTask{
		Progress: "Committing",
		Done:     "Committed: " + message,
		Dirs:     []string{dir},
		Ops:      []RepoOp{OpCommit(message)},
		OnDone:   func() { a.Changes.ClearInput(dir) },
	})
}

func (a *App) ConfirmDiscard(message string, onConfirm func()) {
	a.ShowConfirmDialogEx("Discard Changes?", message,
		[]string{"Cancel", "Discard"},
		[]func(){
			func() { a.DismissDialog() },
			func() {
				a.DismissDialog()
				onConfirm()
			},
		},
	)
}

func registerWidgetCallbacks(app *App) {
	reg := app.Reg
	// Any foreground navigation supersedes a diff read that has not landed yet.
	// Put invalidation on the editor group's active-content boundary so direct
	// callers such as plugins cannot bypass it.
	app.EditorGroup.OnActiveContentChange = func() {
		app.cancelPendingDiff()
		app.syncRepositoryObservation()
	}

	for i := range menuBarMenus {
		idx := i
		reg.Register(command.Command{
			ID:    menuBarLabels[idx],
			Title: "Menu: " + app.MenuBar.Items[idx].Name,
			Handler: func() {
				openMenuBarDropdown(app, idx)
			},
		})
	}

	app.MenuBar.OnSelect = func(index int) {
		openMenuBarDropdown(app, index)
	}

	app.Root.OnRightClick = func(mx, my int) {
		handleRightClick(app, mx, my)
	}

	app.SplitPanel.OnLeftClick = func() {
		reg.Execute("sidebar.focus")
	}
	app.SplitPanel.OnRightClick = func() {}

	app.Sidebar.Tabs.Config.Actions = []widgets.TabAction{
		{Icon: "⋮", OnClick: app.ShowSidebarMoreMenu},
	}
	app.Sidebar.OnPanelReorder = app.persistSidebarPanelOrder

	app.Sidebar.Tabs.Config.OnOverflow = func(sx, sy int) {
		ids, titles := app.Sidebar.HiddenTabs()
		var items []ui.ContextMenuItem
		for i, id := range ids {
			panelID := id
			items = append(items, ui.ContextMenuItem{Label: titles[i], Command: "sidebar.overflow." + panelID})
			reg.Register(command.Command{
				ID:      "sidebar.overflow." + panelID,
				Title:   titles[i],
				Handler: func() { app.Sidebar.SetActivePanel(panelID) },
			})
		}
		openContextMenu(app, items, sx, sy)
	}

	app.Sidebar.OnPanelChange = func(id string) {
		if id == "search" {
			app.applySearchHighlights()
		} else {
			app.EditorGroup.ClearSearch()
		}
		app.syncRepositoryObservation()
		if id == "outline" {
			app.RefreshSymbols()
		}
	}

	revealSymbol := func(line, col int) {
		app.EditorGroup.GoToLine(line + 1)
		if app.EditorGroup.IsEditorActive() {
			editor := app.EditorGroup.Editor
			if editor.Cursor.Line < len(editor.Buf.Lines) {
				runes := []rune(editor.Buf.Lines[editor.Cursor.Line])
				if col > 0 && col <= len(runes) {
					editor.Cursor.Col = col
				}
			}
		}
	}
	app.Symbols.OnReveal = revealSymbol
	app.Symbols.OnJump = func(line, col int) {
		revealSymbol(line, col)
		app.Root.SetFocus(app.EditorGroup)
	}

	app.EditorGroup.TabBar.OnTabClose = func(index int) {
		app.EditorGroup.SwitchTab(index)
		reg.Execute("tab.close")
	}

	app.EditorGroup.TabBar.OnTabUnpin = func(index int) {
		app.EditorGroup.SwitchTab(index)
		app.EditorGroup.TogglePinTab()
	}

	app.EditorGroup.TabBar.MoreButton.OnClick = func(sx, sy int) {
		moreMenu := []ui.ContextMenuItem{
			{Label: "Close All", Command: "tab.closeAll"},
			{Label: "Close All Saved", Command: "tab.closeAllSaved"},
		}
		openContextMenu(app, moreMenu, sx, sy)
	}

	app.EditorGroup.TabBar.OnTabRightClick = func(index, sx, sy int) {
		app.EditorGroup.SwitchTab(index)
		pinLabel := "Pin Tab"
		if app.EditorGroup.IsActiveTabPinned() {
			pinLabel = "Unpin Tab"
		}
		tabContextMenu := []ui.ContextMenuItem{
			{Label: pinLabel, Shortcut: app.KeyFor("tab.pin"), Command: "tab.pin"},
		}
		if app.EditorGroup.CanMoveActiveTab(-1) {
			tabContextMenu = append(tabContextMenu, ui.ContextMenuItem{Label: "Move Tab Left", Command: "tab.moveLeft"})
		}
		if app.EditorGroup.CanMoveActiveTab(1) {
			tabContextMenu = append(tabContextMenu, ui.ContextMenuItem{Label: "Move Tab Right", Command: "tab.moveRight"})
		}
		tabContextMenu = append(tabContextMenu,
			ui.MenuSep(),
			ui.ContextMenuItem{Label: "Close", Shortcut: app.KeyFor("tab.close"), Command: "tab.close"},
			ui.ContextMenuItem{Label: "Close Others", Shortcut: "", Command: "tab.closeOthers"},
			ui.ContextMenuItem{Label: "Close All", Shortcut: "", Command: "tab.closeAll"},
			ui.ContextMenuItem{Label: "Close All Saved", Shortcut: "", Command: "tab.closeAllSaved"},
			ui.MenuSep(),
			ui.ContextMenuItem{Label: "Copy Absolute Path", Command: "file.copyAbsolutePath"},
			ui.ContextMenuItem{Label: "Copy Relative Path", Command: "file.copyRelativePath"},
		)
		if dv := app.EditorGroup.ActiveDiffWidget(); dv != nil {
			cmd := "diff.extendedView"
			label := "Extended Diff"
			if dv.IsExtended() {
				cmd = "diff.compactView"
				label = "Compact Diff"
			}
			tabContextMenu = append(tabContextMenu,
				ui.MenuSep(),
				ui.ContextMenuItem{Label: label, Command: cmd},
			)
		}
		openContextMenu(app, tabContextMenu, sx, sy)
	}

	app.Explorer.OnOpenFile = func(path string) {
		app.EditorGroup.OpenFile(path)
		app.FocusEditorIfEnabled()
	}
	app.Explorer.OnRightClick = func(node *widgets.TreeNode, sx, sy int) {
		app.ExplorerContextNode = node
		items := []ui.ContextMenuItem{
			{Label: "Open", Command: "explorer.open"},
			ui.MenuSep(),
			{Label: "New File", Command: "explorer.newFile"},
			{Label: "New Folder", Command: "explorer.newFolder"},
			ui.MenuSep(),
			{Label: "Copy Absolute Path", Command: "explorer.copyAbsolutePath"},
			{Label: "Copy Relative Path", Command: "explorer.copyRelativePath"},
			ui.MenuSep(),
			{Label: "Reveal in File Manager", Command: "explorer.reveal"},
			ui.MenuSep(),
			{Label: "Rename", Command: "explorer.rename"},
			{Label: "Delete", Command: "explorer.delete"},
		}
		openContextMenu(app, items, sx, sy)
	}
	app.Explorer.OnRootMenu = func(node *widgets.TreeNode, sx, sy int) {
		app.ExplorerContextNode = node
		items := []ui.ContextMenuItem{
			{Label: "Refresh", Command: "explorer.refresh"},
			{Label: "Copy Path", Command: "explorer.copyAbsolutePath"},
			ui.MenuSep(),
			{Label: "Remove from Workspace", Command: "explorer.removeRoot"},
		}
		openContextMenu(app, items, sx, sy)
	}

	app.Search.OnClear = func() {
		app.EditorGroup.ClearSearch()
	}
	app.Search.PostBatch = func(batch *ui.SearchBatch) {
		app.Screen.PostEvent(tcell.NewEventInterrupt(batch))
	}
	app.Search.DiffSources = app.DiffSearchSources
	app.Search.OnOpenMatch = app.NavigateToSearchMatch
	app.Search.OnPreview = app.PreviewSearchReplace
	app.Search.OnReplace = app.ApplySearchReplace
	app.Search.OnReplaceAll = app.ApplySearchReplaceAll

	app.Changes.OnRightClick = func(dir string, status git.FileStatus, sx, sy int) {
		var items []ui.ContextMenuItem
		if status.Staged {
			items = append(items, changesContextMenuStaged...)
		} else {
			items = append(items, changesContextMenuUnstaged...)
		}
		items = append(items, ui.MenuSep())
		items = append(items, app.BuildChangesContextMenu()...)
		openContextMenu(app, items, sx, sy)
	}
	app.Changes.OnPanelMenu = app.ShowChangesContextMenu

	app.Changes.OnOpenFile = func(path string) {
		app.EditorGroup.OpenFile(path)
		app.FocusEditorIfEnabled()
	}
	app.Changes.OnOpenDiff = func(dir string, status git.FileStatus, extended bool) {
		app.OpenChangeDiff(dir, status, extended)
	}
	app.Changes.OnOpenPRDiff = func(group *ui.ChangesGroup, status git.FileStatus, extended bool) {
		app.OpenPRDiff(group, status, extended)
	}
	app.Changes.OnOpenCommitDiff = app.OpenCommitDiff
	app.Changes.OnOpenCommit = app.OpenCommitDetail
	app.Changes.OnPRGroupMenu = app.ShowPRGroupMenu
	app.Changes.OnRefreshPR = app.FetchAndOpenPR
	app.Changes.OnGroupMenu = app.ShowGroupMenu
	app.Changes.OnCommit = app.CommitChanges
	app.Changes.OnConfirmDiscard = app.ConfirmDiscard
	app.Changes.OnError = app.StatusError
	app.Changes.OnRefresh = func() {
		if app.Repository != nil {
			app.Repository.RefreshNow(RepositoryStatus | RepositoryHistory)
		} else {
			app.Changes.Refresh()
		}
	}
	app.Changes.OnStatusChanged = func() {
		app.invalidateAllRepositories(RepositoryStatus)
	}
	app.Changes.OnHistoryResult = func(err error) {
		if app.Repository != nil {
			app.Repository.HandleHistory(err)
		}
	}

	app.ContentSplit.OnResize = func(height int) {
		if height <= 0 {
			app.ContentSplit.ShowBottom = false
		} else {
			app.ContentSplit.ShowBottom = true
			app.ContentSplit.BottomH = height
			if len(app.Terminals) == 0 {
				app.SpawnTerminal()
			}
		}
	}

	app.ContentSplit.OnTopClick = func() {
		app.Root.SetFocus(app.EditorGroup)
	}

	app.ContentSplit.OnBottomClick = func() {
		if w := app.BottomPanel.ActiveWidget(); w != nil {
			app.Root.SetFocus(w)
		}
	}

	app.SplitPanel.OnResize = func(width int) {
		app.SetSidebarWidth(width)
	}

	app.BottomPanel.Tabs.Config.OnTabClick = func(index int) {
		panels := app.BottomPanel.PanelIDs()
		if index >= 0 && index < len(panels) {
			app.BottomPanel.SetActivePanel(panels[index])
			if w := app.BottomPanel.ActiveWidget(); w != nil {
				app.Root.SetFocus(w)
			}
		}
	}

	app.BottomPanel.Tabs.Config.Actions = []widgets.TabAction{
		{Icon: "+", OnClick: func(_, _ int) {
			reg.Execute("terminal.new")
		}},
		{Icon: "⋮", OnClick: func(sx, sy int) {
			items := []ui.ContextMenuItem{
				{Label: "New Terminal", Command: "terminal.new"},
				ui.MenuSep(),
				{Label: "Close All Terminals", Command: "terminal.closeAll"},
				ui.MenuSep(),
				{Label: "Close Panel", Command: "panel.toggle"},
			}
			openContextMenu(app, items, sx, sy)
		}},
	}
}
