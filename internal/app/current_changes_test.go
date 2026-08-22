package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
)

func currentChangesStatuses(t *testing.T, dir string) []git.FileStatus {
	t.Helper()
	statuses, err := git.StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	return statuses
}

func TestReadCurrentChangesUsesRawStatusIdentityForWorkingTreeShapes(t *testing.T) {
	dir := testAppRepository(t)
	write := func(name string, content []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("delete.txt", []byte("remove me\n"))
	write("old name.txt", []byte("before rename\n"))
	write("blob.bin", []byte{0, 1, 2, 3})
	testAppGit(t, dir, "add", "-A")
	testAppGit(t, dir, "commit", "-m", "shapes")

	write("initial.txt", []byte("staged version\n"))
	testAppGit(t, dir, "add", "initial.txt")
	write("initial.txt", []byte("final working version\n"))
	if err := os.Remove(filepath.Join(dir, "delete.txt")); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "mv", "old name.txt", "renamed:界.txt")
	write("renamed:界.txt", []byte("after rename 界\n"))
	write("blob.bin", []byte{0, 9, 8, 7})
	write("empty.txt", nil)
	rawPath := "raw\n\t界.txt"
	write(rawPath, []byte("path content\n"))

	revision := git.RevisionIdentity(dir)
	result := readCurrentChanges(context.Background(), dir, revision, currentChangesTabID(dir), 7, 11, currentChangesStatuses(t, dir))
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	files := make(map[string]int)
	for i := range result.Files {
		files[result.Files[i].Path] = i
	}
	for _, path := range []string{"initial.txt", "delete.txt", "renamed:界.txt", "blob.bin", "empty.txt", rawPath} {
		if _, ok := files[path]; !ok {
			t.Fatalf("typed current changes omitted %q: %+v", path, result.Files)
		}
	}
	mixed := result.Files[files["initial.txt"]]
	if mixed.Stage != ui.CommitDetailStageMixed {
		t.Fatalf("mixed stage = %v", mixed.Stage)
	}
	foundFinal := false
	for _, line := range mixed.Diff.AllLines() {
		foundFinal = foundFinal || line.Right.Text == "final working version"
		if line.Right.Text == "staged version" {
			t.Fatal("current changes stopped at the staged snapshot")
		}
	}
	if !foundFinal {
		t.Fatalf("mixed diff omitted final working content: %+v", mixed.Diff.AllLines())
	}
	deleted := result.Files[files["delete.txt"]]
	if deleted.Status != "D" || len(deleted.Diff.Hunks) == 0 {
		t.Fatalf("deleted file = %+v", deleted)
	}
	renamed := result.Files[files["renamed:界.txt"]]
	if renamed.Status != "R" || renamed.OldPath != "old name.txt" {
		t.Fatalf("renamed file identity = %+v", renamed)
	}
	if result.Files[files["blob.bin"]].ContentKind != ui.CommitDetailContentBinary {
		t.Fatalf("binary file kind = %v", result.Files[files["blob.bin"]].ContentKind)
	}
	if result.Files[files["empty.txt"]].ContentKind != ui.CommitDetailContentEmpty {
		t.Fatalf("empty file kind = %v", result.Files[files["empty.txt"]].ContentKind)
	}
	if !strings.Contains(result.Summary, "mixed") || result.Fingerprint == "" {
		t.Fatalf("summary=%q fingerprint=%q", result.Summary, result.Fingerprint)
	}
}

