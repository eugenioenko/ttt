package app

import (
	"context"
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

	RepositoryStatus = RepositoryWorktree

	repositoryRefreshDebounce = 150 * time.Millisecond
	repositoryPollInterval    = 2 * time.Second
	repositoryStatusTimeout   = 10 * time.Second
)

type repositoryStatusEntry struct {
	Group       changesGroup
	Revision    string
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
}

func NewRepositoryState(changes *ChangesPanel, dirs []string) *RepositoryState {
	return &RepositoryState{
		changes:       changes,
		dirs:          append([]string(nil), dirs...),
		scheduler:     realRepositoryScheduler{},
		readStatus:    readRepositoryStatus,
		debounceWait:  repositoryRefreshDebounce,
		pollWait:      repositoryPollInterval,
		statusWait:    repositoryStatusTimeout,
		lastGroups:    make(map[string]changesGroup),
		lastRevisions: make(map[string]string),
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
	if s.changes != nil {
		s.changes.Dirs = append([]string(nil), dirs...)
		s.changes.multiRoot = len(dirs) > 1
		s.changes.CancelHistoryRead()
		s.changes.applyWorkingTree(nil)
		s.changes.RefreshHistory()
	}
	s.lastGroups = make(map[string]changesGroup)
	s.lastRevisions = make(map[string]string)
	s.dirty |= RepositoryWorktree | RepositoryHistory
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
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	abs = filepath.Clean(abs)
	for _, dir := range s.dirs {
		root, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(filepath.Clean(root), abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
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
	revisionChanged := false
	hadError := false
	for _, entry := range result.Entries {
		group := entry.Group
		if entry.StatusErr != nil {
			hadError = true
			if previous, ok := s.lastGroups[group.Dir]; ok {
				group = previous
			}
		}
		groups = append(groups, group)
		nextGroups[group.Dir] = group

		revision := entry.Revision
		if entry.RevisionErr != nil {
			hadError = true
			revision = s.lastRevisions[group.Dir]
		}
		if previous, ok := s.lastRevisions[group.Dir]; ok && group.Dir == selectedDir && previous != revision {
			revisionChanged = true
		}
		nextRevisions[group.Dir] = revision
	}
	s.lastGroups = nextGroups
	s.lastRevisions = nextRevisions

	if s.changes != nil {
		s.changes.applyWorkingTree(groups)
	}
	if !hadError {
		s.dirty &^= RepositoryWorktree
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
		if root := git.RepoRootContext(ctx, dir); root != "" {
			dir = root
		}
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
			Group: changesGroup{
				Dir:      dir,
				Name:     filepath.Base(dir),
				Staged:   staged,
				Unstaged: unstaged,
			},
			Revision:    revision,
			StatusErr:   statusErr,
			RevisionErr: revisionErr,
		})
	}
	return entries
}

func (a *App) syncRepositoryObservation() {
	if a == nil || a.Repository == nil || a.Sidebar == nil || a.SplitPanel == nil {
		return
	}
	visible := a.Sidebar.Visible && a.SplitPanel.ShowLeft && a.Sidebar.ActivePanel == "changes"
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
