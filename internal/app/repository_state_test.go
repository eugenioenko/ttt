package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
	"github.com/gdamore/tcell/v3"
)

type testEventPoster struct {
	events chan tcell.Event
}

func newTestEventPoster() *testEventPoster {
	return &testEventPoster{events: make(chan tcell.Event, 32)}
}

func (p *testEventPoster) PostEvent(event tcell.Event) error {
	p.events <- event
	return nil
}

func (p *testEventPoster) await(t *testing.T) any {
	t.Helper()
	select {
	case event := <-p.events:
		interrupt, ok := event.(*tcell.EventInterrupt)
		if !ok {
			t.Fatalf("event type = %T, want interrupt", event)
		}
		return interrupt.Data()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for repository event")
		return nil
	}
}

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

func repositoryResult(seq uint64, dir, revision string, files ...git.FileStatus) *RepositoryStatusResult {
	group := changesGroup{Dir: dir, Name: filepath.Base(dir)}
	for _, file := range files {
		if file.Staged {
			group.Staged = append(group.Staged, file)
		} else {
			group.Unstaged = append(group.Unstaged, file)
		}
	}
	return &RepositoryStatusResult{Seq: seq, Entries: []repositoryStatusEntry{{Group: group, Revision: revision}}}
}

func testRepositoryState(changes *ChangesPanel, dir string) (*RepositoryState, *fakeRepositoryScheduler, *testEventPoster) {
	s := NewRepositoryState(changes, []string{dir})
	scheduler := &fakeRepositoryScheduler{}
	poster := newTestEventPoster()
	s.scheduler = scheduler
	s.poster = poster
	s.started = true
	return s, scheduler, poster
}

func TestRepositoryInvalidationBurstCoalesces(t *testing.T) {
	s, scheduler, poster := testRepositoryState(nil, "/repo")
	var reads atomic.Int32
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		reads.Add(1)
		return repositoryResult(seq, "/repo", "head")
	}

	s.InvalidateAll(RepositoryWorktree)
	s.InvalidateAll(RepositoryWorktree)
	s.InvalidateAll(RepositoryWorktree)
	if len(scheduler.timers) != 3 {
		t.Fatalf("scheduled timers = %d, want 3 trailing-edge generations", len(scheduler.timers))
	}
	for _, timer := range scheduler.timers[:2] {
		if !timer.stopped {
			t.Fatal("superseded debounce timer remained active")
		}
	}

	scheduler.latest(t).fire()
	s.HandleDebounce(poster.await(t).(*repositoryDebounceTick))
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	if got := reads.Load(); got != 1 {
		t.Fatalf("status reads = %d, want 1", got)
	}
}

func TestRepositoryInFlightInvalidationPublishesOnlyOneFollowUp(t *testing.T) {
	cp := NewChangesPanel("/repo")
	publications := 0
	cp.OnRefreshed = func() { publications++ }
	s, scheduler, poster := testRepositoryState(cp, "/repo")
	started := make(chan uint64, 2)
	release := make(chan struct{}, 2)
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		started <- seq
		<-release
		path := "fresh.txt"
		if seq == 1 {
			path = "stale.txt"
		}
		return repositoryResult(seq, "/repo", "head", git.FileStatus{Status: "M", Path: path})
	}

	s.InvalidateAll(RepositoryWorktree)
	scheduler.latest(t).fire()
	s.HandleDebounce(poster.await(t).(*repositoryDebounceTick))
	first := <-started

	s.InvalidateAll(RepositoryWorktree)
	s.InvalidateAll(RepositoryWorktree)
	scheduler.latest(t).fire()
	s.HandleDebounce(poster.await(t).(*repositoryDebounceTick))
	select {
	case seq := <-started:
		t.Fatalf("overlapping status read %d started while %d was in flight", seq, first)
	default:
	}

	release <- struct{}{}
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	if publications != 0 || cp.TotalChanges() != 0 {
		t.Fatal("stale in-flight result published working-tree state")
	}
	second := <-started
	if second != s.requested || second == first {
		t.Fatalf("follow-up seq = %d, first = %d, requested = %d", second, first, s.requested)
	}
	release <- struct{}{}
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	if publications != 1 || cp.TotalChanges() != 1 || cp.groups[0].Unstaged[0].Path != "fresh.txt" {
		t.Fatalf("follow-up publish = %d, groups = %+v", publications, cp.groups)
	}
}

