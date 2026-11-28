package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) OpenChangeDiff(dir string, change scm.Change, extended bool) {
	a.cancelPendingDiff()
	fullPath := filepath.Join(dir, change.Path)
	working := change.Group == "" || change.Group == scm.WorkingTree
	if working && change.Kind == scm.Untracked {
		if err := a.EditorGroup.OpenFile(fullPath); err != nil {
			return
		}
		a.FocusEditorIfEnabled()
		return
	}
	if working && change.Kind != scm.Deleted {
		if info, err := os.Lstat(fullPath); err == nil && info.Mode().IsRegular() {
			opened, conflict, err := a.openInlineDiff(fullPath, dir, change.Path)
			if err != nil && !errors.Is(err, errors.ErrUnsupported) {
				a.StatusError(err.Error())
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
	if err := a.EditorGroup.OpenFile(fullPath); err != nil {
		return
	}
	if a.Settings.Editor.MergeConflictView != config.MergeText && a.EditorGroup.ActiveBuffer() != nil {
		a.OpenMergeEditor()
		a.FocusEditorIfEnabled()
		return
	}
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
