package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

const currentChangesTimeout = 30 * time.Second

type CurrentChangesResult struct {
	Dir         string
	TabID       string
	Gen         uint64
	Fingerprint string
	Summary     string
	Files       []ui.CommitDetailFile
	Err         string
}

type currentChangesLoadState struct {
	cancel               context.CancelFunc
	appliedFingerprint   string
	requestedFingerprint string
}

type currentFileState struct {
	status   git.FileStatus
	staged   bool
	unstaged bool
}

func currentChangesTabID(dir string) string {
	return "current-changes:" + filepath.Clean(dir)
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
	dir := a.Changes.selectedGroupDir()
	if dir == "" && len(a.Changes.groups) > 0 {
		dir = a.Changes.groups[0].Dir
	}
	if dir == "" && len(a.Changes.Dirs) > 0 {
		dir = a.Changes.Dirs[0]
	}
	if root := git.RepoRoot(dir); root != "" {
		dir = root
	}
	return dir
}

// OpenCurrentChanges opens one stable, live change-set tab per repository.
// The repository coordinator supplies freshness; this loader supplies the
// heavier per-file diff document only while that tab is active.
func (a *App) OpenCurrentChanges() {
	dir := a.selectedChangesDir()
	if dir == "" {
		a.StatusWarn("No repository selected")
		return
	}
	tabID := currentChangesTabID(dir)
	detail := a.EditorGroup.CurrentChangesWidgetByTab(tabID)
	created := detail == nil
	if detail == nil {
		detail = ui.NewCurrentChangesWidget(dir, a.EditorGroup.SyntaxHighlight)
		a.EditorGroup.ApplyDiffDefaults(detail)
		a.EditorGroup.OpenPluginTab(tabID, currentChangesTitle(dir, len(a.Changes.groups) > 1), detail)
	} else {
		a.EditorGroup.SwitchToTabByPath(tabID)
	}
	a.FocusEditorIfEnabled()
	if created {
		a.refreshCurrentChanges(detail, a.currentChangesSourceFingerprint(dir))
	}
	// Activating the tab makes the repository coordinator visible and starts
	// its normal freshness cycle through OnActiveContentChange.
	if a.Repository == nil && !created {
		a.refreshCurrentChanges(detail, a.currentChangesSourceFingerprint(dir))
	}
}

func (a *App) refreshActiveCurrentChanges() {
	if a == nil || a.EditorGroup == nil {
		return
	}
	if detail := a.EditorGroup.ActiveCurrentChangesWidget(); detail != nil {
		fingerprint := a.currentChangesSourceFingerprint(detail.Dir)
		state := a.currentChangesLoadState(currentChangesTabID(detail.Dir))
		if fingerprint != state.appliedFingerprint && fingerprint != state.requestedFingerprint {
			a.refreshCurrentChanges(detail, fingerprint)
		}
	}
}

func (a *App) currentChangesLoadState(tabID string) *currentChangesLoadState {
	if a.currentChangesLoads == nil {
		a.currentChangesLoads = make(map[string]*currentChangesLoadState)
	}
	state := a.currentChangesLoads[tabID]
	if state == nil {
		state = &currentChangesLoadState{}
		a.currentChangesLoads[tabID] = state
	}
	return state
}

func (a *App) refreshCurrentChanges(detail *ui.CommitDetailWidget, fingerprint string) {
	if detail == nil || !detail.CurrentChanges {
		return
	}
	tabID := currentChangesTabID(detail.Dir)
	state := a.currentChangesLoadState(tabID)
	if state.cancel != nil {
		state.cancel()
	}
	state.requestedFingerprint = fingerprint
	detail.LoadGen++
	gen := detail.LoadGen
	if len(detail.Files) == 0 && detail.Message == "" {
		detail.Loading = true
	}
	ctx, cancel := context.WithTimeout(context.Background(), currentChangesTimeout)
	state.cancel = cancel
	read := func() *CurrentChangesResult {
		result := readCurrentChanges(ctx, detail.Dir, tabID, gen)
		result.Fingerprint = fingerprint
		return result
	}
	if a.Screen == nil {
		result := read()
		cancel()
		a.ApplyCurrentChanges(result)
		return
	}
	screen := a.Screen
	go func() {
		result := read()
		cancel()
		screen.PostEvent(tcell.NewEventInterrupt(result))
	}()
}

