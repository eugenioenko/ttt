package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/gdamore/tcell/v3"
)

type RepositoryResource uint8

const (
	RepositoryWorktree RepositoryResource = 1 << iota
	RepositoryHistory
	RepositoryCurrentChanges

	RepositoryStatus = RepositoryWorktree

	repositoryRefreshDebounce = 150 * time.Millisecond
	repositoryPollInterval    = 2 * time.Second
	repositoryStatusTimeout   = 10 * time.Second
)

type repositoryStatusEntry struct {
	SourceDir   string
	Group       changesGroup
	Revision    string
	RootErr     error
	StatusErr   error
	RevisionErr error
}

type RepositoryStatusResult struct {
	Seq     uint64
	Entries []repositoryStatusEntry
}

type RepositoryInvalidationRequest struct {
	Path      string
	Resources RepositoryResource
}

type repositoryDebounceTick struct{ Gen uint64 }
type repositoryPollTick struct{ Gen uint64 }

type repositoryTimer interface {
	Stop() bool
}

type repositoryScheduler interface {
	AfterFunc(time.Duration, func()) repositoryTimer
}

type realRepositoryScheduler struct{}

func (realRepositoryScheduler) AfterFunc(d time.Duration, f func()) repositoryTimer {
	return time.AfterFunc(d, f)
}

type RepositoryState struct {
	changes *ChangesPanel
	dirs    []string
	poster  eventPoster

	scheduler    repositoryScheduler
	readStatus   func(context.Context, []string, uint64) *RepositoryStatusResult
	pathIdentity func(string) (string, error)
	debounceWait time.Duration
	pollWait     time.Duration
	statusWait   time.Duration

	started bool
	closed  bool
	visible bool
	dirty   RepositoryResource

	requested    uint64
	inFlight     bool
	refreshAgain bool
	statusCancel context.CancelFunc

	debounceTimer repositoryTimer
	debounceGen   uint64
	pollTimer     repositoryTimer
	pollGen       uint64

	lastGroups    map[string]changesGroup
	lastRevisions map[string]string
	lastRoots     map[string]string

	currentRoot               string
	currentTabID              string
	currentEpoch              uint64
	currentRequest            uint64
	currentInFlightRequest    uint64
	currentInFlightIdentity   [32]byte
	currentInFlight           bool
	currentRefreshAgain       bool
	currentDirty              bool
	currentCancel             context.CancelFunc
	currentAppliedFingerprint string
	currentInputs             map[string]currentChangesInput
	currentInputReady         bool
	readCurrentChanges        func(context.Context, string, string, string, uint64, uint64, []git.FileStatus) *CurrentChangesResult
	currentChangesHandler     func(*CurrentChangesResult)
}

func NewRepositoryState(changes *ChangesPanel, dirs []string) *RepositoryState {
	return &RepositoryState{
		changes:            changes,
		dirs:               append([]string(nil), dirs...),
		scheduler:          realRepositoryScheduler{},
		readStatus:         readRepositoryStatus,
		debounceWait:       repositoryRefreshDebounce,
		pollWait:           repositoryPollInterval,
		statusWait:         repositoryStatusTimeout,
		lastGroups:         make(map[string]changesGroup),
		lastRevisions:      make(map[string]string),
		lastRoots:          make(map[string]string),
		currentInputs:      make(map[string]currentChangesInput),
		readCurrentChanges: readCurrentChanges,
	}
}

func (s *RepositoryState) SetPoster(poster eventPoster) {
	if s == nil || s.closed {
		return
	}
	s.poster = poster
}

func (s *RepositoryState) Start() {
	if s == nil || s.started || s.closed {
		return
	}
	s.started = true
	s.dirty |= RepositoryHistory
	s.RefreshNow(RepositoryWorktree)
}