func TestRepositoryVisibilityPollRearmAndGenerationLifecycle(t *testing.T) {
	s, scheduler, poster := testRepositoryState(nil, "/repo")
	s.dirty = RepositoryWorktree
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		return repositoryResult(seq, "/repo", "head")
	}

	s.SetVisible(true)
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	firstPoll := scheduler.latest(t)
	if firstPoll.stopped || s.pollTimer == nil {
		t.Fatal("visible repository did not arm polling")
	}
	s.SetVisible(false)
	if !firstPoll.stopped || s.pollTimer != nil {
		t.Fatal("hiding Changes did not invalidate its queued poll")
	}
	firstPoll.fireLate()
	s.HandlePoll(poster.await(t).(*repositoryPollTick))
	if s.inFlight {
		t.Fatal("late hidden poll started a status read")
	}

	s.SetVisible(true)
	secondPoll := scheduler.latest(t)
	if secondPoll == firstPoll || secondPoll.stopped {
		t.Fatal("re-entering visible state did not rearm polling")
	}
	s.Close()
	secondPoll.fireLate()
	s.HandlePoll(poster.await(t).(*repositoryPollTick))
	if s.inFlight || !secondPoll.stopped {
		t.Fatal("late poll survived repository close")
	}
}

func TestHiddenRepositoryMutationStillRefreshesWorkingTree(t *testing.T) {
	cp := NewChangesPanel("/repo")
	s, scheduler, poster := testRepositoryState(cp, "/repo")
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		return repositoryResult(seq, "/repo", "head", git.FileStatus{Status: "M", Path: "hidden.txt"})
	}

	s.InvalidateAll(RepositoryWorktree)
	scheduler.latest(t).fire()
	s.HandleDebounce(poster.await(t).(*repositoryDebounceTick))
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	if s.visible || s.pollTimer != nil {
		t.Fatal("hidden invalidation enabled visible polling")
	}
	if cp.TotalChanges() != 1 || cp.groups[0].Unstaged[0].Path != "hidden.txt" {
		t.Fatalf("hidden mutation was not observed: %+v", cp.groups)
	}
}

func TestRepositoryStatusFailurePreservesLastGoodAndRetries(t *testing.T) {
	cp := NewChangesPanel("/repo")
	previous := changesGroup{Dir: "/repo", Name: "repo", Unstaged: []git.FileStatus{{Status: "M", Path: "kept.txt"}}}
	s, _, _ := testRepositoryState(cp, "/repo")
	s.lastGroups["/repo"] = previous
	s.lastRevisions["/repo"] = "head"
	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryWorktree

	s.HandleStatus(&RepositoryStatusResult{Seq: 1, Entries: []repositoryStatusEntry{{
		Group: changesGroup{Dir: "/repo", Name: "repo"}, Revision: "head", StatusErr: errors.New("index locked"),
	}}})
	if cp.TotalChanges() != 1 || cp.groups[0].Unstaged[0].Path != "kept.txt" {
		t.Fatalf("failed status replaced last good snapshot: %+v", cp.groups)
	}
	if s.dirty&RepositoryWorktree == 0 {
		t.Fatal("failed status was marked fresh")
	}
}

