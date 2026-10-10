package app

import (
	"context"
	"fmt"

	"github.com/eugenioenko/ttt/internal/git"
)

type inlineDiffRepo struct {
	dir, rel string
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

func (a *App) indexLines(path, repoDir, relPath string) ([]string, error) {
	showTrailing := false
	if buf := a.EditorGroup.BufferForPath(path); buf != nil {
		showTrailing = buf.ShowTrailingNewline
	}
	content, err := git.ShowIndexFileContext(context.Background(), repoDir, relPath)
	if err != nil {
		return nil, err
	}
	return blobLines(content, showTrailing), nil
}

// openInlineDiff opens path in diff mode against its staged version. A file
// with merge conflicts has no single staged version: it stays open without a
// diff and conflict reports why. Only a failed index read pays for that check.
func (a *App) openInlineDiff(path, repoDir, relPath string) (opened, conflict bool) {
	a.EditorGroup.OpenFile(path)
	if a.EditorGroup.ActiveFilePath() != path || !a.EditorGroup.IsEditorActive() {
		return false, false
	}
	base, err := a.indexLines(path, repoDir, relPath)
	if err != nil && git.IsUnmergedContext(context.Background(), repoDir, relPath) {
		return false, true
	}
	if !a.EditorGroup.EnableInlineDiff(path, base) {
		return false, false
	}
	if a.inlineDiffRepos == nil {
		a.inlineDiffRepos = make(map[string]inlineDiffRepo)
	}
	a.inlineDiffRepos[path] = inlineDiffRepo{dir: repoDir, rel: relPath}
	return true, false
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
		a.StatusNotify("Not in a git repository")
		return
	}
	opened, conflict := a.openInlineDiff(path, repoDir, relPath)
	if conflict {
		a.StatusNotify(fmt.Sprintf("%s has merge conflicts; no diff to show", relPath))
		return
	}
	if !opened {
		a.StatusNotify("Not in a git repository")
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
	// Staging, unstaging and commits move the index an open working-tree diff
	// is based on.
	if a.EditorGroup != nil && a.EditorGroup.IsInlineDiffActive() {
		a.RequestGitGutterForActiveFile()
	}
}
