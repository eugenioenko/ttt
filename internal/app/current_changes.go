package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/ui"
)

type CurrentChangesResult struct {
	Dir         string
	TabID       string
	Epoch       uint64
	Request     uint64
	Fingerprint string
	Summary     string
	Files       []ui.CommitDetailFile
	Err         error
	Canceled    bool
}

const currentChangesMaxDiffCells = 4_000_000

func currentChangesTabID(dir string) string {
	return "current-changes:\x00" + filepath.Clean(dir)
}

func currentChangesTitle(dir string, multiRoot bool) string {
	if multiRoot {
		return i18n.Format("changes.repositoryTitle", i18n.Args{"filename": filepath.Base(dir)})
	}
	return i18n.Text("changes.currentChanges")
}

func (a *App) selectedChangesDir() string {
	if a == nil || a.Changes == nil {
		return ""
	}
	if dir := a.Changes.selectedGroupDir(); dir != "" {
		return dir
	}
	if len(a.Changes.groups) > 0 {
		return a.Changes.groups[0].Dir
	}
	return ""
}

func (a *App) OpenCurrentChanges() {
	dir := a.selectedChangesDir()
	if dir == "" {
		a.pendingCurrentChangesOpen = true
		if a.Repository != nil {
			a.Repository.RefreshNow(RepositoryWorktree)
		}
		a.StatusNotify(i18n.Text("status.loadingRepositoryChanges"))
		return
	}
	a.pendingCurrentChangesOpen = false
	tabID := currentChangesTabID(dir)
	detail := a.EditorGroup.CurrentChangesWidgetByTab(tabID)
	if detail == nil {
		detail = ui.NewCurrentChangesWidget(dir, a.EditorGroup.SyntaxHighlight)
		detail.OnClose = func() {
			if a.Repository != nil {
				a.Repository.CloseCurrentChangesTab(tabID)
			}
		}
		a.EditorGroup.ApplyDiffDefaults(detail)
		a.EditorGroup.OpenPluginTab(tabID, currentChangesTitle(dir, len(a.Changes.groups) > 1), detail)
	} else {
		a.EditorGroup.SwitchToTabByPath(tabID)
	}
	a.FocusEditorIfEnabled()
	a.syncRepositoryObservation()
}

func (a *App) ApplyCurrentChanges(result *CurrentChangesResult) {
	if result == nil || result.Canceled || a.EditorGroup == nil {
		return
	}
	detail := a.EditorGroup.CurrentChangesWidgetByTab(result.TabID)
	if detail == nil || detail.Dir != result.Dir || detail.Incarnation != result.Epoch {
		return
	}
	detail.SetCurrentChanges(result.Summary, result.Files, currentChangesErrorText(result.Err))
}

func currentChangesErrorText(err error) string {
	if err == nil {
		return ""
	}
	return i18n.Format("changes.couldNotRefreshCurrentChanges", i18n.Args{"error": err.Error()})
}