func TestReadCurrentChangesSupportsTypedCopyIdentity(t *testing.T) {
	dir := testAppRepository(t)
	if err := os.WriteFile(filepath.Join(dir, "copy.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := readCurrentChanges(context.Background(), dir, git.RevisionIdentity(dir), currentChangesTabID(dir), 1, 1,
		[]git.FileStatus{{Status: "C", OldPath: "initial.txt", Path: "copy.txt", Staged: true}})
	if result.Err != nil || len(result.Files) != 1 || result.Files[0].Status != "C" || result.Files[0].OldPath != "initial.txt" {
		t.Fatalf("copy result=%+v", result)
	}
}

func TestReadCurrentChangesHandlesAddedFilesWithFurtherWorkingTreeChanges(t *testing.T) {
	dir := testAppRepository(t)
	path := filepath.Join(dir, "added.txt")
	if err := os.WriteFile(path, []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "add", "added.txt")
	if err := os.WriteFile(path, []byte("final\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := readCurrentChanges(context.Background(), dir, git.RevisionIdentity(dir), currentChangesTabID(dir), 1, 1, currentChangesStatuses(t, dir))
	if result.Err != nil || len(result.Files) != 1 {
		t.Fatalf("added mixed result=%+v", result)
	}
	file := result.Files[0]
	if file.Status != "A" || file.Stage != ui.CommitDetailStageMixed {
		t.Fatalf("added mixed identity=%+v", file)
	}
	foundFinal := false
	for _, line := range file.Diff.AllLines() {
		foundFinal = foundFinal || line.Right.Text == "final"
	}
	if !foundFinal {
		t.Fatalf("added mixed diff omitted final content: %+v", file.Diff.AllLines())
	}
}

func TestCurrentChangesTabIdentityDoesNotCollideAcrossRoots(t *testing.T) {
	left := filepath.Join(t.TempDir(), "project")
	right := filepath.Join(t.TempDir(), "project")
	if currentChangesTabID(left) == currentChangesTabID(right) {
		t.Fatalf("multi-root tab IDs collided for %q and %q", left, right)
	}
}

func TestReadCurrentChangesFingerprintIncludesStageIdentity(t *testing.T) {
	dir := testAppRepository(t)
	path := filepath.Join(dir, "initial.txt")
	if err := os.WriteFile(path, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	revision := git.RevisionIdentity(dir)
	unstaged := readCurrentChanges(context.Background(), dir, revision, currentChangesTabID(dir), 1, 1, currentChangesStatuses(t, dir))
	if unstaged.Err != nil {
		t.Fatal(unstaged.Err)
	}
	testAppGit(t, dir, "add", "initial.txt")
	staged := readCurrentChanges(context.Background(), dir, revision, currentChangesTabID(dir), 1, 2, currentChangesStatuses(t, dir))
	if staged.Err != nil {
		t.Fatal(staged.Err)
	}
	if unstaged.Fingerprint == staged.Fingerprint {
		t.Fatal("stage-only transition did not change the current changes fingerprint")
	}
}

func primeCurrentChangesInput(s *RepositoryState, dir, revision string, group changesGroup) {
	statuses := make([]git.FileStatus, 0, len(group.Staged)+len(group.Unstaged))
	statuses = append(statuses, group.Staged...)
	statuses = append(statuses, group.Unstaged...)
	s.lastGroups[dir] = group
	s.lastRevisions[dir] = revision
	s.currentInputs[dir] = newCurrentChangesInput(revision, statuses)
}

func TestRepositoryCurrentChangesCoalescesDropsStaleAndRetriesErrors(t *testing.T) {
	dir := "/repo"
	s, _, poster := testRepositoryState(nil, dir)
	primeCurrentChangesInput(s, dir, "head", changesGroup{Dir: dir, Unstaged: []git.FileStatus{{Status: "M", Path: "file.txt"}}})
	started := make(chan uint64, 4)
	release := make(chan struct{}, 4)
	s.readCurrentChanges = func(ctx context.Context, dir, revision, tabID string, epoch, request uint64, statuses []git.FileStatus) *CurrentChangesResult {
		started <- request
		select {
		case <-release:
			return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Fingerprint: "fresh", Summary: "fresh"}
		case <-ctx.Done():
			return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Err: ctx.Err(), Canceled: true}
		}
	}
	var applied atomic.Int32
	s.SetCurrentChangesHandler(func(*CurrentChangesResult) { applied.Add(1) })
	tabID := currentChangesTabID(dir)
	epoch := s.SetCurrentChangesRoot(dir, tabID)
	s.EnsureCurrentChanges()
	first := <-started
	s.currentInputs[dir] = newCurrentChangesInput("head", []git.FileStatus{{Status: "M", Path: "new-file.txt"}})
	s.requestCurrentChanges()
	s.requestCurrentChanges()
	if s.currentRequest == first {
		t.Fatal("in-flight invalidations did not supersede the request")
	}
	s.HandleCurrentChanges(poster.await(t).(*CurrentChangesResult))
	second := <-started
	if second != s.currentRequest {
		t.Fatalf("coalesced request = %d, want %d", second, s.currentRequest)
	}
	release <- struct{}{}
	if !s.HandleCurrentChanges(poster.await(t).(*CurrentChangesResult)) || applied.Load() != 1 {
		t.Fatalf("fresh result was not applied once: epoch=%d applied=%d", epoch, applied.Load())
	}

	s.readCurrentChanges = func(_ context.Context, dir, revision, tabID string, epoch, request uint64, statuses []git.FileStatus) *CurrentChangesResult {
		return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Err: errors.New("index locked")}
	}
	s.requestCurrentChanges()
	if !s.HandleCurrentChanges(poster.await(t).(*CurrentChangesResult)) || !s.currentDirty || applied.Load() != 2 {
		t.Fatalf("error was not visible and retryable: dirty=%v applied=%d", s.currentDirty, applied.Load())
	}
	s.readCurrentChanges = func(_ context.Context, dir, revision, tabID string, epoch, request uint64, statuses []git.FileStatus) *CurrentChangesResult {
		return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Fingerprint: "fresh", Summary: "recovered"}
	}
	s.currentInputReady = true
	s.requestCurrentChanges()
	if !s.HandleCurrentChanges(poster.await(t).(*CurrentChangesResult)) || s.currentDirty || applied.Load() != 3 {
		t.Fatalf("retry did not recover: dirty=%v applied=%d", s.currentDirty, applied.Load())
	}
}

