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

func (a *App) indexLines(path, repoDir, relPath string) []string {
	showTrailing := false
	if buf := a.EditorGroup.BufferForPath(path); buf != nil {
		showTrailing = buf.ShowTrailingNewline
	}
	content, err := git.ShowIndexFileContext(context.Background(), repoDir, relPath)
	if err != nil {
		return nil
	}
	return blobLines(content, showTrailing)
}

func (a *App) openInlineDiff(path, repoDir, relPath string) bool {
	a.EditorGroup.OpenFile(path)
	if a.EditorGroup.ActiveFilePath() != path || !a.EditorGroup.IsEditorActive() {
		return false
	}
	if !a.EditorGroup.EnableInlineDiff(path, a.indexLines(path, repoDir, relPath)) {
		return false
	}
	if a.inlineDiffRepos == nil {
		a.inlineDiffRepos = make(map[string]inlineDiffRepo)
	}
	a.inlineDiffRepos[path] = inlineDiffRepo{dir: repoDir, rel: relPath}
	return true
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
	if ok && git.IsUnmergedContext(context.Background(), repoDir, relPath) {
		a.StatusNotify(fmt.Sprintf("%s has merge conflicts; no diff to show", relPath))
		return
	}
	if !ok || !a.openInlineDiff(path, repoDir, relPath) {
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
