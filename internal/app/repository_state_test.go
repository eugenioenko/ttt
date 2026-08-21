package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/git"
)

type fakeRepositoryTimer struct {
	fn      func()
	stopped bool
}

func (t *fakeRepositoryTimer) Stop() bool {
	wasActive := !t.stopped
	t.stopped = true
	return wasActive
}

func (t *fakeRepositoryTimer) fire() {
	if !t.stopped {
		t.fn()
	}
}

func (t *fakeRepositoryTimer) fireLate() { t.fn() }

type fakeRepositoryScheduler struct {
	timers []*fakeRepositoryTimer
}

func (s *fakeRepositoryScheduler) AfterFunc(_ time.Duration, fn func()) repositoryTimer {
	timer := &fakeRepositoryTimer{fn: fn}
	s.timers = append(s.timers, timer)
	return timer
}

func (s *fakeRepositoryScheduler) latest(t *testing.T) *fakeRepositoryTimer {
	t.Helper()
	if len(s.timers) == 0 {
		t.Fatal("no repository timer was scheduled")
	}
	return s.timers[len(s.timers)-1]
}

func repositoryResult(seq uint64, revision string, files ...git.FileStatus) *RepositoryStatusResult {
	group := changesGroup{Dir: "/repo", Name: "repo"}
	for _, file := range files {
		if file.Staged {
			group.Staged = append(group.Staged, file)
		} else {
			group.Unstaged = append(group.Unstaged, file)
		}
	}
	return &RepositoryStatusResult{Seq: seq, Entries: []repositoryStatusEntry{{
		Group: group, Revision: revision,
	}}}
}

func testRepositoryState(changes *ChangesPanel) (*RepositoryState, *fakeRepositoryScheduler, *testPoster) {
	s := NewRepositoryState(changes, []string{"/repo"})
	scheduler := &fakeRepositoryScheduler{}
	poster := newTestPoster()
	s.scheduler = scheduler
	s.poster = poster
	s.started = true
	return s, scheduler, poster
}

func TestRepositoryInvalidationsCoalesce(t *testing.T) {
	s, scheduler, poster := testRepositoryState(nil)
	var reads atomic.Int32
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		reads.Add(1)
		return repositoryResult(seq, "head")
	}

	s.InvalidateAll(RepositoryStatus)
	s.InvalidateAll(RepositoryStatus)
	s.InvalidateAll(RepositoryStatus)
	if len(scheduler.timers) != 3 {
		t.Fatalf("scheduled %d timers, want three trailing-edge replacements", len(scheduler.timers))
	}
	for _, timer := range scheduler.timers[:2] {
		if !timer.stopped {
			t.Error("a superseded debounce timer remained active")
		}
	}

	scheduler.latest(t).fire()
	s.HandleDebounce(poster.await(t).(*repositoryDebounceTick))
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	if got := reads.Load(); got != 1 {
		t.Fatalf("status reads = %d, want 1", got)
	}
}

func TestRepositoryStateRunsOneFollowUpAfterInFlightInvalidation(t *testing.T) {
	s, scheduler, poster := testRepositoryState(nil)
	started := make(chan uint64, 2)
	release := make(chan struct{}, 2)
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		started <- seq
		<-release
		return repositoryResult(seq, "head")
	}

	s.InvalidateAll(RepositoryStatus)
	scheduler.latest(t).fire()
	s.HandleDebounce(poster.await(t).(*repositoryDebounceTick))
	first := <-started

	s.InvalidateAll(RepositoryStatus)
	s.InvalidateAll(RepositoryStatus)
	scheduler.latest(t).fire()
	s.HandleDebounce(poster.await(t).(*repositoryDebounceTick))
	select {
	case seq := <-started:
		t.Fatalf("overlapping status read %d started while %d was in flight", seq, first)
	default:
	}

	release <- struct{}{}
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	second := <-started
	if second == first || second != s.requested {
		t.Fatalf("follow-up seq = %d, first = %d, requested = %d", second, first, s.requested)
	}
	release <- struct{}{}
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
}

func TestWorkingTreeStatusDoesNotReloadUnchangedHistory(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.groups = []changesGroup{{Dir: "/repo", Name: "repo"}}
	cp.lastLogDir = "/repo"
	cp.logDir = "/repo"
	s, _, _ := testRepositoryState(cp)
	s.lastGroups["/repo"] = cp.groups[0]
	s.lastRevisions["/repo"] = "same"
	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryStatus
	before := cp.logGen

	s.HandleStatus(repositoryResult(1, "same", git.FileStatus{Status: "M", Path: "a.txt"}))
	if cp.logGen != before {
		t.Fatalf("working-tree apply changed log generation from %d to %d", before, cp.logGen)
	}
	if cp.TotalChanges() != 1 {
		t.Fatalf("working-tree changes = %d, want 1", cp.TotalChanges())
	}
}

func TestStatusFailurePreservesLastGoodSnapshot(t *testing.T) {
	cp := NewChangesPanel("/repo")
	previous := changesGroup{Dir: "/repo", Name: "repo", Unstaged: []git.FileStatus{{Status: "M", Path: "kept.txt"}}}
	s, _, _ := testRepositoryState(cp)
	s.lastGroups["/repo"] = previous
	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryStatus

	s.HandleStatus(&RepositoryStatusResult{Seq: 1, Entries: []repositoryStatusEntry{{
		Group: changesGroup{Dir: "/repo", Name: "repo"}, StatusErr: errors.New("index locked"),
	}}})
	if cp.TotalChanges() != 1 || cp.groups[0].Unstaged[0].Path != "kept.txt" {
		t.Fatalf("failed scan replaced last good state: %+v", cp.groups)
	}
	if s.dirty&RepositoryStatus == 0 {
		t.Error("failed status scan was marked fresh")
	}
}