func (s *RepositoryState) SetDirs(dirs []string) {
	if s == nil || s.closed {
		return
	}
	s.dirs = append([]string(nil), dirs...)
	s.stopCurrentChanges()
	if s.changes != nil {
		s.changes.Dirs = append([]string(nil), dirs...)
		s.changes.multiRoot = len(dirs) > 1
		s.changes.CancelHistoryRead()
		s.changes.applyWorkingTree(nil)
		s.changes.RefreshHistory()
	}
	s.lastGroups = make(map[string]changesGroup)
	s.lastRevisions = make(map[string]string)
	s.lastRoots = make(map[string]string)
	s.currentInputs = make(map[string]currentChangesInput)
	s.dirty |= RepositoryWorktree | RepositoryHistory | RepositoryCurrentChanges
	s.stopDebounce()
	s.stopPoll()
	s.requested++
	if s.inFlight {
		s.refreshAgain = true
		if s.statusCancel != nil {
			s.statusCancel()
		}
	} else if s.started {
		s.startStatus()
	}
}

func (s *RepositoryState) SetVisible(visible bool) {
	if s == nil || s.closed || s.visible == visible {
		return
	}
	s.visible = visible
	if !visible {
		s.stopPoll()
		return
	}
	if s.dirty&RepositoryHistory != 0 {
		s.refreshHistory()
	}
	if s.dirty&RepositoryWorktree != 0 {
		s.RefreshNow(RepositoryWorktree)
		return
	}
	s.armPoll()
}

func (s *RepositoryState) InvalidateAll(resources RepositoryResource) {
	if s == nil || s.closed || resources == 0 {
		return
	}
	s.dirty |= resources
	if resources&RepositoryWorktree != 0 && s.currentRoot != "" {
		s.dirty |= RepositoryCurrentChanges
		s.currentDirty = true
		s.currentInputReady = false
	}
	if resources&RepositoryHistory != 0 && s.visible {
		s.refreshHistory()
	}
	if resources&RepositoryWorktree == 0 {
		return
	}
	s.requested++
	if s.inFlight {
		s.refreshAgain = true
	}
	if s.poster == nil {
		if !s.inFlight {
			s.startStatus()
		}
		return
	}
	s.scheduleDebounce()
}

func (s *RepositoryState) InvalidatePath(path string, resources RepositoryResource) {
	if s == nil || s.closed || path == "" || resources == 0 {
		return
	}
	identity, err := s.resolvePathIdentity(path)
	if err != nil {
		return
	}
	for _, root := range s.repositoryIdentityRoots() {
		if pathWithin(root, identity) {
			s.InvalidateAll(resources)
			return
		}
	}
}

func (s *RepositoryState) RefreshNow(resources RepositoryResource) {
	if s == nil || s.closed || resources == 0 {
		return
	}
	s.dirty |= resources
	if resources&RepositoryWorktree != 0 && s.currentRoot != "" {
		s.dirty |= RepositoryCurrentChanges
		s.currentDirty = true
		s.currentInputReady = false
	}
	if resources&RepositoryHistory != 0 && s.visible {
		s.refreshHistory()
	}
	if resources&RepositoryWorktree == 0 {
		return
	}
	s.requested++
	s.stopDebounce()
	if s.inFlight {
		s.refreshAgain = true
		return
	}
	s.startStatus()
}

func (s *RepositoryState) startStatus() {
	if s.closed || s.inFlight {
		return
	}
	s.inFlight = true
	s.refreshAgain = false
	seq := s.requested
	dirs := append([]string(nil), s.dirs...)
	read := s.readStatus
	ctx, cancel := context.WithTimeout(context.Background(), s.statusWait)
	s.statusCancel = cancel
	if s.poster == nil {
		result := read(ctx, dirs, seq)
		cancel()
		s.HandleStatus(result)
		return
	}
	poster := s.poster
	go func() {
		result := read(ctx, dirs, seq)
		cancel()
		_ = poster.PostEvent(tcell.NewEventInterrupt(result))
	}()
}