func TestRepositoryCurrentChangesIdenticalPollCoalescesWithoutStarvation(t *testing.T) {
	dir := "/repo"
	s, _, poster := testRepositoryState(nil, dir)
	primeCurrentChangesInput(s, dir, "head", changesGroup{Dir: dir, Unstaged: []git.FileStatus{{Status: "M", Path: "file.txt"}}})
	started := make(chan uint64, 2)
	release := make(chan struct{}, 2)
	s.readCurrentChanges = func(ctx context.Context, dir, revision, tabID string, epoch, request uint64, statuses []git.FileStatus) *CurrentChangesResult {
		started <- request
		select {
		case <-release:
			return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Fingerprint: strings.Repeat("x", int(request))}
		case <-ctx.Done():
			return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Err: ctx.Err(), Canceled: true}
		}
	}
	var applied atomic.Int32
	s.SetCurrentChangesHandler(func(*CurrentChangesResult) { applied.Add(1) })
	s.SetCurrentChangesRoot(dir, currentChangesTabID(dir))
	s.EnsureCurrentChanges()
	first := <-started
	s.requestCurrentChanges()
	if s.currentRequest != first {
		t.Fatalf("identical poll superseded in-flight request: first=%d current=%d", first, s.currentRequest)
	}
	release <- struct{}{}
	if !s.HandleCurrentChanges(poster.await(t).(*CurrentChangesResult)) {
		t.Fatal("slow identical-input read was not applied")
	}
	second := <-started
	if second == first {
		t.Fatalf("coalesced follow-up reused request %d", first)
	}
	release <- struct{}{}
	if !s.HandleCurrentChanges(poster.await(t).(*CurrentChangesResult)) || applied.Load() != 2 {
		t.Fatalf("coalesced follow-up did not apply: applied=%d", applied.Load())
	}
}