func readCurrentChanges(ctx context.Context, source repositorySource, revision, tabID string, epoch, request uint64, changes []scm.Change) *CurrentChangesResult {
	dir := source.root
	result := &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request}
	result.Files = make([]ui.CommitDetailFile, 0, len(changes))
	staged, unstaged, conflicts := 0, 0, 0
	additions, deletions := 0, 0
	paths := make(map[string]struct{}, len(changes))
	fingerprint := sha256.New()
	writeFingerprint := func(value []byte) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = fingerprint.Write(size[:])
		_, _ = fingerprint.Write(value)
	}
	writeFingerprint([]byte(dir))
	writeFingerprint([]byte(revision))

	repo, err := source.resolve(ctx)
	if err != nil {
		result.Err = err
		result.Canceled = errors.Is(err, context.Canceled)
		return result
	}
	reader, ok := repo.(scm.ChangeReader)
	if !ok {
		result.Err = errors.ErrUnsupported
		return result
	}
	for _, change := range changes {
		if err := ctx.Err(); err != nil {
			result.Err = err
			result.Canceled = errors.Is(err, context.Canceled)
			return result
		}
		content, err := reader.ReadChange(ctx, change)
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if err != nil {
			result.Err = fmt.Errorf("read change for %q: %w", change.Path, err)
			result.Canceled = errors.Is(err, context.Canceled)
			return result
		}
		paths[change.Path] = struct{}{}
		stage := ui.CommitDetailStageUnstaged
		boundary := ui.CommitDetailBoundaryNone
		if content.Conflicted || change.Kind == scm.Conflicted {
			stage = ui.CommitDetailStageConflict
			boundary = ui.CommitDetailBoundaryConflictToWorktree
			conflicts++
		} else if change.Group == scm.Staged {
			stage = ui.CommitDetailStageStaged
			boundary = ui.CommitDetailBoundaryHeadToIndex
			staged++
		} else {
			if change.Group == "" || change.Group == scm.WorkingTree {
				boundary = ui.CommitDetailBoundaryIndexToWorktree
			}
			unstaged++
		}
		oldContent, newContent := []byte(content.Before.Content), []byte(content.After.Content)
		oldLines, newLines := splitCurrentChangesLines(oldContent), splitCurrentChangesLines(newContent)
		patch := content.Patch
		fileDiff := diff.Parse(patch)
		if !isBinaryContent(oldContent) && !isBinaryContent(newContent) && (content.Before != content.After) {
			generated := ""
			if !currentChangesDiffExceedsLimit(len(oldLines), len(newLines)) {
				generated, err = diff.GenerateContext(ctx, oldLines, newLines, change.Path)
			} else if patch == "" {
				generated, err = generateCurrentChangesReplacement(ctx, oldLines, newLines, change.Path)
			} else {
				generated = patch
			}
			if err != nil {
				result.Err = fmt.Errorf("generate diff for %q: %w", change.Path, err)
				result.Canceled = errors.Is(err, context.Canceled)
				return result
			}
			fileDiff = diff.Parse(generated)
		}
		presentedStatus := ui.ChangeBadge(change)
		if change.Kind == scm.Untracked {
			presentedStatus = "A"
		}
		if change.Kind == scm.Conflicted || content.Conflicted {
			presentedStatus = "U"
		}
		if presentedStatus == "" {
			presentedStatus = "M"
		}
		presentedOldPath := ""
		if change.Kind == scm.Renamed || change.Kind == scm.Copied {
			presentedOldPath = change.OldPath
		}

		writeFingerprint([]byte(presentedStatus))
		writeFingerprint([]byte{byte(stage)})
		writeFingerprint([]byte{byte(boundary)})
		writeFingerprint([]byte(presentedOldPath))
		writeFingerprint([]byte(change.Path))
		writeFingerprint([]byte(change.Group))
		writeFingerprint([]byte(change.Kind))
		writeFingerprint([]byte(change.Detail))
		writeFingerprint([]byte(fmt.Sprint(len(content.ConflictSources))))
		for _, side := range content.ConflictSources {
			writeFingerprint([]byte(side))
		}
		writeFingerprint([]byte(content.ConflictDetail))
		writeFingerprint([]byte(fmt.Sprintf("%t/%t", content.Before.Exists, content.After.Exists)))
		writeFingerprint(oldContent)
		writeFingerprint(newContent)
		writeFingerprint([]byte(patch))

		file := ui.CommitDetailFile{
			Status: presentedStatus, Path: change.Path, OldPath: presentedOldPath, Stage: stage, Boundary: boundary,
			Group: string(change.Group), ConflictSources: append([]string(nil), content.ConflictSources...), ConflictCode: content.ConflictDetail,
		}
		switch {
		case isBinaryContent(oldContent) || isBinaryContent(newContent):
			file.ContentKind = ui.CommitDetailContentBinary
			file.FullFileState = ui.CommitDetailFullFileLoaded
		case len(oldContent) == 0 && len(newContent) == 0 && (content.Before.Exists || content.After.Exists):
			file.ContentKind = ui.CommitDetailContentEmpty
			file.FullFileState = ui.CommitDetailFullFileLoaded
		default:
			file.Diff = fileDiff
			file = ui.CommitDetailFileWithContent(file, oldLines, newLines)
			added, deleted := currentDiffStats(file.Diff)
			additions += added
			deletions += deleted
		}
		result.Files = append(result.Files, file)
	}
	result.Fingerprint = fmt.Sprintf("%x", fingerprint.Sum(nil))
	result.Summary = currentChangesSummary(len(paths), additions, deletions, staged, unstaged, conflicts)
	return result
}

func currentChangesDiffExceedsLimit(oldLines, newLines int) bool {
	rows, columns := oldLines+1, newLines+1
	return rows > currentChangesMaxDiffCells/columns
}

func generateCurrentChangesReplacement(ctx context.Context, oldLines, newLines []string, path string) (string, error) {
	var generated strings.Builder
	fmt.Fprintf(&generated, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n", path, path, path, path, len(oldLines), len(newLines))
	for _, line := range oldLines {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		generated.WriteByte('-')
		generated.WriteString(line)
		generated.WriteByte('\n')
	}
	for _, line := range newLines {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		generated.WriteByte('+')
		generated.WriteString(line)
		generated.WriteByte('\n')
	}
	return generated.String(), nil
}

func isBinaryContent(content []byte) bool {
	return bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content)
}

func splitCurrentChangesLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func currentDiffStats(file diff.FileDiff) (added, deleted int) {
	for _, line := range file.AllLines() {
		if line.Left.Kind == diff.Deleted {
			deleted++
		}
		if line.Right.Kind == diff.Added {
			added++
		}
	}
	return added, deleted
}

func currentChangesSummary(files, additions, deletions, staged, unstaged, conflicts int) string {
	if files == 0 {
		return i18n.Text("changes.workingTreeClean")
	}
	fileLabel := i18n.Format("changes.fileCount", i18n.Args{"count": files})
	if files != 1 {
		fileLabel = i18n.Format("changes.filesCount", i18n.Args{"count": files})
	}
	parts := []string{fileLabel, fmt.Sprintf("+%d −%d", additions, deletions)}
	if staged > 0 {
		parts = append(parts, i18n.Format("changes.stagedCount", i18n.Args{"staged": staged}))
	}
	if unstaged > 0 {
		parts = append(parts, i18n.Format("changes.unstagedCount", i18n.Args{"unstaged": unstaged}))
	}
	if conflicts > 0 {
		label := i18n.Format("changes.conflictCount", i18n.Args{"count": conflicts})
		if conflicts != 1 {
			label = i18n.Format("changes.conflictsCount", i18n.Args{"count": conflicts})
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " · ")
}