func (a *App) ApplyCurrentChanges(result *CurrentChangesResult) {
	if result == nil {
		return
	}
	detail := a.EditorGroup.CurrentChangesWidgetByTab(result.TabID)
	if detail == nil || detail.Dir != result.Dir || detail.LoadGen != result.Gen {
		return
	}
	state := a.currentChangesLoadState(result.TabID)
	state.cancel = nil
	state.requestedFingerprint = ""
	if result.Err == "" {
		state.appliedFingerprint = result.Fingerprint
	}
	detail.SetDetail(result.Summary, result.Files, result.Err)
}

func (a *App) cleanupCurrentChangesTab(id string) {
	if state := a.currentChangesLoads[id]; state != nil && state.cancel != nil {
		state.cancel()
	}
	delete(a.currentChangesLoads, id)
	a.syncRepositoryObservation()
}

func (a *App) closeCurrentChanges() {
	for id, state := range a.currentChangesLoads {
		if state.cancel != nil {
			state.cancel()
		}
		delete(a.currentChangesLoads, id)
	}
}

// currentChangesSourceFingerprint keeps repository polling cheap. Git status
// supplies the path/state identity; file metadata catches content edits that
// remain the same status (for example M before and after an external write).
func (a *App) currentChangesSourceFingerprint(dir string) string {
	if a == nil || a.Changes == nil {
		return "status-unavailable"
	}
	var group *changesGroup
	for i := range a.Changes.groups {
		if filepath.Clean(a.Changes.groups[i].Dir) == filepath.Clean(dir) {
			group = &a.Changes.groups[i]
			break
		}
	}
	if group == nil {
		return "status-unavailable"
	}
	statuses := make([]git.FileStatus, 0, len(group.Staged)+len(group.Unstaged))
	statuses = append(statuses, group.Staged...)
	statuses = append(statuses, group.Unstaged...)
	states := mergeCurrentFileStates(statuses)
	var fingerprint strings.Builder
	fingerprint.WriteString(filepath.Clean(dir))
	for _, state := range states {
		fmt.Fprintf(&fingerprint, "\x00%s\x00%s\x00%s\x00%t\x00%t", state.status.Status,
			state.status.OldPath, state.status.Path, state.staged, state.unstaged)
		info, err := os.Stat(filepath.Join(dir, state.status.Path))
		if err != nil {
			fingerprint.WriteString("\x00missing")
			continue
		}
		fmt.Fprintf(&fingerprint, "\x00%d\x00%d\x00%d", info.Size(), info.ModTime().UnixNano(), info.Mode())
	}
	return fingerprint.String()
}

func readCurrentChanges(ctx context.Context, dir, tabID string, gen uint64) *CurrentChangesResult {
	result := &CurrentChangesResult{Dir: dir, TabID: tabID, Gen: gen}
	statuses, err := git.StatusFilesContext(ctx, dir)
	if err != nil {
		result.Err = "Could not read current changes: " + err.Error()
		return result
	}

	states := mergeCurrentFileStates(statuses)
	result.Files = make([]ui.CommitDetailFile, 0, len(states))
	staged, unstaged, mixed := 0, 0, 0
	additions, deletions := 0, 0
	for _, state := range states {
		stateLabel := "unstaged"
		switch {
		case state.staged && state.unstaged:
			stateLabel = "mixed"
			mixed++
		case state.staged:
			stateLabel = "staged"
			staged++
		default:
			unstaged++
		}
		status := combinedCurrentStatus(state)
		pathLabel := state.status.Path
		if state.status.OldPath != "" && state.status.OldPath != state.status.Path {
			pathLabel = state.status.OldPath + " → " + state.status.Path
		}
		file := ui.CommitDetailFile{
			Path:         state.status.Path,
			OldPath:      state.status.OldPath,
			Heading:      fmt.Sprintf("%s  %s · %s", ui.StatusBadge(status), pathLabel, stateLabel),
			HeadingStyle: ui.StatusStyle(status),
		}
		diffText, diffErr := git.DiffWorkingTreeFileContext(ctx, dir, git.FileStatus{
			Status: status, Path: state.status.Path, OldPath: state.status.OldPath,
		})
		if diffErr != nil {
			file.Error = "Could not read current diff for " + state.status.Path
		} else {
			file.Diff = diff.Parse(diffText)
			added, deleted := currentDiffStats(file.Diff)
			additions += added
			deletions += deleted
		}
		result.Files = append(result.Files, file)
	}
	result.Summary = currentChangesSummary(len(states), additions, deletions, staged, unstaged, mixed)
	return result
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
			if state.status.Status == "M" || state.status.Status == "" {
				state.status.Status = status.Status
			}
		} else {
			state.unstaged = true
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
