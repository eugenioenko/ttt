package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func (a *App) openFileLink(path string, line, col int) {
	if err := a.EditorGroup.OpenFile(path); err != nil {
		a.StatusWarn(err.Error())
		return
	}
	if line > 0 {
		a.EditorGroup.GoToLineCol(line, col)
	}
	a.FocusEditor()
}

func (a *App) ShowSidebarMoreMenu(sx, sy int) {
	var items []ui.ContextMenuItem
	switch a.Sidebar.ActivePanel {
	case "explorer":
		items = []ui.ContextMenuItem{
			{Label: i18n.Text("explorer.searchInFolders"), Command: "explorer.find"},
			ui.MenuSep(),
			{Label: i18n.Text("general.newFile"), Command: "file.new"},
			{Label: i18n.Text("general.addFolder"), Command: "workspace.addFolder"},
			{Label: i18n.Text("general.refresh"), Command: "explorer.refresh"},
			ui.MenuSep(),
			{Label: i18n.Text("general.expandAll"), Command: "explorer.expandAll"},
			{Label: i18n.Text("general.collapseAll"), Command: "explorer.collapseAll"},
			ui.MenuSep(),
			{Label: i18n.Text("general.help"), Command: "explorer.help"},
		}
	case "search":
		replaceLabel := i18n.Text("general.replace")
		if a.Search.IsReplaceMode() {
			replaceLabel = i18n.Text("general.search")
		}
		items = []ui.ContextMenuItem{
			{Label: replaceLabel, Command: "sidebar.searchReplace"},
			ui.MenuSep(),
			{Label: i18n.Text("general.expandAll"), Command: "search.expandAll"},
			{Label: i18n.Text("general.collapseAll"), Command: "search.collapseAll"},
			ui.MenuSep(),
			{Label: i18n.Text("menu.clearResults"), Command: "search.clear"},
			ui.MenuSep(),
			{Label: i18n.Text("general.help"), Command: "search.help"},
		}
	case "changes":
		items = a.BuildChangesPanelMenu()
	case "outline":
		items = []ui.ContextMenuItem{
			{Label: i18n.Text("general.refresh"), Command: "sidebar.outline"},
		}
	case appsPanelID:
		items = []ui.ContextMenuItem{{Label: i18n.Text("apps.add"), Command: "apps.add"}}
	case "plugins":
		items = []ui.ContextMenuItem{
			{Label: i18n.Text("menu.installFromURL"), Command: "plugin.install"},
			{Label: i18n.Text("general.refresh"), Command: "plugin.refresh"},
			ui.MenuSep(),
			{Label: i18n.Text("general.help"), Command: "plugin.help"},
		}
	default:
		if a.PluginManager != nil {
			for _, p := range a.PluginManager.Plugins() {
				if a.Sidebar.ActivePanel == "plugin."+p.Name && len(p.SidebarMenuEntries) > 0 {
					items = contextMenuItemsFromWidgetEntries(p.SidebarMenuEntries)
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
			sources = append(sources, ui.DiffSearchSource{TabName: tabName, IconPath: path, Lines: ui.DiffCombinedLines(diff.Parse(diffText))})
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
			dv.ApplySearchHighlight(a.Search.Input.Text(), a.Search.Options)
		}
		a.Root.SetFocus(a.EditorGroup)
		return
	}
	a.EditorGroup.OpenFile(path)
	a.EditorGroup.GoToLine(line)
	if a.EditorGroup.IsEditorActive() {
		text := a.EditorGroup.Editor.Buf.Lines[a.EditorGroup.Editor.Cursor.Line]
		runeCol := utf8.RuneCountInString(text[:min(col, len(text))])
		a.EditorGroup.GoToLineCol(line, runeCol+1)
	}
	if a.Search.Input.Text() != "" {
		matches, _ := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, a.Search.Input.Text(), a.Search.Options)
		a.EditorGroup.SetSearch(a.Search.Input.Text(), a.Search.Options, matches)
	}
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) PreviewSearchReplace(filePath string, matches []ui.SearchMatch, replacement string, opts ui.SearchOptions) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		a.StatusWarn(i18n.Format("status.cannotReadFile", i18n.Args{"error": err.Error()}))
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

// staleSearchMatches reports whether any match's line no longer reads as it did
// when the search ran, so its columns may point at other text.
func staleSearchMatches(lines []string, matches []ui.SearchMatch) bool {
	for _, m := range matches {
		idx := m.LineNum - 1
		if idx < 0 || idx >= len(lines) {
			return true
		}
		// rg sends non-UTF-8 lines as bytes, which leaves LineText empty.
		if m.LineText != "" && strings.TrimRight(lines[idx], "\r") != m.LineText {
			return true
		}
	}
	return false
}

// joinReplacedLines ends the text with a newline only if the original did.
func joinReplacedLines(lines []string, original []byte) []byte {
	text := strings.Join(lines, "\n")
	if strings.HasSuffix(string(original), "\n") {
		text += "\n"
	}
	return []byte(text)
}

func (a *App) ApplySearchReplace(filePath string, matches []ui.SearchMatch, replacement string, opts ui.SearchOptions) {
	if a.EditorGroup.IsDirtyPath(filePath) {
		a.StatusWarn(i18n.Format("status.hasUnsavedChangesSaveItBeforeReplacing", i18n.Args{"filename": filepath.Base(filePath)}))
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		a.StatusWarn(i18n.Format("status.cannotReadFile", i18n.Args{"error": err.Error()}))
		return
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if staleSearchMatches(lines, matches) {
		a.StatusWarn(i18n.Format("status.changedSinceTheSearchNothingReplaced", i18n.Args{"filename": filepath.Base(filePath)}))
		a.Search.Refresh()
		return
	}
	newLines := ui.ApplyReplacements(lines, matches, replacement, opts)
	if err := os.WriteFile(filePath, joinReplacedLines(newLines, data), 0644); err != nil {
		a.StatusWarn(i18n.Format("status.cannotWriteFile", i18n.Args{"error": err.Error()}))
		return
	}
	a.invalidateRepositoryPath(filePath, RepositoryWorktree)
	a.EditorGroup.ReloadFile(filePath)
	a.Search.Refresh()
	a.StatusNotify(i18n.Format("status.replacedMatchesIn", i18n.Args{"matchesCount": len(matches), "filename": filepath.Base(filePath)}))
}

func (a *App) ApplySearchReplaceAll(allMatches map[string][]ui.SearchMatch, replacement string, opts ui.SearchOptions) {
	a.confirmSearchReplaceAll(allMatches, replacement, opts, nil)
}

func (a *App) confirmSearchReplaceAll(allMatches map[string][]ui.SearchMatch, replacement string, opts ui.SearchOptions, current func() bool) {
	totalFiles := len(allMatches)
	totalMatches := 0
	for _, m := range allMatches {
		totalMatches += len(m)
	}
	msg := i18n.Format("search.confirmReplaceAll", i18n.Args{"totalMatches": totalMatches, "totalFiles": totalFiles})
	a.ShowConfirmDialog(msg, []string{i18n.Text("general.cancel"), i18n.Text("general.replaceAll")}, []func(){
		func() {
			a.DismissDialog()
			if current != nil {
				a.cancelSearchReplace()
			}
		},
		func() {
			if current != nil && !current() {
				return
			}
			a.DismissDialog()
			invalidatePath := ""
			replacedMatches, replacedFiles := 0, 0
			var skipped []string
			for filePath, matches := range allMatches {
				if a.EditorGroup.IsDirtyPath(filePath) {
					skipped = append(skipped, filepath.Base(filePath))
					continue
				}
				data, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}
				lines := strings.Split(string(data), "\n")
				if len(lines) > 0 && lines[len(lines)-1] == "" {
					lines = lines[:len(lines)-1]
				}
				if staleSearchMatches(lines, matches) {
					skipped = append(skipped, filepath.Base(filePath))
					continue
				}
				newLines := ui.ApplyReplacements(lines, matches, replacement, opts)
				if err := os.WriteFile(filePath, joinReplacedLines(newLines, data), 0644); err != nil {
					continue
				}
				replacedMatches += len(matches)
				replacedFiles++
				if invalidatePath == "" {
					invalidatePath = filePath
				}
				a.EditorGroup.ReloadFile(filePath)
			}
			a.invalidateRepositoryPath(invalidatePath, RepositoryWorktree)
			a.Search.Refresh()
			msg := i18n.Format("search.replacedAcrossFiles", i18n.Args{"replacedMatches": replacedMatches, "replacedFiles": replacedFiles})
			if len(skipped) > 0 {
				sort.Strings(skipped)
				a.StatusWarn(i18n.Format("status.skippedUnsavedOrChangedSinceTheSearch", i18n.Args{"msg": msg, "items": strings.Join(skipped, ", ")}))
				return
			}
			a.StatusNotify(msg)
		},
	})
}

func (a *App) openSelectedDiff(extended bool) { a.Changes.OpenSelectedDiff(extended) }

func (a *App) OpenChangeDiff(dir string, change scm.Change, extended bool) {
	a.cancelPendingDiff()
	fullPath := filepath.Join(dir, change.Path)
	working := change.Group == "" || change.Group == scm.WorkingTree
	if working && change.Kind == scm.Untracked {
		a.EditorGroup.OpenFile(fullPath)
		a.FocusEditorIfEnabled()
		return
	}
	if working && change.Kind != scm.Deleted {
		if info, err := os.Lstat(fullPath); err == nil && info.Mode().IsRegular() {
			opened, conflict, err := a.openInlineDiff(fullPath, dir, change.Path)
			if err != nil && !errors.Is(err, errors.ErrUnsupported) {
				a.StatusError(err.Error())
				a.FocusEditorIfEnabled()
				return
			}
			if conflict {
				a.openConflictedFile(fullPath, change.Path)
				return
			}
			if opened {
				if d := a.EditorGroup.ActiveInlineDiff(); d != nil && extended {
					d.SetContextMode(ui.DiffContextFullFile)
				}
				a.FocusEditorIfEnabled()
				return
			}
		}
	}
	source := a.Repository.repositorySource(dir)
	a.startDiffOpen(func(ctx context.Context) *diffOpenResult {
		return readChangeDiff(ctx, source, change, extended)
	})
}

func readChangeDiff(ctx context.Context, source repositorySource, change scm.Change, extended bool) *diffOpenResult {
	fullPath := filepath.Join(source.root, change.Path)
	content, err := source.readChange(ctx, change)
	if err != nil {
		return &diffOpenResult{Canceled: errors.Is(err, context.Canceled), Warn: i18n.Format("status.couldNotOpenDiff", i18n.Args{"error": err.Error()})}
	}
	if content.Conflicted {
		return &diffOpenResult{Path: fullPath, Conflict: change.Path}
	}
	if content.Patch == "" {
		message := i18n.Format("status.noDiffAvailableFor", i18n.Args{"path": change.Path})
		if change.Group == scm.Staged {
			message = i18n.Format("status.noStagedChangesFor", i18n.Args{"path": change.Path})
		}
		return &diffOpenResult{Warn: message}
	}
	if isBinaryContent([]byte(content.Before.Content)) || isBinaryContent([]byte(content.After.Content)) {
		return &diffOpenResult{Warn: i18n.Text("changes.binaryFileChanged")}
	}
	parsed := diff.Parse(content.Patch)
	suffix := " (diff)"
	if change.Group == scm.Staged {
		suffix = " (staged)"
	}
	var oldLines, newLines []string
	if content.Before.Exists {
		oldLines = diffSideLines(content.Before.Content)
	}
	if content.After.Exists {
		newLines = diffSideLines(content.After.Content)
	}
	return &diffOpenResult{TabName: change.Path + suffix, Path: fullPath, Diff: parsed,
		OldLines: oldLines, NewLines: newLines,
		Extended: extended || len(parsed.Hunks) == 0}
}

func (a *App) openConflictedFile(fullPath, relPath string) {
	a.EditorGroup.OpenFile(fullPath)
	a.StatusNotify(i18n.Format("status.hasMergeConflictsOpenedWithoutADiff", i18n.Args{"relPath": relPath}))
	a.FocusEditorIfEnabled()
}

func diffSideLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (a *App) OpenCommitDiff(dir, ref, short string, status scm.Change, extended bool) {
	source := a.Repository.repositorySource(dir)
	a.startDiffOpen(func(ctx context.Context) *diffOpenResult {
		return readCommitDiff(ctx, source, ref, short, status, extended)
	})
}

func readCommitDiff(ctx context.Context, source repositorySource, ref, short string, status scm.Change, extended bool) *diffOpenResult {
	reader, err := source.revisionReader(ctx)
	if err != nil {
		return &diffOpenResult{Canceled: errors.Is(err, context.Canceled), Warn: i18n.Format("git.readFileAtCommitFailed", i18n.Args{"path": status.Path, "short": short})}
	}
	diffText, err := reader.ReadRevisionPatch(ctx, ref, status)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return &diffOpenResult{Canceled: true}
		}
		return &diffOpenResult{Warn: i18n.Format("git.readFileAtCommitFailed", i18n.Args{"path": status.Path, "short": short})}
	}
	parsed := diff.Parse(diffText)
	if len(parsed.Hunks) == 0 {
		return &diffOpenResult{Warn: i18n.Format("git.noLineChangesInCommit", i18n.Args{"path": status.Path, "short": short})}
	}
	content, err := reader.ReadRevisionContent(ctx, ref, status.Path, status.OldPath)
	if ctx.Err() != nil {
		return &diffOpenResult{Canceled: true}
	}
	if err != nil {
		return &diffOpenResult{Warn: i18n.Format("git.readFileAtCommitFailed", i18n.Args{"path": status.Path, "short": short})}
	}
	var oldLines, newLines []string
	if content.Before.Exists {
		oldLines = splitCurrentChangesLines([]byte(content.Before.Content))
	}
	if content.After.Exists {
		newLines = splitCurrentChangesLines([]byte(content.After.Content))
	}

	return &diffOpenResult{
		TabName:  fmt.Sprintf("%s:%s (diff)", ref, status.Path),
		Title:    fmt.Sprintf("%s @ %s", filepath.Base(status.Path), short),
		Path:     filepath.Join(source.root, status.Path),
		Diff:     parsed,
		OldLines: oldLines,
		NewLines: newLines,
		Extended: extended,
	}
}