func TestVisibleRepositoryStatePollLifecycle(t *testing.T) {
	s, scheduler, poster := testRepositoryState(nil)
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		return repositoryResult(seq, "head")
	}

	s.SetVisible(true)
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	poll := scheduler.latest(t)
	if poll.stopped {
		t.Fatal("visible state armed a stopped poll timer")
	}
	count := len(scheduler.timers)
	s.SetVisible(true)
	if len(scheduler.timers) != count {
		t.Error("repeated visible state duplicated the poll timer")
	}

	s.SetVisible(false)
	if !poll.stopped || s.pollTimer != nil {
		t.Error("hiding Changes did not cancel polling")
	}
	poll.fireLate()
	s.HandlePoll(poster.await(t).(*repositoryPollTick))
	if s.inFlight {
		t.Error("a late hidden poll started a status read")
	}
}

func TestShowingChangesDoesNotReloadUnchangedHistory(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.groups = []changesGroup{{Dir: "/repo", Name: "repo"}}
	cp.buildTree()
	cp.lastLogDir = "/repo"
	cp.logDir = "/repo"
	s, _, poster := testRepositoryState(cp)
	s.lastGroups["/repo"] = cp.groups[0]
	s.lastRevisions["/repo"] = "same"
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		return repositoryResult(seq, "same")
	}
	before := cp.logGen

	s.SetVisible(true)
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	if cp.logGen != before {
		t.Fatalf("showing Changes reloaded unchanged history: generation %d -> %d", before, cp.logGen)
	}
}

func TestRevisionChangeOnlyReloadsSelectedRoot(t *testing.T) {
	cp := NewChangesPanel("/repo-a", "/repo-b")
	cp.groups = []changesGroup{{Dir: "/repo-a", Name: "repo-a"}, {Dir: "/repo-b", Name: "repo-b"}}
	cp.buildTree()
	cp.lastLogDir = "/repo-a"
	cp.logDir = "/repo-a"
	s, _, _ := testRepositoryState(cp)
	s.visible = true
	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryStatus
	s.lastGroups["/repo-a"] = cp.groups[0]
	s.lastGroups["/repo-b"] = cp.groups[1]
	s.lastRevisions["/repo-a"] = "a-old"
	s.lastRevisions["/repo-b"] = "b-old"
	before := cp.logGen

	s.HandleStatus(&RepositoryStatusResult{Seq: 1, Entries: []repositoryStatusEntry{
		{Group: cp.groups[0], Revision: "a-old"},
		{Group: cp.groups[1], Revision: "b-new"},
	}})
	if cp.logGen != before {
		t.Fatalf("unselected root revision reloaded selected history: generation %d -> %d", before, cp.logGen)
	}
}

func TestSelectedRevisionChangeReloadsHistory(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.groups = []changesGroup{{Dir: "/repo", Name: "repo"}}
	cp.buildTree()
	cp.lastLogDir = "/repo"
	cp.logDir = "/repo"
	cp.Screen = newTestPoster()
	s, _, _ := testRepositoryState(cp)
	s.visible = true
	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryStatus
	s.lastGroups["/repo"] = cp.groups[0]
	s.lastRevisions["/repo"] = "old"
	before := cp.logGen

	s.HandleStatus(repositoryResult(1, "new"))
	if cp.logGen != before+1 {
		t.Fatalf("selected revision change history generation = %d, want %d", cp.logGen, before+1)
	}
	if s.dirty&RepositoryHistory == 0 {
		t.Fatal("selected revision change was marked fresh before its history result")
	}
}

func TestRepositoryStateCloseCancelsInFlightStatus(t *testing.T) {
	s, _, _ := testRepositoryState(nil)
	started := make(chan struct{})
	canceled := make(chan struct{})
	s.statusWait = time.Hour
	s.readStatus = func(ctx context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		close(started)
		<-ctx.Done()
		close(canceled)
		return repositoryResult(seq, "")
	}

	s.RefreshNow(RepositoryStatus)
	<-started
	s.Close()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel the in-flight status context")
	}
}

func TestRepositoryStateCloseIgnoresLateTimers(t *testing.T) {
	s, scheduler, poster := testRepositoryState(nil)
	var reads atomic.Int32
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		reads.Add(1)
		return repositoryResult(seq, "head")
	}

	s.InvalidateAll(RepositoryStatus)
	debounce := scheduler.latest(t)
	s.visible = true
	s.armPoll()
	poll := scheduler.latest(t)
	s.Close()
	debounce.fireLate()
	poll.fireLate()
	s.HandleDebounce(poster.await(t).(*repositoryDebounceTick))
	s.HandlePoll(poster.await(t).(*repositoryPollTick))
	if reads.Load() != 0 {
		t.Fatalf("late timers started %d reads after close", reads.Load())
	}
	if !debounce.stopped || !poll.stopped {
		t.Error("Close did not stop both timers")
	}
}