func TestRepositoryCurrentChangesCancelsOnCloseAndRootSupersession(t *testing.T) {
	s, _, poster := testRepositoryState(nil, "/old")
	for _, dir := range []string{"/old", "/new"} {
		primeCurrentChangesInput(s, dir, "head", changesGroup{Dir: dir})
	}
	oldCanceled := make(chan struct{})
	s.readCurrentChanges = func(ctx context.Context, dir, revision, tabID string, epoch, request uint64, statuses []git.FileStatus) *CurrentChangesResult {
		if dir == "/old" {
			<-ctx.Done()
			close(oldCanceled)
			return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Err: ctx.Err(), Canceled: true}
		}
		return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Fingerprint: "new"}
	}
	s.SetCurrentChangesRoot("/old", currentChangesTabID("/old"))
	s.EnsureCurrentChanges()
	s.SetCurrentChangesRoot("/new", currentChangesTabID("/new"))
	s.EnsureCurrentChanges()
	select {
	case <-oldCanceled:
	case <-time.After(time.Second):
		t.Fatal("root supersession did not cancel the old current-changes read")
	}
	for range 2 {
		s.HandleCurrentChanges(poster.await(t).(*CurrentChangesResult))
	}
	if s.currentRoot != "/new" || s.currentAppliedFingerprint != "new" {
		t.Fatalf("superseded root survived: root=%q fingerprint=%q", s.currentRoot, s.currentAppliedFingerprint)
	}

	canceled := make(chan struct{})
	s.readCurrentChanges = func(ctx context.Context, dir, revision, tabID string, epoch, request uint64, statuses []git.FileStatus) *CurrentChangesResult {
		<-ctx.Done()
		close(canceled)
		return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Err: ctx.Err(), Canceled: true}
	}
	s.requestCurrentChanges()
	s.Close()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("repository close did not cancel current changes")
	}
}

func TestRepositoryCurrentChangesCancelsImmediatelyWhenTabCloses(t *testing.T) {
	dir := "/repo"
	s, _, _ := testRepositoryState(nil, dir)
	primeCurrentChangesInput(s, dir, "head", changesGroup{Dir: dir})
	canceled := make(chan struct{})
	s.readCurrentChanges = func(ctx context.Context, dir, revision, tabID string, epoch, request uint64, statuses []git.FileStatus) *CurrentChangesResult {
		<-ctx.Done()
		close(canceled)
		return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Err: ctx.Err(), Canceled: true}
	}
	tabID := currentChangesTabID(dir)
	s.SetCurrentChangesRoot(dir, tabID)
	s.EnsureCurrentChanges()
	s.CloseCurrentChangesTab(tabID)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("tab close did not cancel current changes immediately")
	}
	if s.currentRoot != "" || s.currentTabID != "" || s.currentInFlight {
		t.Fatalf("tab close left current changes active: root=%q tab=%q inFlight=%v", s.currentRoot, s.currentTabID, s.currentInFlight)
	}
}

func TestRepositoryCurrentChangesWaitsForFreshStatusAfterStatusFailure(t *testing.T) {
	dir := "/repo"
	s := NewRepositoryState(nil, []string{dir})
	group := changesGroup{Dir: dir, Unstaged: []git.FileStatus{{Status: "M", Path: "kept.txt"}}}
	primeCurrentChangesInput(s, dir, "head", group)
	s.SetCurrentChangesRoot(dir, currentChangesTabID(dir))
	s.currentInputReady = false
	s.requested = 1
	s.inFlight = true
	s.dirty = RepositoryWorktree | RepositoryCurrentChanges
	var reads atomic.Int32
	var reported atomic.Int32
	s.readCurrentChanges = func(_ context.Context, dir, revision, tabID string, epoch, request uint64, statuses []git.FileStatus) *CurrentChangesResult {
		reads.Add(1)
		return &CurrentChangesResult{Dir: dir, TabID: tabID, Epoch: epoch, Request: request, Fingerprint: "unexpected"}
	}
	s.SetCurrentChangesHandler(func(result *CurrentChangesResult) {
		if result.Err != nil {
			reported.Add(1)
		}
	})

	s.HandleStatus(&RepositoryStatusResult{Seq: 1, Entries: []repositoryStatusEntry{{
		Group:       changesGroup{Dir: dir},
		Revision:    "",
		StatusErr:   errors.New("index locked"),
		RevisionErr: errors.New("HEAD unavailable"),
	}}})
	s.inFlight = true
	s.EnsureCurrentChanges()
	if reads.Load() != 0 || reported.Load() != 1 || s.currentInputReady || !s.currentDirty {
		t.Fatalf("failed status state: reads=%d reported=%d ready=%v dirty=%v", reads.Load(), reported.Load(), s.currentInputReady, s.currentDirty)
	}
}
