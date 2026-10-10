package app

import (
	"context"
	"fmt"
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
	DiffOn  bool
	Diff    []diff.DiffLine
	Base    []string
	// Version and Lines are the buffer version and text the diff was
	// computed from, so edits made while it ran can be replayed onto it.
	Version uint64
	Lines   []string
	// Conflict reports that the file has no single staged version to diff
	// against, because it has unresolved merge conflicts.
	Conflict bool
	RelPath  string
}

// RequestGitGutter triggers an async computation of git gutter indicators for
// the given file. The result is posted as a GitGutterResult via EventInterrupt.
func (a *App) RequestGitGutter(filePath string, bufferLines []string) {
	gutterOn := a.Settings.Editor.IsGitGutterEnabled()
	diffOn := a.EditorGroup.IsInlineDiffPath(filePath)
	if !gutterOn && !diffOn {
		return
	}
	if filePath == "" || a.EditorGroup.IsActiveVirtual() {
		return
	}

	repoDir, relPath, ok := a.repoPathForFile(filePath)
	if !ok && diffOn {
		repo := a.inlineDiffRepos[filePath]
		repoDir, relPath, ok = repo.dir, repo.rel, repo.dir != ""
	}
	if !ok {
		return
	}

	a.GitGutterGen++
	gen := a.GitGutterGen
	// Each edit starts a new diff; stale ones must stop, or pauses while typing
	// in a large file stack concurrent diffs (issue #672).
	if a.gitGutterCancel != nil {
		a.gitGutterCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.gitGutterCancel = cancel

	// Copy buffer lines to avoid races with the editor goroutine
	linesCopy := make([]string, len(bufferLines))
	copy(linesCopy, bufferLines)
	showTrailing := false
	var version uint64
	if buf := a.EditorGroup.BufferForPath(filePath); buf != nil {
		showTrailing = buf.ShowTrailingNewline
		version = buf.Version()
	}

	go func() {
		defer cancel()
		headContent, gitErr := git.ShowFileContext(ctx, repoDir, relPath, "HEAD")
		if ctx.Err() != nil {
			return
		}
		result := &GitGutterResult{Gen: gen, Path: filePath, DiffOn: diffOn, Version: version, Lines: linesCopy}
		if gitErr != nil && gutterOn {
			// File is not tracked by git (new file) — mark all lines as added
			result.Changes = make([]diff.LineChangeKind, len(linesCopy))
			for i := range result.Changes {
				result.Changes[i] = diff.LineAdded
			}
		} else if gutterOn {
			oldLines := strings.Split(headContent, "\n")
			var err error
			result.Changes, err = diff.ComputeGutterChangesContext(ctx, oldLines, linesCopy)
			if err != nil {
				return
			}
		}
		if diffOn {
			indexContent, indexErr := git.ShowIndexFileContext(ctx, repoDir, relPath)
			if ctx.Err() != nil {
				return
			}
			if indexErr == nil {
				result.Base = blobLines(indexContent, showTrailing)
			} else if git.IsUnmergedContext(ctx, repoDir, relPath) {
				result.Conflict, result.RelPath = true, relPath
			}
			if !result.Conflict {
				lines, err := diff.FullDiffLinesContext(ctx, result.Base, linesCopy)
				if err != nil {
					return
				}
				result.Diff = lines
			}
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(result))
	}()
}

func (a *App) ApplyGitGutterResult(v *GitGutterResult) {
	if v.Gen != a.GitGutterGen {
		return
	}
	a.EditorGroup.SetLineChanges(v.Path, v.Changes)
	switch {
	case v.DiffOn && v.Conflict:
		if a.EditorGroup.IsInlineDiffPath(v.Path) {
			a.EditorGroup.DisableInlineDiff(v.Path)
			a.StatusNotify(fmt.Sprintf("%s has merge conflicts; diff closed", v.RelPath))
		}
	case v.DiffOn:
		a.EditorGroup.SetInlineDiff(v.Path, v.Base, v.Diff, v.Version, v.Lines)
	}
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
	if !a.Settings.Editor.IsGitGutterEnabled() && !a.EditorGroup.IsInlineDiffActive() {
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

// blobLines splits a blob the way buffer.LoadFile splits a file, so an
// unchanged file diffs as identical rather than off by a trailing line.
func blobLines(content string, showTrailingNewline bool) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	if showTrailingNewline && lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}
	return lines
}
