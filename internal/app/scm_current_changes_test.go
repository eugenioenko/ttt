package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/ui"
)

type currentChangesRepository struct {
	testSCMRepository
	read func(context.Context, scm.Change) (scm.ChangeDiff, error)
}

func (r *currentChangesRepository) ReadChange(ctx context.Context, change scm.Change) (scm.ChangeDiff, error) {
	return r.read(ctx, change)
}

func TestCurrentChangesConsumesProviderGroupsAndEndpoints(t *testing.T) {
	r := &currentChangesRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}}
	changes := []scm.Change{
		{Path: "界.txt", OldPath: "old.txt", Kind: scm.Renamed, Group: "pending", Detail: "opaque diagnostic"},
		{Path: "界.txt", Kind: scm.Modified, Group: "other"},
	}
	var requests []scm.Change
	r.read = func(_ context.Context, change scm.Change) (scm.ChangeDiff, error) {
		requests = append(requests, change)
		return scm.ChangeDiff{Before: scm.FileContent{Content: "provider before\n", Exists: true}, After: scm.FileContent{Content: "provider after\n", Exists: true}}, nil
	}
	source := repositorySource{root: r.root, repository: r}
	result := readCurrentChanges(context.Background(), source, "opaque revision", "tab", 7, 11, changes)
	if result.Err != nil || len(result.Files) != 2 || !reflect.DeepEqual(requests, changes) {
		t.Fatalf("result=%+v requests=%+v", result, requests)
	}
	first, second := result.Files[0], result.Files[1]
	if first.Status != "R" || first.OldPath != "old.txt" || first.Group != "pending" || first.Boundary != ui.CommitDetailBoundaryNone || second.Group != "other" {
		t.Fatalf("provider identity lost: %+v", result.Files)
	}
	samePath := first
	samePath.Group = second.Group
	if ui.CommitDetailContextKey(first) == ui.CommitDetailContextKey(samePath) {
		t.Fatal("provider groups collapsed")
	}
	added, deleted := currentDiffStats(first.Diff)
	if added != 1 || deleted != 1 || !strings.Contains(result.Summary, "1 file") {
		t.Fatalf("provider content or summary lost: %+v", result)
	}
	original := result.Fingerprint
	changes[0].Group = "another group"
	if next := readCurrentChanges(context.Background(), source, "opaque revision", "tab", 7, 12, changes); next.Fingerprint == original {
		t.Fatal("group transition did not invalidate content")
	}
}

func TestCurrentChangesProviderMetadataAndLargeFallback(t *testing.T) {
	r := &currentChangesRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}}
	content := scm.ChangeDiff{Conflicted: true, ConflictDetail: "provider conflict", ConflictSources: []string{"left", "right"}, Before: scm.FileContent{Content: strings.Repeat("old\n", 2001), Exists: true}, After: scm.FileContent{Content: strings.Repeat("new\n", 2001), Exists: true}}
	r.read = func(context.Context, scm.Change) (scm.ChangeDiff, error) { return content, nil }
	source := repositorySource{root: r.root, repository: r}
	changes := []scm.Change{{Path: "large.txt", Kind: scm.Conflicted, Group: "pending"}}
	read := func() *CurrentChangesResult {
		return readCurrentChanges(context.Background(), source, "revision", "tab", 1, 1, changes)
	}
	result := read()
	if result.Err != nil || len(result.Files) != 1 {
		t.Fatalf("result=%+v", result)
	}
	file := result.Files[0]
	added, deleted := currentDiffStats(file.Diff)
	if added != 2001 || deleted != 2001 || file.ConflictCode != "provider conflict" || !reflect.DeepEqual(file.ConflictSources, content.ConflictSources) {
		t.Fatalf("conflict metadata or bounded replacement lost: +%d -%d %+v", added, deleted, file)
	}
	content.ConflictSources = []string{"left", "different right"}
	next := read()
	if next.Fingerprint == result.Fingerprint || ui.CommitDetailContextKey(next.Files[0]) == ui.CommitDetailContextKey(file) {
		t.Fatal("conflict-side transition did not invalidate identity")
	}
	content = scm.ChangeDiff{Before: scm.FileContent{Exists: true}, After: scm.FileContent{Exists: true}, Patch: "diff --git a/large.txt b/large.txt\nold mode 100644\nnew mode 100755\n"}
	mode := read()
	content.Patch = "diff --git a/large.txt b/large.txt\nold mode 100644\nnew mode 100600\n"
	if mode.Err != nil || mode.Files[0].ContentKind != ui.CommitDetailContentEmpty || mode.Fingerprint == read().Fingerprint {
		t.Fatal("empty metadata-only change was lost")
	}
}

