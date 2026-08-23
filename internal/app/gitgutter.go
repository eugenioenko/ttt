package app

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"

	"github.com/gdamore/tcell/v3"
)

// GitGutterResult carries the async result of a git gutter diff computation.
type GitGutterResult struct {
	Gen     int
	Path    string
	Changes []diff.LineChangeKind
}

// RequestGitGutter triggers an async computation of git gutter indicators for
// the given file. The result is posted as a GitGutterResult via EventInterrupt.
func (a *App) RequestGitGutter(filePath string, bufferLines []string) {
	if !a.Settings.Editor.IsGitGutterEnabled() {
		return
	}
	if filePath == "" || a.EditorGroup.IsActiveVirtual() {
		return
	}

	if a.Repository == nil {
		return
	}
	repoDir, _ := a.Repository.RepositoryForPath(filePath)
	if repoDir == "" {
		return
	}

	gitPath, err := a.Repository.resolvePathIdentity(filePath)
	if err != nil || !pathWithin(repoDir, gitPath) {
		return
	}
	relPath, err := filepath.Rel(repoDir, gitPath)
	if err != nil {
		return
	}
	// Normalize to forward slashes for git
	relPath = filepath.ToSlash(relPath)

	a.GitGutterGen++
	gen := a.GitGutterGen

	// Copy buffer lines to avoid races with the editor goroutine
	linesCopy := make([]string, len(bufferLines))
	copy(linesCopy, bufferLines)

	go func() {
		headContent, gitErr := git.ShowFile(repoDir, relPath, "HEAD")
		var changes []diff.LineChangeKind
		if gitErr != nil {
			// File is not tracked by git (new file) — mark all lines as added
			changes = make([]diff.LineChangeKind, len(linesCopy))
			for i := range changes {
				changes[i] = diff.LineAdded
			}
		} else {
			oldLines := strings.Split(headContent, "\n")
			changes = diff.ComputeGutterChanges(oldLines, linesCopy)
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(&GitGutterResult{
			Gen:     gen,
			Path:    filePath,
			Changes: changes,
		}))
	}()
}

// RequestGitGutterForActiveFile triggers a git gutter update for the currently
// active editor tab.
func (a *App) RequestGitGutterForActiveFile() {
	path := a.EditorGroup.ActiveFilePath()
	buf := a.EditorGroup.ActiveBuffer()
	if buf == nil {
		return
	}
	a.RequestGitGutter(path, buf.Lines)
}

// ScheduleGitGutter debounces git gutter updates during typing. It waits 500ms
// after the last buffer change before computing the diff.
func (a *App) ScheduleGitGutter() {
	if !a.Settings.Editor.IsGitGutterEnabled() {
		return
	}
	if a.GitGutterTimer != nil {
		a.GitGutterTimer.Stop()
	}
	a.GitGutterTimer = time.AfterFunc(500*time.Millisecond, func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(&GitGutterTrigger{}))
	})
}

// GitGutterTrigger is posted as an EventInterrupt to request a git gutter
// recomputation on the main thread after a debounce delay.
type GitGutterTrigger struct{}