func TestRepositoryRootFailurePreservesCanonicalMultiRootOrderAndRecovers(t *testing.T) {
	rootA := filepath.Join(t.TempDir(), "repo-a")
	rootB := filepath.Join(t.TempDir(), "repo-b")
	sourceA := filepath.Join(rootA, "workspace")
	sourceB := filepath.Join(rootB, "workspace")
	goodA := changesGroup{Dir: rootA, Name: "repo-a", Unstaged: []git.FileStatus{{Status: "M", Path: "kept-a.txt"}}}
	goodB := changesGroup{Dir: rootB, Name: "repo-b", Unstaged: []git.FileStatus{{Status: "M", Path: "kept-b.txt"}}}
	cp := NewChangesPanel(sourceB, sourceA)
	s := NewRepositoryState(cp, []string{sourceB, sourceA})

	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryWorktree
	s.HandleStatus(&RepositoryStatusResult{Seq: 1, Entries: []repositoryStatusEntry{
		{SourceDir: sourceB, Group: goodB, Revision: "head-b"},
		{SourceDir: sourceA, Group: goodA, Revision: "head-a"},
	}})

	s.requested = 2
	s.inFlight = true
	s.dirty = RepositoryWorktree
	s.HandleStatus(&RepositoryStatusResult{Seq: 2, Entries: []repositoryStatusEntry{
		{SourceDir: sourceB, Group: changesGroup{Dir: sourceB, Name: "workspace"}, RootErr: errors.New("temporary root failure"), StatusErr: errors.New("temporary status failure"), RevisionErr: errors.New("temporary revision failure")},
		{SourceDir: sourceA, Group: changesGroup{Dir: sourceA, Name: "workspace"}, RootErr: errors.New("temporary root failure"), StatusErr: errors.New("temporary status failure"), RevisionErr: errors.New("temporary revision failure")},
	}})
	if len(cp.groups) != 2 || cp.groups[0].Dir != rootB || cp.groups[1].Dir != rootA {
		t.Fatalf("root failure changed canonical ordering: %+v", cp.groups)
	}
	if cp.groups[0].Unstaged[0].Path != "kept-b.txt" || cp.groups[1].Unstaged[0].Path != "kept-a.txt" {
		t.Fatalf("root failure replaced last-good groups: %+v", cp.groups)
	}
	if s.dirty&RepositoryWorktree == 0 {
		t.Fatal("root failure was marked fresh")
	}

	freshA := changesGroup{Dir: rootA, Name: "repo-a", Unstaged: []git.FileStatus{{Status: "M", Path: "fresh-a.txt"}}}
	freshB := changesGroup{Dir: rootB, Name: "repo-b", Unstaged: []git.FileStatus{{Status: "M", Path: "fresh-b.txt"}}}
	s.requested = 3
	s.inFlight = true
	s.HandleStatus(&RepositoryStatusResult{Seq: 3, Entries: []repositoryStatusEntry{
		{SourceDir: sourceB, Group: freshB, Revision: "head-b"},
		{SourceDir: sourceA, Group: freshA, Revision: "head-a"},
	}})
	if len(cp.groups) != 2 || cp.groups[0].Dir != rootB || cp.groups[0].Unstaged[0].Path != "fresh-b.txt" || cp.groups[1].Dir != rootA || cp.groups[1].Unstaged[0].Path != "fresh-a.txt" {
		t.Fatalf("successful retry did not recover canonical groups: %+v", cp.groups)
	}
	if s.dirty&RepositoryWorktree != 0 {
		t.Fatal("successful retry did not mark working tree fresh")
	}
}

