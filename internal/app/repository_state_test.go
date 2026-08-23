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
	s.activeFile = "/repo/file.txt"
	var statusReads atomic.Int32
	var identityReads atomic.Int32
	s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
		statusReads.Add(1)
		return repositoryResult(seq, "/repo", "head")
	}
	s.readIdentity = func(_ context.Context, path string, seq uint64) *RepositoryIdentityResult {
		identityReads.Add(1)
		return &RepositoryIdentityResult{
			Seq:      seq,
			FilePath: path,
			Identity: git.RepositoryIdentity{Root: "/repo", Branch: "latest"},
		}
	}

	var statusTimers, identityTimers []*fakeRepositoryTimer
	for range 3 {
		s.InvalidateAll(RepositoryWorktree)
		statusTimers = append(statusTimers, s.debounceTimer.(*fakeRepositoryTimer))
		identityTimers = append(identityTimers, s.identityDebounceTimer.(*fakeRepositoryTimer))
	}
	if len(scheduler.timers) != 6 {
		t.Fatalf("scheduled timers = %d, want 3 trailing-edge generations for status and identity", len(scheduler.timers))
	}
	for i := range 2 {
		if !statusTimers[i].stopped || !identityTimers[i].stopped {
			t.Fatal("superseded debounce generation remained active")
		}
	}
	if statusReads.Load() != 0 || identityReads.Load() != 0 || s.identitySeq != 0 {
		t.Fatalf("repository burst refreshed before the trailing edge: status=%d identity=%d seq=%d", statusReads.Load(), identityReads.Load(), s.identitySeq)
	}
	for _, timer := range identityTimers[:2] {
		timer.fireLate()
		s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
	}
	if identityReads.Load() != 0 || s.identitySeq != 0 {
		t.Fatalf("obsolete identity ticks refreshed repository: reads=%d seq=%d", identityReads.Load(), s.identitySeq)
	}

	identityTimers[2].fire()
	if identityReads.Load() != 0 || s.identitySeq != 0 {
		t.Fatal("identity debounce callback read repository off the main thread")
	}
	s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
	s.HandleIdentity(poster.await(t).(*RepositoryIdentityResult))
	statusTimers[2].fire()
	s.HandleDebounce(poster.await(t).(*repositoryDebounceTick))
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	if got := statusReads.Load(); got != 1 {
		t.Fatalf("status reads = %d, want 1", got)
	}
	if got := identityReads.Load(); got != 1 {
		t.Fatalf("identity reads = %d, want 1", got)
	}
	if s.identitySeq != 1 {
		t.Fatalf("identity generation = %d, want 1 effective refresh", s.identitySeq)
	}
	if root, branch := s.ActiveRepository(); root != "/repo" || branch != "latest" {
		t.Fatalf("active identity = (%q, %q), want final repository state", root, branch)
	}
}