func (s *RepositoryState) HandleStatus(result *RepositoryStatusResult) {
	if s == nil || result == nil || s.closed {
		return
	}
	s.inFlight = false
	s.statusCancel = nil
	if result.Seq != s.requested {
		s.stopDebounce()
		if s.refreshAgain && s.dirty&RepositoryWorktree != 0 {
			s.startStatus()
		}
		return
	}

	selectedDir := ""
	if s.changes != nil {
		selectedDir = s.changes.selectedGroupDir()
		if selectedDir == "" && len(result.Entries) > 0 {
			selectedDir = result.Entries[0].Group.Dir
		}
	}

	groups := make([]changesGroup, 0, len(result.Entries))
	nextGroups := make(map[string]changesGroup, len(result.Entries))
	nextRevisions := make(map[string]string, len(result.Entries))
	seenGroups := make(map[string]bool, len(result.Entries))
	revisionChanged := false
	hadError := false
	currentStatusFresh := false
	currentStatusErrors := make(map[string]error)
	for _, entry := range result.Entries {
		group := entry.Group
		sourceDir := cleanAbsolutePath(entry.SourceDir)
		if sourceDir == "" {
			sourceDir = cleanAbsolutePath(group.Dir)
		}
		if entry.RootErr != nil {
			hadError = true
			if previousRoot := s.previousRootForSource(sourceDir); previousRoot != "" {
				group.Dir = previousRoot
				group.Name = filepath.Base(previousRoot)
			}
		}
		if sourceDir != "" && entry.RootErr == nil {
			s.lastRoots[sourceDir] = group.Dir
		}
		if entry.StatusErr != nil {
			hadError = true
			if previous, ok := s.lastGroups[group.Dir]; ok {
				group = previous
			}
		}

		revision := entry.Revision
		if entry.RevisionErr != nil {
			hadError = true
			revision = s.lastRevisions[group.Dir]
		}
		if seenGroups[group.Dir] {
			continue
		}
		seenGroups[group.Dir] = true
		groups = append(groups, group)
		nextGroups[group.Dir] = group
		if previous, ok := s.lastRevisions[group.Dir]; ok && group.Dir == selectedDir && previous != revision {
			revisionChanged = true
		}
		nextRevisions[group.Dir] = revision
		entryErr := errors.Join(entry.RootErr, entry.StatusErr, entry.RevisionErr)
		if entryErr != nil {
			currentStatusErrors[group.Dir] = entryErr
		}
		if group.Dir == s.currentRoot && entryErr == nil {
			s.currentInputReady = true
			currentStatusFresh = true
		}
		if entryErr == nil {
			statuses := make([]git.FileStatus, 0, len(group.Staged)+len(group.Unstaged))
			statuses = append(statuses, group.Staged...)
			statuses = append(statuses, group.Unstaged...)
			s.currentInputs[group.Dir] = newCurrentChangesInput(revision, statuses)
		}
	}
	s.lastGroups = nextGroups
	s.lastRevisions = nextRevisions

	if s.changes != nil {
		s.changes.applyWorkingTree(groups)
	}
	if !hadError {
		s.dirty &^= RepositoryWorktree
	}
	if currentErr := currentStatusErrors[s.currentRoot]; currentErr != nil {
		s.reportCurrentChangesStatusError(currentErr)
	}
	if currentStatusFresh {
		s.requestCurrentChanges()
	}
	if revisionChanged {
		s.dirty |= RepositoryHistory
	}
	if s.visible {
		if s.dirty&RepositoryHistory != 0 {
			s.refreshHistory()
		} else if s.changes != nil {
			s.changes.refreshCommitLog()
		}
		s.armPoll()
	}
}

func (s *RepositoryState) refreshHistory() {
	if s.changes != nil {
		s.changes.RefreshHistory()
	}
}

func (s *RepositoryState) HandleHistory(err error) {
	if s == nil || s.closed {
		return
	}
	if err != nil {
		s.dirty |= RepositoryHistory
		return
	}
	s.dirty &^= RepositoryHistory
}

func (s *RepositoryState) scheduleDebounce() {
	if s.poster == nil || s.closed {
		return
	}
	if s.debounceTimer != nil {
		s.debounceTimer.Stop()
	}
	s.debounceGen++
	gen := s.debounceGen
	poster := s.poster
	s.debounceTimer = s.scheduler.AfterFunc(s.debounceWait, func() {
		_ = poster.PostEvent(tcell.NewEventInterrupt(&repositoryDebounceTick{Gen: gen}))
	})
}