func TestRepositoryRootFailureDeduplicatesCanonicalRepositoryAndRecovers(t *testing.T) {
	repo := t.TempDir()
	sourceA := filepath.Join(repo, "workspace-a")
	sourceB := filepath.Join(repo, "workspace-b")
	for _, source := range []string{sourceA, sourceB} {
		if err := os.Mkdir(source, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	previous := changesGroup{Dir: repo, Name: filepath.Base(repo), Unstaged: []git.FileStatus{{Status: "M", Path: "kept.txt"}}}
	cp := NewChangesPanel(sourceA, sourceB)
	s := NewRepositoryState(cp, []string{sourceA, sourceB})

	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryWorktree
	s.HandleStatus(&RepositoryStatusResult{Seq: 1, Entries: []repositoryStatusEntry{{SourceDir: sourceA, Group: previous, Revision: "old"}}})

	failure := func(source string) repositoryStatusEntry {
		return repositoryStatusEntry{
			SourceDir: source,
			Group:     changesGroup{Dir: source, Name: filepath.Base(source)},
			RootErr:   errors.New("root unavailable"), StatusErr: errors.New("status unavailable"), RevisionErr: errors.New("revision unavailable"),
		}
	}
	assertLastGood := func(stage string) {
		t.Helper()
		if len(cp.groups) != 1 || cp.groups[0].Dir != repo || len(cp.groups[0].Unstaged) != 1 || cp.groups[0].Unstaged[0].Path != "kept.txt" {
			t.Fatalf("%s groups = %+v, want one canonical last-good group", stage, cp.groups)
		}
		if s.dirty&RepositoryWorktree == 0 {
			t.Fatalf("%s failure was marked fresh", stage)
		}
	}

	s.requested = 2
	s.inFlight = true
	s.dirty = RepositoryWorktree
	s.HandleStatus(&RepositoryStatusResult{Seq: 2, Entries: []repositoryStatusEntry{
		{SourceDir: sourceA, Group: previous, Revision: "old"},
		failure(sourceB),
	}})
	assertLastGood("partial")

	s.requested = 3
	s.inFlight = true
	s.HandleStatus(&RepositoryStatusResult{Seq: 3, Entries: []repositoryStatusEntry{failure(sourceB), failure(sourceA)}})
	assertLastGood("all-sources")

	fresh := changesGroup{Dir: repo, Name: filepath.Base(repo), Unstaged: []git.FileStatus{{Status: "M", Path: "fresh.txt"}}}
	s.requested = 4
	s.inFlight = true
	s.HandleStatus(&RepositoryStatusResult{Seq: 4, Entries: []repositoryStatusEntry{{SourceDir: sourceB, Group: fresh, Revision: "new"}}})
	if len(cp.groups) != 1 || cp.groups[0].Dir != repo || cp.groups[0].Unstaged[0].Path != "fresh.txt" {
		t.Fatalf("recovery groups = %+v, want one fresh canonical group", cp.groups)
	}
	if s.dirty&RepositoryWorktree != 0 {
		t.Fatal("successful recovery did not mark working tree fresh")
	}
}

func TestRepositoryRootFailureKeepsDistinctNestedRepositories(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "nested")
	outerSource := filepath.Join(outer, "outer-work")
	innerSource := filepath.Join(inner, "inner-work")
	for _, dir := range []string{inner, outerSource, innerSource} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cp := NewChangesPanel(outerSource, innerSource)
	s := NewRepositoryState(cp, []string{outerSource, innerSource})
	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryWorktree
	s.HandleStatus(&RepositoryStatusResult{Seq: 1, Entries: []repositoryStatusEntry{
		{SourceDir: outerSource, Group: changesGroup{Dir: outer, Name: filepath.Base(outer)}, Revision: "outer"},
		{SourceDir: innerSource, Group: changesGroup{Dir: inner, Name: filepath.Base(inner)}, Revision: "inner"},
	}})

	s.requested = 2
	s.inFlight = true
	s.dirty = RepositoryWorktree
	s.HandleStatus(&RepositoryStatusResult{Seq: 2, Entries: []repositoryStatusEntry{
		{SourceDir: outerSource, Group: changesGroup{Dir: outerSource}, RootErr: errors.New("root"), StatusErr: errors.New("status"), RevisionErr: errors.New("revision")},
		{SourceDir: innerSource, Group: changesGroup{Dir: innerSource}, RootErr: errors.New("root"), StatusErr: errors.New("status"), RevisionErr: errors.New("revision")},
	}})
	if len(cp.groups) != 2 || cp.groups[0].Dir != outer || cp.groups[1].Dir != inner {
		t.Fatalf("nested groups merged or reordered: %+v", cp.groups)
	}
}

func TestRepositoryScanCarriesSourceIdentityAndRootFailure(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nexit 91\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	source := filepath.Join(t.TempDir(), "repo", "workspace")

	entries := scanRepositoryStatus(context.Background(), []string{source})
	if len(entries) != 1 {
		t.Fatalf("scan entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.SourceDir != source || entry.Group.Dir != source || entry.RootErr == nil || entry.StatusErr == nil || entry.RevisionErr == nil {
		t.Fatalf("failed root scan lost source/error identity: %+v", entry)
	}
}

func TestRepositoryInvalidationResolvesSymlinkedParentsWithoutExternalFalsePositives(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	external := t.TempDir()
	alias := filepath.Join(outside, "repo-alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	existing := filepath.Join(repo, "existing.txt")
	if err := os.WriteFile(existing, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path func(t *testing.T) string
		want bool
	}{
		{
			name: "missing internal target",
			path: func(t *testing.T) string { return filepath.Join(repo, "missing.txt") },
			want: true,
		},
		{
			name: "existing target through outside alias",
			path: func(t *testing.T) string { return filepath.Join(alias, "existing.txt") },
			want: true,
		},
		{
			name: "new target through outside alias",
			path: func(t *testing.T) string { return filepath.Join(alias, "new.txt") },
			want: true,
		},
		{
			name: "newly created target through outside alias",
			path: func(t *testing.T) string {
				path := filepath.Join(alias, "created.txt")
				if err := os.WriteFile(path, []byte("created\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			want: true,
		},
		{
			name: "chain of symlinked parents",
			path: func(t *testing.T) string {
				chain := filepath.Join(outside, "chain")
				if err := os.Symlink(alias, chain); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(chain, "new.txt")
			},
			want: true,
		},
		{
			name: "permission failure below resolved repository alias",
			path: func(t *testing.T) string {
				restricted := filepath.Join(repo, "restricted")
				if err := os.Mkdir(restricted, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(restricted, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(restricted, 0o700) })
				return filepath.Join(alias, "restricted", "new.txt")
			},
			want: true,
		},
		{
			name: "broken in-repository symlink",
			path: func(t *testing.T) string {
				link := filepath.Join(repo, "broken-internal")
				if err := os.Symlink(filepath.Join(external, "missing"), link); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(link, "new.txt")
			},
			want: false,
		},
		{
			name: "in-repository symlink loop",
			path: func(t *testing.T) string {
				first := filepath.Join(repo, "loop-a")
				second := filepath.Join(repo, "loop-b")
				if err := os.Symlink(second, first); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(first, second); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(first, "new.txt")
			},
			want: false,
		},
		{
			name: "external target through in-repository symlink",
			path: func(t *testing.T) string {
				link := filepath.Join(repo, "external")
				if err := os.Symlink(external, link); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(link, "new.txt")
			},
			want: false,
		},
		{
			name: "broken outside symlink",
			path: func(t *testing.T) string {
				link := filepath.Join(outside, "broken")
				if err := os.Symlink(filepath.Join(outside, "missing"), link); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(link, "new.txt")
			},
			want: false,
		},
		{
			name: "permission failure outside repository",
			path: func(t *testing.T) string {
				denied := filepath.Join(outside, "denied")
				if err := os.Mkdir(denied, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(repo, filepath.Join(denied, "hidden-alias")); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(denied, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(denied, 0o700) })
				return filepath.Join(denied, "hidden-alias", "new.txt")
			},
			want: false,
		},
		{
			name: "outside symlink loop",
			path: func(t *testing.T) string {
				first := filepath.Join(outside, "loop-a")
				second := filepath.Join(outside, "loop-b")
				if err := os.Symlink(second, first); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(first, second); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(first, "new.txt")
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, _, _ := testRepositoryState(nil, repo)
			s.InvalidatePath(test.path(t), RepositoryWorktree)
			if got := s.requested == 1 && s.dirty&RepositoryWorktree != 0; got != test.want {
				t.Fatalf("invalidated = %v, want %v (requested=%d dirty=%b)", got, test.want, s.requested, s.dirty)
			}
		})
	}
}

func TestRepositoryInvalidationUsesCurrentSymlinkTargetAfterRetarget(t *testing.T) {
	repo := t.TempDir()
	external := t.TempDir()
	alias := filepath.Join(t.TempDir(), "retargeted")
	if err := os.Symlink(repo, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	internal, _, _ := testRepositoryState(nil, repo)
	internal.InvalidatePath(filepath.Join(alias, "new.txt"), RepositoryWorktree)
	if internal.requested != 1 || internal.dirty&RepositoryWorktree == 0 {
		t.Fatalf("internal alias did not invalidate: requested=%d dirty=%b", internal.requested, internal.dirty)
	}

	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, alias); err != nil {
		t.Fatal(err)
	}
	externalState, _, _ := testRepositoryState(nil, repo)
	externalState.InvalidatePath(filepath.Join(alias, "new.txt"), RepositoryWorktree)
	if externalState.requested != 0 || externalState.dirty != 0 {
		t.Fatalf("retargeted external alias invalidated: requested=%d dirty=%b", externalState.requested, externalState.dirty)
	}
}

func TestRepositoryUnchangedHeadDoesNotReloadHistoryChangedHeadDoes(t *testing.T) {
	dir := testAppRepository(t)
	cp := NewChangesPanel(dir)
	cp.groups = []changesGroup{{Dir: dir, Name: filepath.Base(dir)}}
	cp.buildTree()
	cp.lastLogDir = dir
	cp.logDir = dir
	poster := newTestEventPoster()
	cp.Screen = poster
	s, _, _ := testRepositoryState(cp, dir)
	s.visible = true
	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryWorktree
	oldHead := git.RevisionIdentity(dir)
	s.lastGroups[dir] = cp.groups[0]
	s.lastRevisions[dir] = oldHead
	before := cp.logGen

	s.HandleStatus(repositoryResult(1, dir, oldHead))
	if cp.logGen != before {
		t.Fatalf("unchanged HEAD reloaded history: %d -> %d", before, cp.logGen)
	}

	if err := os.WriteFile(filepath.Join(dir, "next.txt"), []byte("next\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "add", "next.txt")
	testAppGit(t, dir, "commit", "-m", "next")
	s.requested = 2
	s.inFlight = true
	s.dirty = RepositoryWorktree
	s.HandleStatus(repositoryResult(2, dir, git.RevisionIdentity(dir)))
	if cp.logGen != before+1 || s.dirty&RepositoryHistory == 0 {
		t.Fatalf("changed HEAD history gen = %d, dirty = %b", cp.logGen, s.dirty)
	}
	cp.CancelHistoryRead()
}

func TestRepositoryRootReplacementCancelsAndSupersedesOldResult(t *testing.T) {
	cp := NewChangesPanel("/old")
	s, _, poster := testRepositoryState(cp, "/old")
	firstCanceled := make(chan struct{})
	secondStarted := make(chan struct{})
	s.statusWait = time.Hour
	s.readStatus = func(ctx context.Context, dirs []string, seq uint64) *RepositoryStatusResult {
		if dirs[0] == "/old" {
			<-ctx.Done()
			close(firstCanceled)
			return repositoryResult(seq, "/old", "old", git.FileStatus{Status: "M", Path: "old.txt"})
		}
		close(secondStarted)
		return repositoryResult(seq, "/new", "new", git.FileStatus{Status: "M", Path: "new.txt"})
	}

	s.RefreshNow(RepositoryWorktree)
	s.SetDirs([]string{"/new"})
	select {
	case <-firstCanceled:
	case <-time.After(time.Second):
		t.Fatal("root replacement did not cancel the old status context")
	}
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("replacement root status did not start")
	}
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	if len(cp.groups) != 1 || cp.groups[0].Dir != "/new" || cp.groups[0].Unstaged[0].Path != "new.txt" {
		t.Fatalf("old root result survived replacement: %+v", cp.groups)
	}
}

func TestRepositoryCloseCancelsStatusAndLateDebounce(t *testing.T) {
	s, _, _ := testRepositoryState(nil, "/repo")
	started := make(chan struct{})
	canceled := make(chan struct{})
	s.statusWait = time.Hour
	s.readStatus = func(ctx context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		close(started)
		<-ctx.Done()
		close(canceled)
		return repositoryResult(seq, "/repo", "")
	}
	s.RefreshNow(RepositoryWorktree)
	<-started
	s.Close()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel the in-flight status read")
	}

	s2, scheduler2, poster2 := testRepositoryState(nil, "/repo")
	var reads atomic.Int32
	s2.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		reads.Add(1)
		return repositoryResult(seq, "/repo", "head")
	}
	s2.InvalidateAll(RepositoryWorktree)
	debounce := scheduler2.latest(t)
	s2.Close()
	debounce.fireLate()
	s2.HandleDebounce(poster2.await(t).(*repositoryDebounceTick))
	if reads.Load() != 0 || !debounce.stopped {
		t.Fatal("late debounce survived repository close")
	}
}

func TestCommitHistoryRefreshKeyRoutesThroughCoordinator(t *testing.T) {
	cp := NewChangesPanel("/repo")
	called := 0
	cp.OnRefresh = func() { called++ }
	if !cp.handleCommitLogKey(tcell.NewEventKey(tcell.KeyRune, "r", tcell.ModNone), nil) {
		t.Fatal("commit-history refresh key was not handled")
	}
	if called != 1 {
		t.Fatalf("coordinator refresh callback calls = %d, want 1", called)
	}
}

func TestManualRepositoryRefreshForcesWorktreeAndSelectedHistory(t *testing.T) {
	dir := testAppRepository(t)
	cp := NewChangesPanel(dir)
	cp.groups = []changesGroup{{Dir: dir, Name: filepath.Base(dir)}}
	cp.buildTree()
	cp.lastLogDir = dir
	cp.logDir = dir
	cp.Screen = newTestEventPoster()
	s, _, _ := testRepositoryState(cp, dir)
	s.visible = true
	s.readStatus = func(ctx context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		<-ctx.Done()
		return repositoryResult(seq, dir, "")
	}
	before := cp.logGen
	s.RefreshNow(RepositoryWorktree | RepositoryHistory)
	if !s.inFlight || s.requested != 1 {
		t.Fatalf("manual refresh status inFlight=%v requested=%d", s.inFlight, s.requested)
	}
	if cp.logGen != before+1 {
		t.Fatalf("manual refresh history gen = %d, want %d", cp.logGen, before+1)
	}
	s.Close()
}

func TestCommitLogFailurePreservesLastGoodAndRemainsRetryable(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.lastLogDir = "/repo"
	cp.logDir = "/repo"
	cp.logGen = 4
	good := &widgets.TreeNode{ID: "commit:good", Label: "last good commit"}
	cp.CommitLog.SetItems([]*widgets.TreeNode{good})
	s := NewRepositoryState(cp, []string{"/repo"})
	var callbackErr error
	cp.OnHistoryResult = func(err error) {
		callbackErr = err
		s.HandleHistory(err)
	}

	cp.ApplyCommitLog(&CommitLogResult{Gen: 4, Dir: "/repo", Err: errors.New("temporary log failure")})
	if len(cp.CommitLog.Config.Items) != 1 || cp.CommitLog.Config.Items[0] != good {
		t.Fatalf("history failure replaced last good items: %+v", cp.CommitLog.Config.Items)
	}
	if cp.lastLogDir != "" || callbackErr == nil {
		t.Fatalf("failed history retry guard = %q, callback err = %v", cp.lastLogDir, callbackErr)
	}
	if s.dirty&RepositoryHistory == 0 {
		t.Fatal("failed history read was marked fresh")
	}
}

func TestDebugScreenshotAndStateDumpEachInvalidateTheirPathOnce(t *testing.T) {
	dir := t.TempDir()
	cfg := config.AppConfig{
		Keybindings: config.DefaultKeybindings(),
		Settings:    config.DefaultSettings(),
		Theme:       config.DefaultTheme(),
	}
	borders := BuildBorderSet(cfg.Theme.Borders)
	a := BuildAppFromConfig(&cfg, &borders, workspace.New([]string{dir}), nil)
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	sim.SetSize(80, 24)
	a.Screen = term.NewTcellScreenFrom(sim)
	a.Root.SetSize(80, 24)
	a.Repository.poster = newTestEventPoster()
	a.Repository.scheduler = &fakeRepositoryScheduler{}

	paths := []string{filepath.Join(dir, "screen.txt"), filepath.Join(dir, "state.json")}
	if err := a.DumpScreenshot(paths[0]); err != nil {
		t.Fatal(err)
	}
	if err := a.DumpDebugState(paths[1]); err != nil {
		t.Fatal(err)
	}
	if a.Repository.requested != 0 {
		t.Fatal("debug writer mutated repository coordinator off the event loop")
	}
	for _, path := range paths {
		var interrupt *tcell.EventInterrupt
		for interrupt == nil {
			interrupt, _ = a.Screen.PollEvent().(*tcell.EventInterrupt)
		}
		request, ok := interrupt.Data().(*RepositoryInvalidationRequest)
		if !ok || request.Path != path || request.Resources != RepositoryWorktree {
			t.Fatalf("debug invalidation = %#v, want path %q worktree", interrupt.Data(), path)
		}
		a.handleRepositoryInvalidation(request)
	}
	if a.Repository.requested != 2 {
		t.Fatalf("debug write invalidations = %d, want exactly 2", a.Repository.requested)
	}
	a.Repository.Close()
}

func testAppRepository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testAppGit(t, dir, "init", "-q", "-b", "main")
	testAppGit(t, dir, "config", "user.email", "test@test.com")
	testAppGit(t, dir, "config", "user.name", "Test User")
	testAppGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "add", "initial.txt")
	testAppGit(t, dir, "commit", "-m", "initial")
	return dir
}

func testAppGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