func TestRepositoryIdentityBurstDuringInFlightReadWaitsForTrailingEdgeAndDropsStaleResult(t *testing.T) {
	s, _, poster := testRepositoryState(nil, "/repo")
	s.activeFile = "/repo/file.txt"
	s.activeRoot = "/repo"
	s.activeBranch = "last-good"
	s.statusWait = time.Hour
	firstStarted := make(chan struct{})
	firstCanceled := make(chan struct{})
	var reads atomic.Int32
	s.readIdentity = func(ctx context.Context, path string, seq uint64) *RepositoryIdentityResult {
		if reads.Add(1) == 1 {
			close(firstStarted)
			<-ctx.Done()
			close(firstCanceled)
			return &RepositoryIdentityResult{
				Seq:      seq,
				FilePath: path,
				Identity: git.RepositoryIdentity{Root: "/repo", Branch: "stale"},
			}
		}
		return &RepositoryIdentityResult{
			Seq:      seq,
			FilePath: path,
			Identity: git.RepositoryIdentity{Root: "/repo", Branch: "current"},
		}
	}

	s.InvalidateAll(RepositoryWorktree)
	firstTimer := s.identityDebounceTimer.(*fakeRepositoryTimer)
	firstTimer.fire()
	s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
	<-firstStarted

	var burstTimers []*fakeRepositoryTimer
	for range 3 {
		s.InvalidateAll(RepositoryWorktree)
		burstTimers = append(burstTimers, s.identityDebounceTimer.(*fakeRepositoryTimer))
	}
	for _, timer := range burstTimers[:2] {
		if !timer.stopped {
			t.Fatal("superseded in-flight burst timer remained active")
		}
	}
	select {
	case <-firstCanceled:
		t.Fatal("watcher burst canceled the in-flight identity read before its trailing edge")
	default:
	}
	if got := reads.Load(); got != 1 {
		t.Fatalf("identity reads before trailing edge = %d, want 1", got)
	}

	burstTimers[2].fire()
	s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
	select {
	case <-firstCanceled:
	case <-time.After(time.Second):
		t.Fatal("trailing-edge refresh did not cancel the superseded identity read")
	}
	for range 2 {
		s.HandleIdentity(poster.await(t).(*RepositoryIdentityResult))
	}
	if got := reads.Load(); got != 2 {
		t.Fatalf("identity reads across two debounce windows = %d, want 2", got)
	}
	if root, branch := s.ActiveRepository(); root != "/repo" || branch != "current" {
		t.Fatalf("active identity = (%q, %q), want final current result", root, branch)
	}
	s.Close()
}