func (a *App) OpenPRDiff(group *ui.ChangesGroup, status git.FileStatus, extended bool) {
	diffText, ok := group.PRDiffs[status.Path]
	if !ok || diffText == "" {
		a.StatusWarn(i18n.Format("status.noDiffAvailableFor", i18n.Args{"path": status.Path}))
		return
	}
	parsed := diff.Parse(diffText)
	if len(parsed.Hunks) == 0 {
		a.StatusWarn(i18n.Format("status.emptyDiffFor", i18n.Args{"path": status.Path}))
		return
	}
	a.EditorGroup.OpenDiff(status.Path, parsed, nil, nil, false)
	if dv := a.EditorGroup.ActiveDiffWidget(); dv != nil {
		dv.SetExtendedFetcher(func(dv *ui.DiffEditorWidget) {
			a.fetchPRFileContent(dv, group.PROwner, group.PRRepo, group.PRBaseSHA, group.PRHeadSHA, status.Path)
		})
		if extended {
			dv.SetExtended(true)
		}
	}
	a.FocusEditorIfEnabled()
}

func (a *App) fetchPRFileContent(dv *ui.DiffEditorWidget, owner, repo, baseSHA, headSHA, path string) {
	if owner == "" || baseSHA == "" {
		dv.FailLoading()
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
		ID: refreshID, Title: i18n.Text("general.refresh"),
		Handler: func() {
			a.Changes.RemovePRGroup(name)
			a.FetchAndOpenPR(url)
		},
	})
	reg.Register(command.Command{
		ID: closeID, Title: i18n.Text("general.close"),
		Handler: func() {
			a.Changes.RemovePRGroup(name)
		},
	})
	items := []ui.ContextMenuItem{
		{Label: i18n.Text("general.refresh"), Command: refreshID},
		{Label: i18n.Text("general.close"), Command: closeID},
	}
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

