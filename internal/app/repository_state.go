package app

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/gdamore/tcell/v3"
)

// RepositoryResource identifies independently stale Git-derived data.
// Working-tree changes do not imply that immutable history changed.
type RepositoryResource uint8

const (
	RepositoryStatus RepositoryResource = 1 << iota
	RepositoryHistory

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

// RepositoryStatusResult carries one complete multi-root observation back to
// the event loop. Seq is the desired-state sequence, not merely a request ID:
// invalidating while a read is running makes that result stale immediately.
type RepositoryStatusResult struct {
	Seq     uint64
	Entries []repositoryStatusEntry
}

// RepositoryInvalidationRequest lets background plugin/debug work return to
// the main event loop before touching coordinator state. An empty Path means
// every configured repository may have changed.
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

// RepositoryState owns freshness and scheduling for Git-derived application
// state. Widgets still own presentation state such as selection and expansion;
// they no longer need every mutation source to remember a raw Refresh call.
//
// All methods except timer callbacks and readStatus run on the main event-loop
// goroutine. Those callbacks only post typed events back to that boundary.
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
	s.poster = poster
}

// Start performs the initial status observation even while Changes is hidden,
// so its dirty badge is accurate. History waits until the panel is visible.
func (s *RepositoryState) Start() {
	if s == nil || s.started || s.closed {
		return
	}
	s.started = true
	s.dirty |= RepositoryHistory
	s.RefreshNow(RepositoryStatus)
}

func (s *RepositoryState) SetDirs(dirs []string) {
	if s == nil || s.closed {
		return
	}
	s.dirs = append([]string(nil), dirs...)
	if s.changes != nil {
		s.changes.Dirs = append([]string(nil), dirs...)
		s.changes.multiRoot = len(dirs) > 1
	}
	s.lastGroups = make(map[string]changesGroup)
	s.lastRevisions = make(map[string]string)
	s.dirty |= RepositoryStatus | RepositoryHistory
	if s.started {
		s.RefreshNow(RepositoryStatus)
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
	// Visibility is a reader request for a current view, not just permission to
	// poll later. Status lands first; pending history refreshes from that result.
	s.RefreshNow(RepositoryStatus)
}

func (s *RepositoryState) InvalidateAll(resources RepositoryResource) {
	if s == nil || s.closed || resources == 0 {
		return
	}
	s.dirty |= resources
	if resources&RepositoryStatus != 0 {
		s.requested++
		if s.poster == nil {
			s.startStatus()
		} else {
			s.scheduleDebounce()
		}
		return
	}
	if resources&RepositoryHistory != 0 && s.visible {
		s.refreshHistory()
	}
}

// InvalidatePath ignores writes outside the configured workspace roots. The
// current scan is multi-root and coalesced, so one relevant path invalidates the
// complete status resource without spawning a Git process on the caller path.
func (s *RepositoryState) InvalidatePath(path string, resources RepositoryResource) {
	if s == nil || path == "" {
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
	if resources&RepositoryStatus != 0 {
		s.requested++
		s.stopDebounce()
		if !s.inFlight {
			s.startStatus()
		}
		return
	}
	if resources&RepositoryHistory != 0 && s.visible {
		s.refreshHistory()
	}
}

func (s *RepositoryState) startStatus() {
	if s.closed || s.inFlight {
		return
	}
	s.inFlight = true
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
		// If the debounce already fired while the older read was in flight,
		// immediately begin the one required follow-up. Otherwise its timer owns
		// the trailing edge and will start it.
		if s.debounceTimer == nil {
			s.startStatus()
		}
		return
	}

	groups := make([]changesGroup, 0, len(result.Entries))
	nextGroups := make(map[string]changesGroup, len(result.Entries))
	nextRevisions := make(map[string]string, len(result.Entries))
	selectedDir := ""
	if s.changes != nil {
		selectedDir = s.changes.selectedGroupDir()
		if selectedDir == "" && len(result.Entries) > 0 {
			selectedDir = result.Entries[0].Group.Dir
		}
	}
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
			revision = s.lastRevisions[group.Dir]
		}
		nextRevisions[group.Dir] = revision
		if previous, ok := s.lastRevisions[group.Dir]; ok && group.Dir == selectedDir && previous != revision {
			revisionChanged = true
		}
	}
	s.lastGroups = nextGroups
	s.lastRevisions = nextRevisions

	if s.changes != nil {
		s.changes.applyWorkingTree(groups)
	}
	if !hadError {
		s.dirty &^= RepositoryStatus
	}
	if revisionChanged {
		s.dirty |= RepositoryHistory
	}
	if s.visible {
		if s.dirty&RepositoryHistory != 0 {
			s.refreshHistory()
		} else if s.changes != nil {
			// Switching the selected workspace root still needs a log read, while
			// the same root remains guarded by lastLogDir.
			s.changes.refreshCommitLog()
		}
		s.armPoll()
	}
}

func (s *RepositoryState) refreshHistory() {
	if s.changes == nil {
		return
	}
	s.changes.RefreshHistory()
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
	if !s.inFlight && s.dirty&RepositoryStatus != 0 {
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
	s.RefreshNow(RepositoryStatus)
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
	if s.statusCancel != nil {
		s.statusCancel()
		s.statusCancel = nil
	}
	if s.changes != nil {
		s.changes.CancelHistoryRead()
	}
	// A bounded Git read may still finish, but its event is ignored after close.
	s.requested++
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
	visible = visible || a.EditorGroup.ActiveCurrentChangesWidget() != nil
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
	a.Screen.PostEvent(tcell.NewEventInterrupt(&RepositoryInvalidationRequest{
		Path: path, Resources: resources,
	}))
}

func (a *App) handleRepositoryInvalidation(request *RepositoryInvalidationRequest) {
	if request.Path == "" {
		a.invalidateAllRepositories(request.Resources)
		return
	}
	a.invalidateRepositoryPath(request.Path, request.Resources)
}
