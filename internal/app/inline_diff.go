package app

import (
	"context"

	"github.com/eugenioenko/ttt/internal/i18n"
)

type inlineDiffRepo struct {
	dir, rel string
	source   baselineSource
}

func (a *App) repoPathForFile(path string) (repoDir, relPath string, ok bool) {
	if a.Repository == nil || path == "" {
		return "", "", false
	}
	repoDir, _ = a.Repository.RepositoryForPath(path)
	if repoDir == "" {
		return "", "", false
	}
	relPath, ok = a.Repository.gitRelativePath(repoDir, path)
	return repoDir, relPath, ok
}

func (a *App) openInlineDiff(path, repoDir, relPath string) (opened, conflict bool, err error) {
	a.EditorGroup.OpenFile(path)
	if a.EditorGroup.ActiveFilePath() != path || !a.EditorGroup.IsEditorActive() {
		return false, false, nil
	}
	source := a.Repository.baselineSource(repoDir)
	ctx, cancel := context.WithTimeout(context.Background(), diffOpenTimeout)
	defer cancel()
	baseline, err := source.read(ctx, relPath)
	if baseline.Conflicted {
		return false, true, nil
	}
	if err != nil {
		return false, false, err
	}
	var base []string
	if baseline.Exists {
		showTrailing := false
		if buf := a.EditorGroup.BufferForPath(path); buf != nil {
			showTrailing = buf.ShowTrailingNewline
		}
		base = blobLines(baseline.Content, showTrailing)
	}
	if !a.EditorGroup.EnableInlineDiff(path, base) {
		return false, false, nil
	}
	if a.inlineDiffRepos == nil {
		a.inlineDiffRepos = make(map[string]inlineDiffRepo)
	}
	a.inlineDiffRepos[path] = inlineDiffRepo{dir: repoDir, rel: relPath, source: source}
	return true, false, nil
}

func (a *App) ToggleInlineDiff() {
	path := a.EditorGroup.ActiveFilePath()
	if path == "" || !a.EditorGroup.IsEditorActive() || a.EditorGroup.IsActiveVirtual() {
		return
	}
	if a.EditorGroup.IsInlineDiffPath(path) {
		a.EditorGroup.DisableInlineDiff(path)
		return
	}
	repoDir, relPath, ok := a.repoPathForFile(path)
	if !ok {
		a.StatusNotify(i18n.Text("status.notInAGitRepository"))
		return
	}
	opened, conflict, err := a.openInlineDiff(path, repoDir, relPath)
	if err != nil {
		a.StatusError(err.Error())
		return
	}
	if conflict {
		a.StatusNotify(i18n.Format("status.hasMergeConflictsNoDiffToShow", i18n.Args{"relPath": relPath}))
		return
	}
	if !opened {
		a.StatusNotify(i18n.Text("status.notInAGitRepository"))
		return
	}
	a.RequestGitGutterForActiveFile()
}

func (a *App) onChangesRefreshed() {
	if a.Sidebar != nil {
		a.Sidebar.SetPanelDirty("changes", a.Changes.TotalChanges() > 0)
	}
	if a.pendingCurrentChangesOpen && a.selectedChangesDir() != "" {
		a.pendingCurrentChangesOpen = false
		a.OpenCurrentChanges()
	}
	// Repository changes can move the baseline of open diffs and gutter markers.
	if a.EditorGroup != nil && (a.EditorGroup.IsInlineDiffActive() || a.Settings.Editor.IsGitGutterEnabled()) {
		a.RequestGitGutterForActiveFile()
	}
}