func (a *App) CommitChanges(dir string, message string) {
	a.RunRepoTask(RepoTask{
		Progress: i18n.Text("git.committing"),
		Done:     i18n.Format("git.committed", i18n.Args{"message": message}),
		Dirs:     []string{dir},
		Ops:      []RepoOp{a.Changes.commitOp(dir, message)},
		OnDone:   func() { a.Changes.ClearInput(dir) },
	})
}

func (a *App) ConfirmDiscard(message string, onConfirm func()) {
	a.ShowConfirmDialogEx(i18n.Text("dialog.discardChanges"), message,
		[]string{i18n.Text("general.cancel"), i18n.Text("general.discard")},
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

	for i := range menuBarMenus {
		idx := i
		reg.Register(command.Command{
			ID:    menuBarLabels[idx],
			Title: i18n.Format("status.menu", i18n.Args{"name": app.MenuBar.Items[idx].Name}),
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

	// A click that opened a dialog (a welcome or empty-Explorer action runs on
	// the press) or claimed focus elsewhere must leave the focus there, or
	// typing goes nowhere.
	app.SplitPanel.OnLeftClick = func() {
		if app.Root.HasModalOverlay() || app.Root.FocusClaimed() {
			return
		}
		reg.Execute("sidebar.focus")
	}
	app.SplitPanel.OnRightClick = func() {}

	app.Sidebar.Tabs.Config.Actions = []widgets.TabAction{
		{Icon: string(icons.Rune(icons.More)), OnClick: app.ShowSidebarMoreMenu},
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
		} else {
			app.stopSymbolsTask()
		}
		if id == "explorer" {
			app.Explorer.Reload()
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

	app.EditorGroup.OnActiveTabChange = func() {
		app.Root.ClearEditorLinkHover()
		if findBar, ok := app.Root.TopOverlayWidget().(*ui.FindBarWidget); ok && findBar.OnEditorTabChange != nil {
			findBar.OnEditorTabChange()
		}
		if bar := app.findBar(); bar != nil && bar.Query() != "" {
			return
		}
		app.applySearchHighlights()
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
		openContextMenu(app, app.tabMoreMenu(), sx, sy)
	}

	app.EditorGroup.TabBar.OnTabRightClick = func(index, sx, sy int) {
		app.EditorGroup.SwitchTab(index)
		pinLabel := i18n.Text("menu.pinTab")
		if app.EditorGroup.IsActiveTabPinned() {
			pinLabel = i18n.Text("menu.unpinTab")
		}
		tabContextMenu := []ui.ContextMenuItem{
			{Label: i18n.Text("general.close"), Shortcut: app.KeyFor("tab.close"), Command: "tab.close"},
			{Label: i18n.Text("menu.closeOthers"), Command: "tab.closeOthers"},
			{Label: i18n.Text("menu.closeAll"), Command: "tab.closeAll"},
			{Label: i18n.Text("menu.closeAllSaved"), Command: "tab.closeAllSaved"},
			ui.MenuSep(),
			{Label: pinLabel, Shortcut: app.KeyFor("tab.pin"), Command: "tab.pin"},
		}
		if app.EditorGroup.CanMoveActiveTab(-1) {
			tabContextMenu = append(tabContextMenu, ui.ContextMenuItem{Label: i18n.Text("menu.moveTabLeft"), Command: "tab.moveLeft"})
		}
		if app.EditorGroup.CanMoveActiveTab(1) {
			tabContextMenu = append(tabContextMenu, ui.ContextMenuItem{Label: i18n.Text("menu.moveTabRight"), Command: "tab.moveRight"})
		}
		tabContextMenu = append(tabContextMenu,
			ui.MenuSep(),
			ui.ContextMenuItem{Label: i18n.Text("menu.copyAbsolutePath"), Command: "file.copyAbsolutePath"},
			ui.ContextMenuItem{Label: i18n.Text("menu.copyRelativePath"), Command: "file.copyRelativePath"},
			ui.ContextMenuItem{Label: i18n.Text("menu.revealInExplorer"), Command: "explorer.revealActiveFile"},
		)
		tabContextMenu = app.withActiveDiffViewSubmenu(tabContextMenu)
		openContextMenu(app, tabContextMenu, sx, sy)
	}

	app.Explorer.OnAction = func(id string) { reg.Execute(id) }
	app.EditorGroup.OnEmpty = func() {
		if app.welcomeWhenEmpty && len(app.Workspace.Paths()) == 0 {
			app.ShowEmptyState()
		}
	}
	app.Explorer.OnOpenFile = func(path string) {
		app.EditorGroup.OpenFile(path)
		app.FocusEditorIfEnabled()
	}
	app.Explorer.OnRightClick = func(node *widgets.TreeNode, sx, sy int) {
		app.ExplorerContextNode = node
		items := []ui.ContextMenuItem{
			{Label: i18n.Text("general.open"), Command: "explorer.open"},
			app.appsOpenWithMenu(),
			ui.MenuSep(),
			{Label: i18n.Text("general.newFile"), Command: "explorer.newFile"},
			{Label: i18n.Text("general.newFolder"), Command: "explorer.newFolder"},
			ui.MenuSep(),
			{Label: i18n.Text("menu.copyAbsolutePath"), Command: "explorer.copyAbsolutePath"},
			{Label: i18n.Text("menu.copyRelativePath"), Command: "explorer.copyRelativePath"},
			ui.MenuSep(),
			{Label: i18n.Text("menu.revealInFileManager"), Command: "explorer.reveal"},
			{Label: i18n.Text("menu.openInTerminal"), Command: "explorer.openInTerminal"},
			ui.MenuSep(),
			{Label: i18n.Text("general.rename"), Command: "explorer.rename"},
			{Label: i18n.Text("general.delete"), Command: "explorer.delete"},
			ui.MenuSep(),
			{Label: i18n.Text("general.expandAll"), Command: "explorer.expandAll"},
			{Label: i18n.Text("general.collapseAll"), Command: "explorer.collapseAll"},
		}
		openContextMenu(app, items, sx, sy)
	}
	app.Explorer.OnRootMenu = func(node *widgets.TreeNode, sx, sy int) {
		app.ExplorerContextNode = node
		favorite := ui.ContextMenuItem{Label: i18n.Text("menu.addToFavorites"), Command: "welcome.addFavorite"}
		if app.favoriteIndex(node.ID) >= 0 {
			favorite = ui.ContextMenuItem{Label: i18n.Text("menu.removeFromFavorites"), Command: "welcome.removeFavorite"}
		}
		items := []ui.ContextMenuItem{
			{Label: i18n.Text("general.refresh"), Command: "explorer.refresh"},
			app.appsOpenWithMenu(),
			{Label: i18n.Text("menu.copyPath"), Command: "explorer.copyAbsolutePath"},
			{Label: i18n.Text("menu.openInTerminal"), Command: "explorer.openInTerminal"},
			ui.MenuSep(),
			favorite,
			{Label: i18n.Text("menu.removeFromWorkspace"), Command: "explorer.removeRoot"},
			ui.MenuSep(),
			{Label: i18n.Text("general.expandAll"), Command: "explorer.expandAll"},
			{Label: i18n.Text("general.collapseAll"), Command: "explorer.collapseAll"},
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
	app.Search.OnReplaceRequest = app.startSearchReplace
	app.Search.OnCancelReplace = app.cancelSearchReplace

	app.Changes.OnRightClick = app.ShowChangesFileContextMenu
	app.Changes.OnRightClickFolder = app.ShowChangesFolderContextMenu
	app.Changes.OnPanelMenu = app.ShowChangesContextMenu

	app.Changes.OnOpenFile = func(path string) {
		app.EditorGroup.OpenFile(path)
		app.EditorGroup.DisableInlineDiff(path)
		app.FocusEditorIfEnabled()
	}
	app.Changes.OnOpenDiff = app.OpenChangeDiff
	app.Changes.OnRefreshed = app.onChangesRefreshed
	app.Changes.OnOpenCommitDiff = app.OpenCommitDiff
	app.Changes.OnOpenCommit = app.OpenCommitDetail
	app.Changes.OnOpenPRDetail = app.OpenPRDetail
	app.Changes.OnOpenPRDiff = func(group *ui.ChangesGroup, status git.FileStatus, extended bool) {
		app.OpenPRDiff(group, status, extended)
	}
	app.Changes.OnPRGroupMenu = app.ShowPRGroupMenu
	app.Changes.OnRefreshPR = app.FetchAndOpenPR
	app.Changes.OnGroupMenu = app.ShowGroupMenu
	app.Changes.OnCommit = app.CommitChanges
	app.Changes.OnConfirmDiscard = app.ConfirmDiscard
	app.Changes.OnError = app.StatusError
	app.Changes.OnRefresh = app.RefreshChanges
	app.Changes.OnStatusChanged = func() {
		app.invalidateAllRepositories(RepositoryWorktree)
	}
	app.Changes.OnHistoryResult = func(err error) {
		if app.Repository != nil {
			app.Repository.HandleHistory(err)
		}
	}
	app.Changes.Split.OnResizeEnd = app.persistCommitHistoryHeight

	app.ContentSplit.OnResize = app.resizePanel
	app.ContentSplit.OnResizeEnd = app.finishPanelResize

	app.ContentSplit.OnTopClick = func() {
		if app.Root.HasModalOverlay() || app.Root.FocusClaimed() {
			return
		}
		app.Root.SetFocus(app.EditorGroup)
	}

	app.ContentSplit.OnBottomClick = func() {
		if app.Root.HasModalOverlay() || app.Root.FocusClaimed() {
			return
		}
		if w := app.BottomPanel.ActiveWidget(); w != nil {
			app.Root.SetFocus(w)
		}
	}

	app.SplitPanel.OnResize = app.resizeSidebar
	app.SplitPanel.OnResizeEnd = app.persistSidebarLayout

	app.BottomPanel.Tabs.Config.OnTabClick = func(index int) {
		panels := app.BottomPanel.PanelIDs()
		if index >= 0 && index < len(panels) {
			app.BottomPanel.SetActivePanel(panels[index])
			if w := app.BottomPanel.ActiveWidget(); w != nil {
				app.Root.SetFocus(w)
			}
		}
	}

	app.BottomPanel.Tabs.Config.OnOverflow = func(sx, sy int) {
		ids, titles := app.BottomPanel.HiddenTabs()
		var items []ui.ContextMenuItem
		for i, id := range ids {
			panelID := id
			commandID := "bottom.overflow." + panelID
			items = append(items, ui.ContextMenuItem{Label: titles[i], Command: commandID})
			reg.Register(command.Command{
				ID:    commandID,
				Title: titles[i],
				Handler: func() {
					app.BottomPanel.SetActivePanel(panelID)
					app.FocusPanel()
				},
			})
		}
		openContextMenu(app, items, sx, sy)
	}

	app.TerminalPanel.TabBar.OnRightClick = func(index, sx, sy int) {
		app.TerminalPanel.SetActive(index)
		app.Root.SetFocus(app.TerminalPanel)
		items := []ui.ContextMenuItem{
			{Label: i18n.Text("menu.closeTerminal"), Command: "terminal.close"},
			{Label: i18n.Text("menu.closeAllTerminals"), Command: "terminal.closeAll"},
		}
		openContextMenu(app, items, sx, sy)
	}

	app.BottomPanel.Tabs.Config.Actions = []widgets.TabAction{
		{Icon: string(icons.Rune(icons.Add)), OnClick: func(_, _ int) {
			reg.Execute("terminal.new")
		}},
		{Icon: string(icons.Rune(icons.More)), OnClick: func(sx, sy int) {
			dock := ui.ContextMenuItem{Label: i18n.Text("menu.dockRight"), Command: "panel.dockRight"}
			if app.ContentSplit.Position == ui.SplitRight {
				dock = ui.ContextMenuItem{Label: i18n.Text("menu.dockBottom"), Command: "panel.dockBottom"}
			}
			items := []ui.ContextMenuItem{
				{Label: i18n.Text("menu.newTerminal"), Command: "terminal.new"},
				ui.MenuSep(),
				{Label: i18n.Text("menu.closeTerminal"), Command: "terminal.close"},
				{Label: i18n.Text("menu.closeAllTerminals"), Command: "terminal.closeAll"},
				ui.MenuSep(),
				dock,
				{Label: i18n.Text("menu.closePanel"), Command: "panel.toggle"},
			}
			openContextMenu(app, items, sx, sy)
		}},
	}
}

func (a *App) tabMoreMenu() []ui.ContextMenuItem {
	return []ui.ContextMenuItem{
		{Label: i18n.Text("menu.reopenClosedTab"), Shortcut: a.KeyFor("tab.reopenClosed"), Command: "tab.reopenClosed", Disabled: !a.EditorGroup.HasClosedTabs()},
		ui.MenuSep(),
		{Label: i18n.Text("menu.closeAll"), Command: "tab.closeAll"},
		{Label: i18n.Text("menu.closeAllSaved"), Command: "tab.closeAllSaved"},
	}
}
