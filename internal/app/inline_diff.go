package app

import (
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

func (a *App) headLines(path, repoDir, relPath string) []string {
	showTrailing := false
	if buf := a.EditorGroup.BufferForPath(path); buf != nil {
		showTrailing = buf.ShowTrailingNewline
	}
	content, err := git.ShowFile(repoDir, relPath, "HEAD")
	if err != nil {
		return nil
	}
	return headBaseLines(content, showTrailing)
}

func (a *App) openInlineDiff(path, repoDir, relPath string) bool {
	a.EditorGroup.OpenFile(path)
	if a.EditorGroup.ActiveFilePath() != path || !a.EditorGroup.IsEditorActive() {
		return false
	}
	if !a.EditorGroup.EnableInlineDiff(path, a.headLines(path, repoDir, relPath)) {
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
	if !ok || !a.openInlineDiff(path, repoDir, relPath) {
		a.StatusNotify("Not in a git repository")
		return
	}
	a.RequestGitGutterForActiveFile()
}
