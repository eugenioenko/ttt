package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
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

type currentFileState struct {
	status   git.FileStatus
	staged   bool
	unstaged bool
	added    bool
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
	states := mergeCurrentFileStates(statuses)
	result.Files = make([]ui.CommitDetailFile, 0, len(states))
	staged, unstaged, mixed := 0, 0, 0
	additions, deletions := 0, 0
	fingerprint := sha256.New()
	writeFingerprint := func(value []byte) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = fingerprint.Write(size[:])
		_, _ = fingerprint.Write(value)
	}
	writeFingerprint([]byte(dir))
	writeFingerprint([]byte(revision))

	for _, state := range states {
		if err := ctx.Err(); err != nil {
			result.Err = err
			result.Canceled = errors.Is(err, context.Canceled)
			return result
		}
		stage := ui.CommitDetailStageUnstaged
		switch {
		case state.staged && state.unstaged:
			stage = ui.CommitDetailStageMixed
			mixed++
		case state.staged:
			stage = ui.CommitDetailStageStaged
			staged++
		default:
			unstaged++
		}
		status := combinedCurrentStatus(state)
		oldPath := state.status.Path
		if state.status.OldPath != "" {
			oldPath = state.status.OldPath
		}
		var oldContent []byte
		oldExists := revision != "" && (!state.added || state.status.OldPath != "")
		if oldExists {
			var err error
			oldContent, err = git.ShowFileBytesContext(ctx, dir, oldPath, revision)
			if err != nil {
				result.Err = fmt.Errorf("read HEAD content for %q: %w", state.status.Path, err)
				result.Canceled = errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)
				return result
			}
		}
		newContent, newExists, err := readWorkingTreeContent(filepath.Join(dir, state.status.Path))
		if err != nil {
			result.Err = fmt.Errorf("read working tree content for %q: %w", state.status.Path, err)
			return result
		}
		if !newExists && status != "D" {
			result.Err = fmt.Errorf("working tree path %q disappeared during refresh", state.status.Path)
			return result
		}

		writeFingerprint([]byte(status))
		writeFingerprint([]byte{byte(stage)})
		writeFingerprint([]byte(state.status.OldPath))
		writeFingerprint([]byte(state.status.Path))
		writeFingerprint(oldContent)
		writeFingerprint(newContent)

		file := ui.CommitDetailFile{
			Status: status, Path: state.status.Path, OldPath: state.status.OldPath, Stage: stage,
		}
		switch {
		case isBinaryContent(oldContent) || isBinaryContent(newContent):
			file.ContentKind = ui.CommitDetailContentBinary
			file.FullFileState = ui.CommitDetailFullFileLoaded
		case len(oldContent) == 0 && len(newContent) == 0 && (oldExists || newExists):
			file.ContentKind = ui.CommitDetailContentEmpty
			file.FullFileState = ui.CommitDetailFullFileLoaded
		default:
			oldLines := splitCurrentChangesLines(oldContent)
			newLines := splitCurrentChangesLines(newContent)
			patch, err := git.DiffWorkingTreeFileContext(ctx, dir, revision, state.status)
			if err != nil {
				result.Err = fmt.Errorf("read diff for %q: %w", state.status.Path, err)
				result.Canceled = errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)
				return result
			}
			file.Diff = diff.Parse(patch)
			file = ui.CommitDetailFileWithContent(file, oldLines, newLines)
			added, deleted := currentDiffStats(file.Diff)
			additions += added
			deletions += deleted
		}
		result.Files = append(result.Files, file)
	}
	result.Fingerprint = fmt.Sprintf("%x", fingerprint.Sum(nil))
	result.Summary = currentChangesSummary(len(states), additions, deletions, staged, unstaged, mixed)
	return result
}

func readWorkingTreeContent(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		return []byte(target), true, err
	}
	if info.IsDir() {
		return []byte{0}, true, nil
	}
	content, err := os.ReadFile(path)
	return content, true, err
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

func mergeCurrentFileStates(statuses []git.FileStatus) []currentFileState {
	states := make([]currentFileState, 0, len(statuses))
	index := make(map[string]int, len(statuses))
	for _, status := range statuses {
		i, ok := index[status.Path]
		if !ok {
			i = len(states)
			index[status.Path] = i
			states = append(states, currentFileState{status: status})
		}
		state := &states[i]
		if status.OldPath != "" {
			state.status.OldPath = status.OldPath
		}
		if status.Staged {
			state.staged = true
			if status.Status == "A" {
				state.added = true
			}
			if state.status.Status == "M" || state.status.Status == "" {
				state.status.Status = status.Status
			}
		} else {
			state.unstaged = true
			if status.Status == "?" {
				state.added = true
			}
			if status.Status == "?" || status.Status == "D" {
				state.status.Status = status.Status
			}
		}
	}
	return states
}

func combinedCurrentStatus(state currentFileState) string {
	if state.status.Status == "" {
		return "M"
	}
	return state.status.Status
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

func currentChangesSummary(files, additions, deletions, staged, unstaged, mixed int) string {
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
	if mixed > 0 {
		parts = append(parts, fmt.Sprintf("%d mixed", mixed))
	}
	return strings.Join(parts, " · ")
}