func TestActiveRepositoryIdentityIsCachedPerFileAndClearsOutsideGit(t *testing.T) {
	repo := t.TempDir()
	nested := filepath.Join(repo, "nested", "file.txt")
	plain := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{nested, plain} {
		if err := os.WriteFile(path, []byte("content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := NewRepositoryState(nil, nil)
	reads := 0
	branch := "feature"
	fail := false
	s.readIdentity = func(_ context.Context, path string, seq uint64) *RepositoryIdentityResult {
		reads++
		result := &RepositoryIdentityResult{Seq: seq, FilePath: path}
		if path == nested && !fail {
			result.Identity = git.RepositoryIdentity{Root: repo, Branch: branch}
		} else {
			result.Err = errors.New("not a repository")
		}
		return result
	}

	s.SetActiveFile(nested)
	for range 10 {
		s.SetActiveFile(nested)
	}
	if root, branch := s.ActiveRepository(); root != repo || branch != "feature" || reads != 1 {
		t.Fatalf("cached active identity = (%q, %q), reads=%d", root, branch, reads)
	}

	fail = true
	s.refreshActiveIdentity()
	if root, branch := s.ActiveRepository(); root != repo || branch != "feature" || reads != 2 {
		t.Fatalf("failed refresh replaced active identity = (%q, %q), reads=%d", root, branch, reads)
	}
	fail = false
	branch = "updated"
	s.refreshActiveIdentity()
	if root, activeBranch := s.ActiveRepository(); root != repo || activeBranch != "updated" || reads != 3 {
		t.Fatalf("successful refresh identity = (%q, %q), reads=%d", root, activeBranch, reads)
	}

	s.SetActiveFile(plain)
	if root, branch := s.ActiveRepository(); root != "" || branch != "" || reads != 4 {
		t.Fatalf("plain active identity = (%q, %q), reads=%d, want cleared", root, branch, reads)
	}
	s.SetActiveFile("")
	if root, branch := s.ActiveRepository(); root != "" || branch != "" {
		t.Fatalf("virtual active identity = (%q, %q), want cleared", root, branch)
	}
}

func TestReadRepositoryIdentityUsesStableFileSymlinkIdentity(t *testing.T) {
	repo := canonicalTestPath(t, testAppRepository(t))
	target := filepath.Join(repo, "nested", "file.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	external := canonicalTestPath(t, t.TempDir())
	link := filepath.Join(external, "file-link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	result := readRepositoryIdentity(context.Background(), link, 7)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Seq != 7 || result.FilePath != link {
		t.Fatalf("result identity = seq %d path %q, want seq 7 presentation path %q", result.Seq, result.FilePath, link)
	}
	if result.Identity.Root != repo || result.Identity.Branch != "main" {
		t.Fatalf("repository identity = %+v, want root %q branch main", result.Identity, repo)
	}

	missing := filepath.Join(repo, "missing", "deeper", "file.txt")
	result = readRepositoryIdentity(context.Background(), missing, 8)
	if result.Err != nil || result.FilePath != missing || result.Identity.Root != repo || result.Identity.Branch != "main" {
		t.Fatalf("missing-file repository identity = %+v, want presentation path %q root %q branch main", result, missing, repo)
	}
}

func TestGitRelativePathEnablesBlameForExternalFileSymlink(t *testing.T) {
	repo := canonicalTestPath(t, testAppRepository(t))
	target := filepath.Join(repo, "nested", "blamed.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("blamed line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, repo, "add", "nested/blamed.txt")
	testAppGit(t, repo, "commit", "-m", "add blamed file")

	external := canonicalTestPath(t, t.TempDir())
	link := filepath.Join(external, "blamed-link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if info := git.BlameLine(repo, link, 1); info != nil {
		t.Fatalf("raw external blame = %+v, want nil", info)
	}

	state := NewRepositoryState(nil, nil)
	blamePath, ok := state.gitRelativePath(repo, link)
	if !ok || blamePath != "nested/blamed.txt" {
		t.Fatalf("canonical blame path = (%q, %t), want nested/blamed.txt", blamePath, ok)
	}
	info := git.BlameLine(repo, blamePath, 1)
	if info == nil || info.Author != "Test User" {
		t.Fatalf("canonical blame = %+v, want Test User", info)
	}

	outsideTarget := filepath.Join(external, "outside.txt")
	if err := os.WriteFile(outsideTarget, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsideLink := filepath.Join(canonicalTestPath(t, t.TempDir()), "outside-link.txt")
	if err := os.Symlink(outsideTarget, outsideLink); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if path, ok := state.gitRelativePath(repo, outsideLink); ok {
		t.Fatalf("outside blame path = %q, want containment rejection", path)
	}
}

func TestRepositoryDiscoveryDirectoryHandlesSymlinksMissingPathsAndInvalidLinks(t *testing.T) {
	repo := canonicalTestPath(t, t.TempDir())
	nested := filepath.Join(repo, "nested")
	external := canonicalTestPath(t, t.TempDir())
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(nested, "file.txt")
	if err := os.WriteFile(target, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileLink := filepath.Join(external, "file-link.txt")
	dirLink := filepath.Join(external, "dir-link")
	broken := filepath.Join(external, "broken")
	cycleA := filepath.Join(external, "cycle-a")
	cycleB := filepath.Join(external, "cycle-b")
	links := [][2]string{
		{target, fileLink},
		{nested, dirLink},
		{filepath.Join(external, "missing-target"), broken},
		{cycleB, cycleA},
		{cycleA, cycleB},
	}
	for _, link := range links {
		if err := os.Symlink(link[0], link[1]); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	tests := []struct {
		name         string
		path         string
		wantDir      string
		wantErr      bool
		wantNotExist bool
	}{
		{name: "file symlink", path: fileLink, wantDir: nested},
		{name: "directory symlink", path: filepath.Join(dirLink, "file.txt"), wantDir: nested},
		{name: "missing file", path: filepath.Join(nested, "missing.txt"), wantDir: nested},
		{name: "deep missing path", path: filepath.Join(dirLink, "missing", "deeper", "file.txt"), wantDir: nested},
		{name: "broken file symlink", path: broken, wantErr: true, wantNotExist: true},
		{name: "broken directory symlink", path: filepath.Join(broken, "file.txt"), wantErr: true, wantNotExist: true},
		{name: "file symlink cycle", path: cycleA, wantErr: true},
		{name: "directory symlink cycle", path: filepath.Join(cycleA, "file.txt"), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir, err := repositoryDiscoveryDirectory(test.path)
			if test.wantErr {
				if err == nil {
					t.Fatalf("discovery directory = %q, want error", dir)
				}
				if errors.Is(err, errRepositoryPathIdentityUnstable) {
					t.Fatalf("permanent path error misclassified as transient instability: %v", err)
				}
				if test.wantNotExist && !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("broken link error = %v, want os.ErrNotExist", err)
				}
				return
			}
			if err != nil || dir != test.wantDir {
				t.Fatalf("discovery directory = (%q, %v), want %q", dir, err, test.wantDir)
			}
		})
	}
}

func TestRepositoryForPathUsesStableFileIdentityAndLongestNestedRoot(t *testing.T) {
	outer := canonicalTestPath(t, t.TempDir())
	inner := filepath.Join(outer, "nested")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(inner, "file.txt")
	if err := os.WriteFile(target, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "file-link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	s := NewRepositoryState(nil, nil)
	s.identities[outer] = git.RepositoryIdentity{Root: outer, Branch: "outer"}
	s.identities[inner] = git.RepositoryIdentity{Root: inner, Branch: "inner"}
	root, branch := s.RepositoryForPath(link)
	if root != inner || branch != "inner" {
		t.Fatalf("linked nested identity = (%q, %q), want (%q, inner)", root, branch, inner)
	}
}

func TestActiveRepositoryIdentityCancelsSupersededReadAndDropsStaleGeneration(t *testing.T) {
	first := filepath.Join(t.TempDir(), "first.txt")
	second := filepath.Join(t.TempDir(), "second.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s, _, poster := testRepositoryState(nil, "")
	s.statusWait = time.Hour
	firstStarted := make(chan struct{})
	firstCanceled := make(chan struct{})
	s.readIdentity = func(ctx context.Context, path string, seq uint64) *RepositoryIdentityResult {
		if path == first {
			close(firstStarted)
			<-ctx.Done()
			close(firstCanceled)
			return &RepositoryIdentityResult{Seq: seq, FilePath: path, Err: ctx.Err()}
		}
		return &RepositoryIdentityResult{
			Seq:      seq,
			FilePath: path,
			Identity: git.RepositoryIdentity{Root: filepath.Dir(path), Branch: "second"},
		}
	}

	s.SetActiveFile(first)
	<-firstStarted
	firstSeq := s.identitySeq
	s.SetActiveFile(second)
	select {
	case <-firstCanceled:
	case <-time.After(time.Second):
		t.Fatal("superseded identity read was not canceled")
	}
	for range 2 {
		data := poster.await(t)
		result, ok := data.(*RepositoryIdentityResult)
		if !ok {
			t.Fatalf("identity event type = %T", data)
		}
		s.HandleIdentity(result)
	}
	if root, branch := s.ActiveRepository(); root != filepath.Dir(second) || branch != "second" {
		t.Fatalf("active identity = (%q, %q), want second file identity", root, branch)
	}

	s.HandleIdentity(&RepositoryIdentityResult{
		Seq:      firstSeq,
		FilePath: first,
		Identity: git.RepositoryIdentity{Root: filepath.Dir(first), Branch: "stale"},
	})
	if root, branch := s.ActiveRepository(); root != filepath.Dir(second) || branch != "second" {
		t.Fatalf("stale generation replaced active identity = (%q, %q)", root, branch)
	}
	s.Close()
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

func TestRepositoryImmediateRefreshSupersedesWatcherIdentityDebounce(t *testing.T) {
	tests := []struct {
		name    string
		trigger func(*RepositoryState)
	}{
		{name: "refresh now", trigger: func(s *RepositoryState) { s.RefreshNow(RepositoryWorktree) }},
		{name: "poll", trigger: func(s *RepositoryState) {
			s.visible = true
			s.pollGen = 1
			s.pollTimer = &fakeRepositoryTimer{}
			s.HandlePoll(&repositoryPollTick{Gen: 1})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, _, poster := testRepositoryState(nil, "/repo")
			s.activeFile = "/repo/file.txt"
			var identityReads atomic.Int32
			s.readIdentity = func(_ context.Context, path string, seq uint64) *RepositoryIdentityResult {
				identityReads.Add(1)
				return &RepositoryIdentityResult{
					Seq:      seq,
					FilePath: path,
					Identity: git.RepositoryIdentity{Root: "/repo", Branch: "immediate"},
				}
			}
			s.readStatus = func(_ context.Context, _ []string, seq uint64) *RepositoryStatusResult {
				return repositoryResult(seq, "/repo", "head")
			}

			s.InvalidateAll(RepositoryWorktree)
			identityDebounce := s.identityDebounceTimer.(*fakeRepositoryTimer)
			statusDebounce := s.debounceTimer.(*fakeRepositoryTimer)
			test.trigger(s)
			if !identityDebounce.stopped || !statusDebounce.stopped {
				t.Fatal("immediate refresh did not supersede pending watcher debounces")
			}
			for range 2 {
				switch result := poster.await(t).(type) {
				case *RepositoryIdentityResult:
					s.HandleIdentity(result)
				case *RepositoryStatusResult:
					s.HandleStatus(result)
				default:
					t.Fatalf("immediate event type = %T", result)
				}
			}
			if got := identityReads.Load(); got != 1 {
				t.Fatalf("immediate identity reads = %d, want 1", got)
			}
			if root, branch := s.ActiveRepository(); root != "/repo" || branch != "immediate" {
				t.Fatalf("active identity = (%q, %q)", root, branch)
			}

			identityDebounce.fireLate()
			s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
			if got := identityReads.Load(); got != 1 {
				t.Fatalf("late watcher debounce started identity read after immediate refresh: %d", got)
			}
			s.Close()
		})
	}
}

func TestRepositoryActiveFileSwitchSupersedesWatcherDebounceAndClearsImmediately(t *testing.T) {
	s, _, poster := testRepositoryState(nil, "/repo")
	s.activeFile = "/repo/file.txt"
	s.activeRoot = "/repo"
	s.activeBranch = "last-good"
	var identityReads atomic.Int32
	s.readIdentity = func(_ context.Context, path string, seq uint64) *RepositoryIdentityResult {
		identityReads.Add(1)
		return &RepositoryIdentityResult{Seq: seq, FilePath: path, Err: errors.New("not a repository")}
	}

	s.InvalidateAll(RepositoryWorktree)
	transientTimer := s.identityDebounceTimer.(*fakeRepositoryTimer)
	transientTimer.fire()
	s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
	s.HandleIdentity(poster.await(t).(*RepositoryIdentityResult))
	if root, branch := s.ActiveRepository(); root != "/repo" || branch != "last-good" || identityReads.Load() != 1 {
		t.Fatalf("transient refresh identity = (%q, %q), reads=%d", root, branch, identityReads.Load())
	}

	s.InvalidateAll(RepositoryWorktree)
	fileTimer := s.identityDebounceTimer.(*fakeRepositoryTimer)
	s.SetActiveFile("/plain/file.txt")
	if !fileTimer.stopped {
		t.Fatal("active-file switch did not stop pending identity debounce")
	}
	if root, branch := s.ActiveRepository(); root != "" || branch != "" {
		t.Fatalf("non-repository switch did not clear immediately: (%q, %q)", root, branch)
	}
	s.HandleIdentity(poster.await(t).(*RepositoryIdentityResult))
	fileTimer.fireLate()
	s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
	if got := identityReads.Load(); got != 2 {
		t.Fatalf("late file-switch tick reread identity: reads=%d", got)
	}

	s.InvalidateAll(RepositoryWorktree)
	virtualTimer := s.identityDebounceTimer.(*fakeRepositoryTimer)
	s.SetActiveFile("")
	if !virtualTimer.stopped {
		t.Fatal("virtual-file switch did not stop pending identity debounce")
	}
	virtualTimer.fireLate()
	s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
	if root, branch := s.ActiveRepository(); root != "" || branch != "" || identityReads.Load() != 2 {
		t.Fatalf("virtual active identity = (%q, %q), reads=%d", root, branch, identityReads.Load())
	}
	s.Close()
}

func TestRepositorySetDirsRearmsPendingIdentityDebounce(t *testing.T) {
	s, _, poster := testRepositoryState(nil, "/old")
	s.activeFile = "/old/file.txt"
	statusStarted := make(chan struct{})
	releaseStatus := make(chan struct{})
	s.readStatus = func(_ context.Context, dirs []string, seq uint64) *RepositoryStatusResult {
		close(statusStarted)
		<-releaseStatus
		return repositoryResult(seq, dirs[0], "head")
	}
	var identityReads atomic.Int32
	s.readIdentity = func(_ context.Context, path string, seq uint64) *RepositoryIdentityResult {
		identityReads.Add(1)
		return &RepositoryIdentityResult{
			Seq:      seq,
			FilePath: path,
			Identity: git.RepositoryIdentity{Root: "/old", Branch: "current"},
		}
	}

	s.InvalidateAll(RepositoryWorktree)
	oldTimer := s.identityDebounceTimer.(*fakeRepositoryTimer)
	s.SetDirs([]string{"/new"})
	<-statusStarted
	newTimer := s.identityDebounceTimer.(*fakeRepositoryTimer)
	if oldTimer == newTimer || !oldTimer.stopped || newTimer.stopped {
		t.Fatal("SetDirs did not generation-rearm pending identity work")
	}

	oldTimer.fireLate()
	s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
	if identityReads.Load() != 0 || s.identitySeq != 0 {
		t.Fatal("superseded pre-SetDirs identity tick started a read")
	}
	newTimer.fire()
	if identityReads.Load() != 0 || s.identitySeq != 0 {
		t.Fatal("rearmed SetDirs timer read identity off the main thread")
	}
	s.HandleIdentityDebounce(poster.await(t).(*repositoryIdentityDebounceTick))
	s.HandleIdentity(poster.await(t).(*RepositoryIdentityResult))
	if identityReads.Load() != 1 || s.identitySeq != 1 {
		t.Fatalf("rearmed identity reads=%d seq=%d, want 1/1", identityReads.Load(), s.identitySeq)
	}

	close(releaseStatus)
	s.HandleStatus(poster.await(t).(*RepositoryStatusResult))
	s.Close()
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
	repo := canonicalTestPath(t, t.TempDir())
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

func TestResolveRepositoryPathIdentityBoundsPersistentChurn(t *testing.T) {
	if os.Getenv("TTT_TEST_REPOSITORY_PATH_CHURN") == "1" {
		testResolveRepositoryPathIdentityPersistentChurn(t)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestResolveRepositoryPathIdentityBoundsPersistentChurn$")
	cmd.Env = append(os.Environ(), "TTT_TEST_REPOSITORY_PATH_CHURN=1")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("persistent churn resolution did not terminate within 3s: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("persistent churn subprocess failed: %v\n%s", err, output)
	}
}

func testResolveRepositoryPathIdentityPersistentChurn(t *testing.T) {
	base := filepath.Join(t.TempDir(), "repo")
	ancestor := filepath.Join(base, "missing")
	target := filepath.Join(ancestor, "file.txt")
	stableTarget := filepath.Join(base, "canonical.txt")
	churning := true
	succeedOnAttempt := 0
	stableAttempt := false
	attempts := 0
	targetRestarts := 0
	ancestorRestarts := 0
	targetLstatCalls := 0
	ancestorLstatCalls := 0
	notExist := func(op, path string) error {
		return &os.PathError{Op: op, Path: path, Err: os.ErrNotExist}
	}

	evalSymlinks := func(path string) (string, error) {
		if !churning && path == target {
			return stableTarget, nil
		}
		switch path {
		case target:
			if stableAttempt {
				return stableTarget, nil
			}
			attempts++
			targetLstatCalls = 0
			ancestorLstatCalls = 0
			if attempts == succeedOnAttempt {
				stableAttempt = true
				return stableTarget, nil
			}
			return "", notExist("evalsymlinks", path)
		case ancestor:
			if attempts%2 == 0 {
				return "", notExist("evalsymlinks", path)
			}
			return path, nil
		case base:
			return path, nil
		default:
			t.Fatalf("unexpected EvalSymlinks path %q", path)
			return "", nil
		}
	}
	lstat := func(path string) (os.FileInfo, error) {
		switch path {
		case target:
			targetLstatCalls++
			if attempts%2 == 1 && targetLstatCalls == 2 {
				targetRestarts++
				return nil, nil
			}
			return nil, notExist("lstat", path)
		case ancestor:
			ancestorLstatCalls++
			if attempts%2 == 0 && ancestorLstatCalls == 2 {
				ancestorRestarts++
				return nil, nil
			}
			return nil, notExist("lstat", path)
		default:
			t.Fatalf("unexpected lstat path %q", path)
			return nil, nil
		}
	}

	resolved, err := resolveRepositoryPathIdentityWith(target, lstat, evalSymlinks)
	wantErr := "repository path identity is transiently unstable after 8 attempts for " + target
	if resolved != "" || err == nil || err.Error() != wantErr || !errors.Is(err, errRepositoryPathIdentityUnstable) {
		t.Fatalf("persistent churn resolved=%q err=%v, want classified error %q", resolved, err, wantErr)
	}
	if attempts != repositoryPathIdentityMaxAttempts || targetRestarts != 4 || ancestorRestarts != 4 {
		t.Fatalf("persistent churn attempts=%d target restarts=%d ancestor restarts=%d, want 8, 4, 4", attempts, targetRestarts, ancestorRestarts)
	}

	churning = false
	resolved, err = resolveRepositoryPathIdentityWith(target, lstat, evalSymlinks)
	if err != nil || resolved != stableTarget {
		t.Fatalf("later stable retry resolved=%q err=%v, want %q", resolved, err, stableTarget)
	}

	churning = true
	succeedOnAttempt = repositoryPathIdentityMaxAttempts
	stableAttempt = false
	attempts = 0
	targetRestarts = 0
	ancestorRestarts = 0
	resolved, err = resolveRepositoryPathIdentityWith(target, lstat, evalSymlinks)
	if err != nil || resolved != stableTarget {
		t.Fatalf("last-attempt recovery resolved=%q err=%v, want %q", resolved, err, stableTarget)
	}
	if attempts != repositoryPathIdentityMaxAttempts || targetRestarts != 4 || ancestorRestarts != 3 {
		t.Fatalf("last-attempt recovery attempts=%d target restarts=%d ancestor restarts=%d, want 8, 4, 3", attempts, targetRestarts, ancestorRestarts)
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

func TestRepositoryInvalidationRejectsUnprovablePermissionAncestors(t *testing.T) {
	repo := canonicalTestPath(t, t.TempDir())
	external := canonicalTestPath(t, t.TempDir())
	restricted := filepath.Join(repo, "restricted")
	plain := filepath.Join(repo, "plain")
	for _, dir := range []string{restricted, plain} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(external, filepath.Join(restricted, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Join(restricted, "escape")); err != nil || resolved != external {
		t.Fatalf("external symlink parent identity=%q err=%v, want %q", resolved, err, external)
	}
	for _, dir := range []string{restricted, plain} {
		if err := os.Chmod(dir, 0); err != nil {
			t.Fatal(err)
		}
		dir := dir
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	}

	tests := []struct {
		name   string
		denied string
		path   string
	}{
		{
			name:   "permission-hidden external symlink",
			denied: restricted,
			path:   filepath.Join(restricted, "escape", "missing.txt"),
		},
		{
			name:   "plain permission-denied internal directory",
			denied: plain,
			path:   filepath.Join(plain, "missing.txt"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, _, _ := testRepositoryState(nil, repo)
			s.pathIdentity = permissionDeniedRepositoryPathResolver(test.denied)
			resolved, err := s.resolvePathIdentity(test.path)
			if resolved != "" || !errors.Is(err, os.ErrPermission) {
				t.Fatalf("permission seam resolved=%q err=%v, want os.ErrPermission", resolved, err)
			}
			if errors.Is(err, errRepositoryPathIdentityUnstable) {
				t.Fatalf("permission error misclassified as transient instability: %v", err)
			}
			s.InvalidatePath(test.path, RepositoryWorktree)
			if s.requested != 0 || s.dirty != 0 {
				t.Fatalf("unprovable path invalidated: requested=%d dirty=%b", s.requested, s.dirty)
			}
		})
	}
}

func permissionDeniedRepositoryPathResolver(denied string) func(string) (string, error) {
	permissionError := func(op, path string) error {
		return &os.PathError{Op: op, Path: path, Err: os.ErrPermission}
	}
	deniedDescendant := func(path string) bool {
		return filepath.Clean(path) != filepath.Clean(denied) && pathWithin(denied, path)
	}
	lstat := func(path string) (os.FileInfo, error) {
		if deniedDescendant(path) {
			return nil, permissionError("lstat", path)
		}
		return os.Lstat(path)
	}
	evalSymlinks := func(path string) (string, error) {
		if deniedDescendant(path) {
			return "", permissionError("evalsymlinks", path)
		}
		return filepath.EvalSymlinks(path)
	}
	return func(path string) (string, error) {
		return resolveRepositoryPathIdentityWith(path, lstat, evalSymlinks)
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

func TestRepositoryCloseCancelsIdentityDebounceAndInFlightRead(t *testing.T) {
	pending, _, pendingPoster := testRepositoryState(nil, "/repo")
	pending.activeFile = "/repo/file.txt"
	var pendingReads atomic.Int32
	pending.readIdentity = func(_ context.Context, path string, seq uint64) *RepositoryIdentityResult {
		pendingReads.Add(1)
		return &RepositoryIdentityResult{Seq: seq, FilePath: path}
	}
	pending.InvalidateAll(RepositoryWorktree)
	pendingTimer := pending.identityDebounceTimer.(*fakeRepositoryTimer)
	pending.Close()
	pendingTimer.fireLate()
	pending.HandleIdentityDebounce(pendingPoster.await(t).(*repositoryIdentityDebounceTick))
	if !pendingTimer.stopped || pendingReads.Load() != 0 {
		t.Fatal("late identity debounce survived repository close")
	}

	inFlight, _, inFlightPoster := testRepositoryState(nil, "/repo")
	inFlight.activeFile = "/repo/file.txt"
	inFlight.activeRoot = "/repo"
	inFlight.activeBranch = "last-good"
	inFlight.statusWait = time.Hour
	started := make(chan struct{})
	canceled := make(chan struct{})
	inFlight.readIdentity = func(ctx context.Context, path string, seq uint64) *RepositoryIdentityResult {
		close(started)
		<-ctx.Done()
		close(canceled)
		return &RepositoryIdentityResult{
			Seq:      seq,
			FilePath: path,
			Identity: git.RepositoryIdentity{Root: "/repo", Branch: "late"},
		}
	}
	inFlight.InvalidateAll(RepositoryWorktree)
	inFlight.identityDebounceTimer.(*fakeRepositoryTimer).fire()
	inFlight.HandleIdentityDebounce(inFlightPoster.await(t).(*repositoryIdentityDebounceTick))
	<-started
	inFlight.Close()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel the in-flight identity read")
	}
	inFlight.HandleIdentity(inFlightPoster.await(t).(*RepositoryIdentityResult))
	if root, branch := inFlight.ActiveRepository(); root != "/repo" || branch != "last-good" {
		t.Fatalf("late close result replaced identity = (%q, %q)", root, branch)
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

func canonicalTestPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func testAppGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