func (s *RepositoryState) HandleDebounce(tick *repositoryDebounceTick) {
	if s == nil || tick == nil || s.closed || tick.Gen != s.debounceGen {
		return
	}
	s.debounceTimer = nil
	if !s.inFlight && s.dirty&RepositoryWorktree != 0 {
		s.startStatus()
	}
}

func (s *RepositoryState) armPoll() {
	if s.poster == nil || s.closed || !s.visible || s.pollTimer != nil {
		return
	}
	s.pollGen++
	gen := s.pollGen
	poster := s.poster
	s.pollTimer = s.scheduler.AfterFunc(s.pollWait, func() {
		_ = poster.PostEvent(tcell.NewEventInterrupt(&repositoryPollTick{Gen: gen}))
	})
}

func (s *RepositoryState) HandlePoll(tick *repositoryPollTick) {
	if s == nil || tick == nil || s.closed || tick.Gen != s.pollGen || !s.visible {
		return
	}
	s.pollTimer = nil
	s.RefreshNow(RepositoryWorktree)
}

func (s *RepositoryState) stopDebounce() {
	if s.debounceTimer != nil {
		s.debounceTimer.Stop()
		s.debounceTimer = nil
	}
	s.debounceGen++
}

func (s *RepositoryState) stopPoll() {
	if s.pollTimer != nil {
		s.pollTimer.Stop()
		s.pollTimer = nil
	}
	s.pollGen++
}

func (s *RepositoryState) Close() {
	if s == nil || s.closed {
		return
	}
	s.closed = true
	s.stopCurrentChanges()
	s.stopDebounce()
	s.stopPoll()
	s.requested++
	if s.statusCancel != nil {
		s.statusCancel()
		s.statusCancel = nil
	}
	if s.changes != nil {
		s.changes.CancelHistoryRead()
	}
}

func readRepositoryStatus(ctx context.Context, dirs []string, seq uint64) *RepositoryStatusResult {
	return &RepositoryStatusResult{Seq: seq, Entries: scanRepositoryStatus(ctx, dirs)}
}

func scanRepositoryStatus(ctx context.Context, dirs []string) []repositoryStatusEntry {
	entries := make([]repositoryStatusEntry, 0, len(dirs))
	seen := make(map[string]bool)
	for _, dir := range dirs {
		sourceDir := cleanAbsolutePath(dir)
		root, rootErr := git.RepoRootWithErrorContext(ctx, dir)
		if rootErr == nil && root != "" {
			dir = root
		}
		dir = cleanAbsolutePath(dir)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		files, statusErr := git.StatusFilesContext(ctx, dir)
		var staged, unstaged []git.FileStatus
		if statusErr == nil {
			for _, file := range files {
				if file.Staged {
					staged = append(staged, file)
				} else {
					unstaged = append(unstaged, file)
				}
			}
		}
		revision, revisionErr := git.RevisionIdentityContext(ctx, dir)
		entries = append(entries, repositoryStatusEntry{
			SourceDir: sourceDir,
			Group: changesGroup{
				Dir:      dir,
				Name:     filepath.Base(dir),
				Staged:   staged,
				Unstaged: unstaged,
			},
			Revision:    revision,
			RootErr:     rootErr,
			StatusErr:   statusErr,
			RevisionErr: revisionErr,
		})
	}
	return entries
}

func (s *RepositoryState) previousRootForSource(source string) string {
	if root := s.lastRoots[source]; root != "" {
		return root
	}
	resolved, err := resolveRepositoryPathIdentity(source)
	if err != nil {
		resolved = source
	}
	best := ""
	for root := range s.lastGroups {
		if pathWithin(root, resolved) && len(root) > len(best) {
			best = root
		}
	}
	return best
}

func (s *RepositoryState) repositoryIdentityRoots() []string {
	roots := make([]string, 0, len(s.lastGroups)+len(s.dirs))
	seen := make(map[string]bool)
	add := func(path string) {
		identity, err := s.resolvePathIdentity(path)
		if err != nil || identity == "" || seen[identity] {
			return
		}
		seen[identity] = true
		roots = append(roots, identity)
	}
	for root := range s.lastGroups {
		add(root)
	}
	for _, root := range s.lastRoots {
		add(root)
	}
	for _, dir := range s.dirs {
		add(dir)
	}
	return roots
}