func TestCurrentChangesUnsupportedFailureAndCancellation(t *testing.T) {
	// Being inside Git must not silently substitute its reader for this provider.
	root := testAppRepository(t)
	unsupported := &testSCMRepository{root: root}
	result := readCurrentChanges(context.Background(), repositorySource{root: root, repository: unsupported}, "revision", "tab", 1, 1, nil)
	if !errors.Is(result.Err, errors.ErrUnsupported) {
		t.Fatalf("unsupported provider fell back: %+v", result)
	}
	r := &currentChangesRepository{testSCMRepository: testSCMRepository{root: root}}
	failure := errors.New("provider read failed")
	r.read = func(context.Context, scm.Change) (scm.ChangeDiff, error) { return scm.ChangeDiff{}, failure }
	changes := []scm.Change{{Path: "initial.txt", Group: "pending"}}
	source := repositorySource{root: root, repository: r}
	result = readCurrentChanges(context.Background(), source, "revision", "tab", 1, 1, changes)
	if !errors.Is(result.Err, failure) {
		t.Fatalf("read failure became missing content: %+v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.read = func(context.Context, scm.Change) (scm.ChangeDiff, error) { cancel(); return scm.ChangeDiff{}, nil }
	result = readCurrentChanges(ctx, source, "revision", "tab", 1, 1, changes)
	if !result.Canceled || !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("canceled delivery survived: %+v", result)
	}
}

func TestCurrentChangesWorkerUsesCapturedRepository(t *testing.T) {
	root := t.TempDir()
	s, _, poster := testRepositoryState(nil, root)
	defer s.Close()
	r := &currentChangesRepository{testSCMRepository: testSCMRepository{root: root}}
	change := scm.Change{Path: "file.txt", Group: "pending", Kind: scm.Modified}
	r.read = func(_ context.Context, got scm.Change) (scm.ChangeDiff, error) {
		if got != change {
			return scm.ChangeDiff{}, errors.New("typed change lost")
		}
		return scm.ChangeDiff{After: scm.FileContent{Content: "captured provider\n", Exists: true}}, nil
	}
	s.repositories[root] = r
	primeCurrentChangesInput(s, root, "opaque", changesGroup{Dir: root, Changes: []scm.Change{change}})
	started, release := make(chan struct{}), make(chan struct{})
	s.readCurrentChanges = func(ctx context.Context, source repositorySource, revision, tabID string, epoch, request uint64, changes []scm.Change) *CurrentChangesResult {
		close(started)
		<-release
		return readCurrentChanges(ctx, source, revision, tabID, epoch, request, changes)
	}
	s.SetCurrentChangesRoot(root, currentChangesTabID(root))
	s.EnsureCurrentChanges()
	<-started
	s.repositories[root] = &testSCMRepository{root: root}
	close(release)
	result := poster.await(t).(*CurrentChangesResult)
	if result.Err != nil || len(result.Files) != 1 || !s.HandleCurrentChanges(result) {
		t.Fatalf("worker did not use captured provider: %+v", result)
	}
}

func TestCurrentChangesOnlyDeduplicatesExplicitConflictViews(t *testing.T) {
	r := &currentChangesRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}}
	content := scm.ChangeDiff{Conflicted: true, ConflictSources: []string{"left", "right"}, Before: scm.FileContent{Content: "before\n", Exists: true}, After: scm.FileContent{Content: "resolved\n", Exists: true}}
	r.read = func(context.Context, scm.Change) (scm.ChangeDiff, error) { return content, nil }
	changes := []scm.Change{{Path: "file.txt", Group: "first"}, {Path: "file.txt", Group: "second"}}
	source := repositorySource{root: r.root, repository: r}
	read := func() *CurrentChangesResult {
		return readCurrentChanges(context.Background(), source, "revision", "tab", 1, 1, changes)
	}
	result := read()
	if result.Err != nil || len(result.Files) != 2 {
		t.Fatalf("distinct provider groups collapsed without opt-in: %+v", result)
	}
	content.ConflictID = "shared resolution"
	result = read()
	if result.Err != nil || len(result.Files) != 1 || !strings.Contains(result.Summary, "1 conflict") {
		t.Fatalf("shared resolution was not projected once: %+v", result)
	}
}
