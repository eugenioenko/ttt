package app

import (
	"context"
	"fmt"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
)

const gitGutterDelay = 500 * time.Millisecond

type gitGutterResult struct {
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

func (a *App) RequestGitGutter(filePath string, bufferLines []string) {
	if a.gitGutterDebounce != nil {
		a.gitGutterDebounce.Stop()
	}
	if a.gitGutterTask != nil {
		a.gitGutterTask.Stop()
	}
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

	// Snapshot on the UI goroutine before the worker can race with edits.
	linesCopy := make([]string, len(bufferLines))
	copy(linesCopy, bufferLines)
	showTrailing := false
	var version uint64
	if buf := a.EditorGroup.BufferForPath(filePath); buf != nil {
		showTrailing = buf.ShowTrailingNewline
		version = buf.Version()
	}

	if a.gitGutterTask == nil {
		a.gitGutterTask = NewTask[*gitGutterResult](a, asynctask.TaskOptions{})
	}
	a.gitGutterTask.Schedule(func(ctx context.Context) (*gitGutterResult, error) {
		headContent, gitErr := git.ShowFileContext(ctx, repoDir, relPath, "HEAD")
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result := &gitGutterResult{Path: filePath, DiffOn: diffOn, Version: version, Lines: linesCopy}
		if gitErr != nil && gutterOn {
			if !git.IgnoredFilesContext(ctx, repoDir, []string{relPath})[relPath] {
				// File is not tracked by git (new file) — mark all lines as added
				result.Changes = make([]diff.LineChangeKind, len(linesCopy))
				for i := range result.Changes {
					result.Changes[i] = diff.LineAdded
				}
			}
		} else if gutterOn {
			oldLines := blobLines(headContent, showTrailing)
			var err error
			result.Changes, err = diff.ComputeGutterChangesContext(ctx, oldLines, linesCopy)
			if err != nil {
				return nil, err
			}
		}
		if diffOn {
			indexContent, indexErr := git.ShowIndexFileContext(ctx, repoDir, relPath)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if indexErr == nil {
				result.Base = blobLines(indexContent, showTrailing)
			} else if git.IsUnmergedContext(ctx, repoDir, relPath) {
				result.Conflict, result.RelPath = true, relPath
			}
			if !result.Conflict {
				lines, err := diff.FullDiffLinesContext(ctx, result.Base, linesCopy)
				if err != nil {
					return nil, err
				}
				result.Diff = lines
			}
		}
		return result, ctx.Err()
	}, func(result *gitGutterResult, err error) {
		if err == nil {
			a.applyGitGutterResult(result)
		}
	})
}

func (a *App) applyGitGutterResult(v *gitGutterResult) {
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

func (a *App) RequestGitGutterForActiveFile() {
	a.ScheduleGitGutter()
	if a.gitGutterDebounce != nil {
		a.gitGutterDebounce.Flush()
	}
}

func (a *App) ScheduleGitGutter() {
	if a.gitGutterDebounce != nil {
		a.gitGutterDebounce.Stop()
	}
	if a.gitGutterTask != nil {
		a.gitGutterTask.Stop()
	}
	path := a.EditorGroup.ActiveFilePath()
	diffOn := a.EditorGroup.IsInlineDiffPath(path)
	if a.EditorGroup.ActiveBuffer() == nil || path == "" || a.EditorGroup.IsActiveVirtual() || (!a.Settings.Editor.IsGitGutterEnabled() && !diffOn) {
		return
	}
	if _, _, ok := a.repoPathForFile(path); !ok && (!diffOn || a.inlineDiffRepos[path].dir == "") {
		return
	}
	if a.gitGutterDebounce == nil {
		a.gitGutterDebounce = NewTask[struct{}](a, asynctask.TaskOptions{Delay: gitGutterDelay})
	}
	a.gitGutterDebounce.Schedule(func(context.Context) (struct{}, error) {
		return struct{}{}, nil
	}, func(_ struct{}, err error) {
		if err != nil {
			return
		}
		if buf := a.EditorGroup.ActiveBuffer(); buf != nil {
			a.RequestGitGutter(a.EditorGroup.ActiveFilePath(), buf.Lines)
		}
	})
}

// blobLines splits a blob the way buffer.LoadFile splits a file, so an
// unchanged file diffs as identical rather than off by a trailing line.
func blobLines(content string, showTrailingNewline bool) []string {
	lines := buffer.SplitLines(content)
	if len(lines) == 0 {
		lines = []string{""}
	}
	if showTrailingNewline && lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}
	return lines
}
