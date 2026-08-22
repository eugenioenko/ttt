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
	"github.com/eugenioenko/ttt/internal/git"
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

type currentFileEndpoint struct {
	content []byte
	exists  bool
	gitlink bool
}

type currentChangeStatus struct {
	status       git.FileStatus
	conflictCode string
}

func currentChangesTabID(dir string) string {
	return "current-changes:\x00" + filepath.Clean(dir)
}

func currentChangesTitle(dir string, multiRoot bool) string {
	if multiRoot {
		return "Changes · " + filepath.Base(dir)
	}
	return "Current Changes"
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
		a.StatusNotify("Loading repository changes…")
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
	return "Could not refresh current changes: " + err.Error()
}

func readCurrentChanges(ctx context.Context, dir, revision, tabID string, epoch, request uint64, statuses []git.FileStatus) *CurrentChangesResult {
	result := &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request}
	result.Files = make([]ui.CommitDetailFile, 0, len(statuses))
	staged, unstaged, conflicts := 0, 0, 0
	additions, deletions := 0, 0
	paths := make(map[string]struct{}, len(statuses))
	fingerprint := sha256.New()
	writeFingerprint := func(value []byte) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = fingerprint.Write(size[:])
		_, _ = fingerprint.Write(value)
	}
	writeFingerprint([]byte(dir))
	writeFingerprint([]byte(revision))

	for _, change := range normalizeCurrentChangeStatuses(statuses) {
		status := change.status
		if err := ctx.Err(); err != nil {
			result.Err = err
			result.Canceled = errors.Is(err, context.Canceled)
			return result
		}
		paths[status.Path] = struct{}{}
		stage := ui.CommitDetailStageUnstaged
		boundary := ui.CommitDetailBoundaryIndexToWorktree
		if change.conflictCode != "" {
			stage = ui.CommitDetailStageConflict
			boundary = ui.CommitDetailBoundaryConflictToWorktree
			conflicts++
		} else if status.Staged {
			stage = ui.CommitDetailStageStaged
			boundary = ui.CommitDetailBoundaryHeadToIndex
			staged++
		} else {
			unstaged++
		}

		var oldEndpoint, newEndpoint currentFileEndpoint
		var indexStages []byte
		var err error
		if change.conflictCode != "" {
			oldEndpoint, newEndpoint, indexStages, err = readCurrentConflictEndpoints(ctx, dir, status.Path)
		} else {
			oldEndpoint, newEndpoint, indexStages, err = readCurrentFileEndpoints(ctx, dir, revision, status)
		}
		if err != nil {
			result.Err = fmt.Errorf("read %s boundary for %q: %w", currentBoundaryName(boundary), status.Path, err)
			result.Canceled = errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)
			return result
		}
		oldLines := splitCurrentChangesLines(oldEndpoint.content)
		newLines := splitCurrentChangesLines(newEndpoint.content)
		var fileDiff diff.FileDiff
		var patch string
		if change.conflictCode == "" && (oldEndpoint.gitlink || newEndpoint.gitlink) {
			patch, err = git.DiffCurrentFileContext(ctx, dir, revision, status)
			if err != nil {
				result.Err = fmt.Errorf("read %s gitlink diff for %q: %w", currentBoundaryName(boundary), status.Path, err)
				result.Canceled = errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)
				return result
			}
			fileDiff = diff.Parse(patch)
		} else if !bytes.Equal(oldEndpoint.content, newEndpoint.content) || oldEndpoint.exists != newEndpoint.exists {
			generated, generateErr := diff.GenerateContext(ctx, oldLines, newLines, status.Path)
			err = generateErr
			if err != nil {
				result.Err = fmt.Errorf("generate %s diff for %q: %w", currentBoundaryName(boundary), status.Path, err)
				result.Canceled = errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)
				return result
			}
			fileDiff = diff.Parse(generated)
		}
		presentedStatus := currentPresentedStatus(status.Status)
		presentedOldPath := currentPresentedOldPath(status)

		writeFingerprint([]byte(presentedStatus))
		writeFingerprint([]byte{byte(stage)})
		writeFingerprint([]byte{byte(boundary)})
		writeFingerprint([]byte(presentedOldPath))
		writeFingerprint([]byte(status.Path))
		writeFingerprint(indexStages)
		writeFingerprint([]byte(change.conflictCode))
		writeFingerprint(oldEndpoint.content)
		writeFingerprint(newEndpoint.content)
		writeFingerprint([]byte(patch))

		file := ui.CommitDetailFile{
			Status: presentedStatus, Path: status.Path, OldPath: presentedOldPath, Stage: stage, Boundary: boundary,
			IndexStages: append([]byte(nil), indexStages...), ConflictCode: change.conflictCode,
		}
		switch {
		case isBinaryContent(oldEndpoint.content) || isBinaryContent(newEndpoint.content):
			file.ContentKind = ui.CommitDetailContentBinary
			file.FullFileState = ui.CommitDetailFullFileLoaded
		case len(oldEndpoint.content) == 0 && len(newEndpoint.content) == 0 && (oldEndpoint.exists || newEndpoint.exists) && !oldEndpoint.gitlink && !newEndpoint.gitlink:
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

func normalizeCurrentChangeStatuses(statuses []git.FileStatus) []currentChangeStatus {
	type pathStatuses struct {
		count            int
		staged, unstaged int
	}
	pairs := make(map[string]pathStatuses, len(statuses))
	for i, status := range statuses {
		pair := pairs[status.Path]
		if pair.count == 0 {
			pair.staged = -1
			pair.unstaged = -1
		}
		pair.count++
		if status.Staged {
			pair.staged = i
		} else {
			pair.unstaged = i
		}
		pairs[status.Path] = pair
	}

	conflicts := make(map[int]string)
	skip := make(map[int]struct{})
	for _, pair := range pairs {
		if pair.count != 2 || pair.staged < 0 || pair.unstaged < 0 {
			continue
		}
		code, ok := currentConflictCode(statuses[pair.staged].Status, statuses[pair.unstaged].Status)
		if !ok {
			continue
		}
		first, second := pair.staged, pair.unstaged
		if second < first {
			first, second = second, first
		}
		conflicts[first] = code
		skip[second] = struct{}{}
	}

	normalized := make([]currentChangeStatus, 0, len(statuses)-len(skip))
	for i, status := range statuses {
		if _, omitted := skip[i]; omitted {
			continue
		}
		code := conflicts[i]
		if code != "" {
			status.Status = "U"
			status.OldPath = ""
			status.Staged = false
		}
		normalized = append(normalized, currentChangeStatus{status: status, conflictCode: code})
	}
	return normalized
}

func currentConflictCode(staged, unstaged string) (string, bool) {
	code := staged + unstaged
	switch code {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return code, true
	default:
		return "", false
	}
}

func readCurrentConflictEndpoints(ctx context.Context, dir, path string) (currentFileEndpoint, currentFileEndpoint, []byte, error) {
	entries, err := git.IndexEntriesContext(ctx, dir, path)
	if err != nil {
		return currentFileEndpoint{}, currentFileEndpoint{}, nil, err
	}
	indexStages := make([]byte, 0, len(entries))
	for _, entry := range entries {
		indexStages = append(indexStages, byte(entry.Stage))
	}
	entry, ok := preferredConflictEntry(entries)
	if !ok {
		return currentFileEndpoint{}, currentFileEndpoint{}, indexStages, fmt.Errorf("unmerged path has no stage 1, 2, or 3 endpoint")
	}
	oldEndpoint, err := readIndexEntryEndpoint(ctx, dir, path, entry)
	if err != nil {
		return currentFileEndpoint{}, currentFileEndpoint{}, indexStages, err
	}
	working, err := readWorkingTreeFileContext(ctx, dir, path)
	if err != nil {
		var pathErr *workingTreePathError
		if errors.As(err, &pathErr) && pathErr.Kind == workingTreePathDirectory && oldEndpoint.gitlink {
			return oldEndpoint, currentFileEndpoint{exists: true, gitlink: true}, indexStages, nil
		}
		return currentFileEndpoint{}, currentFileEndpoint{}, indexStages, err
	}
	if !working.Exists {
		return oldEndpoint, currentFileEndpoint{}, indexStages, nil
	}
	return oldEndpoint, currentFileEndpoint{content: working.Content, exists: true}, indexStages, nil
}

func preferredConflictEntry(entries []git.IndexEntry) (git.IndexEntry, bool) {
	for _, stage := range []int{2, 3, 1} {
		for _, entry := range entries {
			if entry.Stage == stage {
				return entry, true
			}
		}
	}
	return git.IndexEntry{}, false
}

func readCurrentFileEndpoints(ctx context.Context, dir, revision string, status git.FileStatus) (currentFileEndpoint, currentFileEndpoint, []byte, error) {
	indexPath := status.Path
	if !status.Staged && (status.Status == "R" || status.Status == "C") && status.OldPath != "" {
		indexPath = status.OldPath
	}
	entries, err := git.IndexEntriesContext(ctx, dir, indexPath)
	if err != nil {
		return currentFileEndpoint{}, currentFileEndpoint{}, nil, err
	}
	indexStages := make([]byte, 0, len(entries))
	for _, entry := range entries {
		indexStages = append(indexStages, byte(entry.Stage))
	}

	if status.Staged {
		oldEndpoint, err := readTreeEndpoint(ctx, dir, revision, currentOldPath(status))
		if err != nil {
			return currentFileEndpoint{}, currentFileEndpoint{}, nil, err
		}
		newEndpoint, err := readIndexEndpoint(ctx, dir, indexPath, entries)
		return oldEndpoint, newEndpoint, indexStages, err
	}

	oldEndpoint := currentFileEndpoint{}
	if status.Status != "?" {
		oldEndpoint, err = readIndexEndpoint(ctx, dir, indexPath, entries)
		if err != nil {
			return currentFileEndpoint{}, currentFileEndpoint{}, nil, err
		}
	}
	if status.Status == "D" {
		return oldEndpoint, currentFileEndpoint{}, indexStages, nil
	}
	working, err := readWorkingTreeFileContext(ctx, dir, status.Path)
	if err != nil {
		var pathErr *workingTreePathError
		if errors.As(err, &pathErr) && pathErr.Kind == workingTreePathDirectory && (oldEndpoint.gitlink || hasGitlinkEntry(entries)) {
			return oldEndpoint, currentFileEndpoint{exists: true, gitlink: true}, indexStages, nil
		}
		return currentFileEndpoint{}, currentFileEndpoint{}, nil, err
	}
	if !working.Exists {
		return currentFileEndpoint{}, currentFileEndpoint{}, nil, fmt.Errorf("working tree path disappeared")
	}
	return oldEndpoint, currentFileEndpoint{content: working.Content, exists: true}, indexStages, nil
}

func readTreeEndpoint(ctx context.Context, dir, revision, path string) (currentFileEndpoint, error) {
	if revision == "" || path == "" {
		return currentFileEndpoint{}, nil
	}
	entry, exists, err := git.TreeEntryContext(ctx, dir, revision, path)
	if err != nil || !exists {
		return currentFileEndpoint{}, err
	}
	if entry.Mode == "160000" {
		return currentFileEndpoint{content: []byte("Subproject commit " + entry.Object + "\n"), exists: true, gitlink: true}, nil
	}
	content, err := git.ShowFileBytesContext(ctx, dir, path, revision)
	return currentFileEndpoint{content: content, exists: err == nil}, err
}

func readIndexEndpoint(ctx context.Context, dir, path string, entries []git.IndexEntry) (currentFileEndpoint, error) {
	if len(entries) == 0 {
		return currentFileEndpoint{}, nil
	}
	entry := entries[0]
	for _, candidate := range entries {
		if candidate.Stage == 0 || candidate.Stage == 2 {
			entry = candidate
			if candidate.Stage == 0 {
				break
			}
		}
	}
	return readIndexEntryEndpoint(ctx, dir, path, entry)
}

func readIndexEntryEndpoint(ctx context.Context, dir, path string, entry git.IndexEntry) (currentFileEndpoint, error) {
	if entry.Mode == "160000" {
		return currentFileEndpoint{content: []byte("Subproject commit " + entry.Object + "\n"), exists: true, gitlink: true}, nil
	}
	content, err := git.ShowIndexFileBytesContext(ctx, dir, path, entry.Stage)
	return currentFileEndpoint{content: content, exists: err == nil}, err
}

func hasGitlinkEntry(entries []git.IndexEntry) bool {
	for _, entry := range entries {
		if entry.Mode == "160000" {
			return true
		}
	}
	return false
}

func currentOldPath(status git.FileStatus) string {
	if status.OldPath != "" {
		return status.OldPath
	}
	return status.Path
}

func currentPresentedOldPath(status git.FileStatus) string {
	if status.Status == "R" || status.Status == "C" {
		return status.OldPath
	}
	return ""
}

func currentPresentedStatus(status string) string {
	if status == "?" {
		return "A"
	}
	if status == "" {
		return "M"
	}
	return status
}

func currentBoundaryName(boundary ui.CommitDetailBoundary) string {
	switch boundary {
	case ui.CommitDetailBoundaryHeadToIndex:
		return "HEAD-to-index"
	case ui.CommitDetailBoundaryConflictToWorktree:
		return "conflict-to-worktree"
	default:
		return "index-to-worktree"
	}
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
		return "Working tree clean"
	}
	fileLabel := "file"
	if files != 1 {
		fileLabel = "files"
	}
	parts := []string{fmt.Sprintf("%d %s", files, fileLabel), fmt.Sprintf("+%d −%d", additions, deletions)}
	if staged > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", staged))
	}
	if unstaged > 0 {
		parts = append(parts, fmt.Sprintf("%d unstaged", unstaged))
	}
	if conflicts > 0 {
		label := "conflict"
		if conflicts != 1 {
			label = "conflicts"
		}
		parts = append(parts, fmt.Sprintf("%d %s", conflicts, label))
	}
	return strings.Join(parts, " · ")
}