func (s *RepositoryState) resolvePathIdentity(path string) (string, error) {
	if s.pathIdentity != nil {
		return s.pathIdentity(path)
	}
	return resolveRepositoryPathIdentity(path)
}

func resolveRepositoryPathIdentity(path string) (string, error) {
	return resolveRepositoryPathIdentityWith(path, os.Lstat, filepath.EvalSymlinks)
}

func resolveRepositoryPathIdentityWith(
	path string,
	lstat func(string) (os.FileInfo, error),
	evalSymlinks func(string) (string, error),
) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	resolved, resolveErr := stableSymlinkIdentityWith(abs, evalSymlinks)
	if resolveErr == nil {
		return resolved, nil
	}
	if _, lstatErr := lstat(abs); lstatErr == nil {
		return "", resolveErr
	} else if !errors.Is(lstatErr, os.ErrNotExist) {
		return "", lstatErr
	}
	parent := filepath.Dir(abs)
	resolvedParent, parentErr := stableSymlinkIdentityWith(parent, evalSymlinks)
	if parentErr != nil {
		return "", resolveErr
	}
	if _, lstatErr := lstat(abs); lstatErr == nil {
		return stableSymlinkIdentityWith(abs, evalSymlinks)
	} else if !errors.Is(lstatErr, os.ErrNotExist) {
		return "", lstatErr
	}
	confirmedParent, err := stableSymlinkIdentityWith(parent, evalSymlinks)
	if err != nil || confirmedParent != resolvedParent {
		return "", errors.New("repository path parent changed while resolving")
	}
	return filepath.Join(resolvedParent, filepath.Base(abs)), nil
}

func stableSymlinkIdentityWith(path string, evalSymlinks func(string) (string, error)) (string, error) {
	first, err := evalSymlinks(path)
	if err != nil {
		return "", err
	}
	second, err := evalSymlinks(path)
	if err != nil {
		return "", err
	}
	first = filepath.Clean(first)
	second = filepath.Clean(second)
	if first != second {
		return "", errors.New("repository path changed while resolving")
	}
	return first, nil
}

func cleanAbsolutePath(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return filepath.Clean(abs)
}

func pathWithin(root, path string) bool {
	root = cleanAbsolutePath(root)
	path = cleanAbsolutePath(path)
	if root == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (a *App) syncRepositoryObservation() {
	if a == nil || a.Repository == nil || a.Sidebar == nil || a.SplitPanel == nil {
		return
	}
	visible := a.Sidebar.Visible && a.SplitPanel.ShowLeft && a.Sidebar.ActivePanel == "changes"
	if detail := a.EditorGroup.ActiveCurrentChangesWidget(); detail != nil {
		tabID := currentChangesTabID(detail.Dir)
		detail.Incarnation = a.Repository.SetCurrentChangesRoot(detail.Dir, tabID)
		a.Repository.EnsureCurrentChanges()
		visible = true
	} else {
		a.Repository.SetCurrentChangesRoot("", "")
	}
	a.Repository.SetVisible(visible)
}

func (a *App) invalidateRepositoryPath(path string, resources RepositoryResource) {
	if a != nil && a.Repository != nil {
		a.Repository.InvalidatePath(path, resources)
	}
}

func (a *App) invalidateAllRepositories(resources RepositoryResource) {
	if a != nil && a.Repository != nil {
		a.Repository.InvalidateAll(resources)
	}
}

func (a *App) postRepositoryInvalidation(path string, resources RepositoryResource) {
	if a == nil || resources == 0 {
		return
	}
	if a.Screen == nil {
		if path == "" {
			a.invalidateAllRepositories(resources)
		} else {
			a.invalidateRepositoryPath(path, resources)
		}
		return
	}
	a.Screen.PostEvent(tcell.NewEventInterrupt(&RepositoryInvalidationRequest{Path: path, Resources: resources}))
}

func (a *App) handleRepositoryInvalidation(request *RepositoryInvalidationRequest) {
	if request == nil {
		return
	}
	if request.Path == "" {
		a.invalidateAllRepositories(request.Resources)
		return
	}
	a.invalidateRepositoryPath(request.Path, request.Resources)
}
