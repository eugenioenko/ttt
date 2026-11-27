package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/scm"
)

type historyRepository struct {
	testSCMRepository
	read func(context.Context, scm.HistoryRequest) (scm.HistoryPage, error)
}

func (r *historyRepository) ReadHistory(ctx context.Context, request scm.HistoryRequest) (scm.HistoryPage, error) {
	return r.read(ctx, request)
}

func TestProviderHistoryUsesOpaqueAnchorsAndBoundedPages(t *testing.T) {
	r := &historyRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}}
	var requests []scm.HistoryRequest
	r.read = func(_ context.Context, req scm.HistoryRequest) (scm.HistoryPage, error) {
		requests = append(requests, req)
		return scm.HistoryPage{Anchor: "snapshot:opaque", HasMore: true, Entries: []scm.HistoryEntry{{Ref: "revision:opaque^", Label: "change label", Message: "provider history"}}}, nil
	}
	source := repositorySource{root: r.root, repository: r}
	result := readCommitLog(context.Background(), source, 7)
	if result.Err != nil || result.Gen != 7 || result.Branch != "working-change" || result.Anchor != "snapshot:opaque" || len(result.Entries) != 1 || result.Entries[0].Label != "change label" {
		t.Fatalf("provider history: %+v", result)
	}
	next := readCommitLogPage(context.Background(), source, result.Anchor, 10, 7)
	if next.Err != nil || !next.Append || next.Offset != 10 || next.Anchor != result.Anchor {
		t.Fatalf("next page: %+v", next)
	}
	want := []scm.HistoryRequest{{Limit: commitLogInitialLimit}, {Anchor: result.Anchor, Offset: 10, Limit: commitLogPageSize}}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests=%+v, want %+v", requests, want)
	}
}

func TestProviderHistoryUnsupportedErrorsAndCancellation(t *testing.T) {
	root := testAppRepository(t)
	source := repositorySource{root: root, repository: &testSCMRepository{root: root}}
	if got := readCommitLog(context.Background(), source, 1); !got.Unavailable || got.Err != nil {
		t.Fatalf("unsupported provider fell back to Git: %+v", got)
	}
	r := &historyRepository{testSCMRepository: testSCMRepository{root: root}}
	failure := errors.New("history failed")
	r.read = func(context.Context, scm.HistoryRequest) (scm.HistoryPage, error) { return scm.HistoryPage{}, failure }
	source.repository = r
	if got := readCommitLog(context.Background(), source, 1); !errors.Is(got.Err, failure) {
		t.Fatalf("read failure hidden: %+v", got)
	}
	r.read = func(context.Context, scm.HistoryRequest) (scm.HistoryPage, error) {
		return scm.HistoryPage{Anchor: "different"}, nil
	}
	if got := readCommitLogPage(context.Background(), source, "snapshot", 10, 1); got.Err == nil {
		t.Fatalf("snapshot drift accepted: %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.read = func(context.Context, scm.HistoryRequest) (scm.HistoryPage, error) {
		cancel()
		return scm.HistoryPage{}, nil
	}
	if got := readCommitLog(ctx, source, 1); !got.Canceled || !errors.Is(got.Err, context.Canceled) {
		t.Fatalf("canceled read accepted: %+v", got)
	}
}

func TestHistoryWorkerCapturesRepositoryBeforeLaunch(t *testing.T) {
	root := t.TempDir()
	r := &historyRepository{testSCMRepository: testSCMRepository{root: root}}
	started, release := make(chan struct{}), make(chan struct{})
	r.read = func(context.Context, scm.HistoryRequest) (scm.HistoryPage, error) {
		close(started)
		<-release
		return scm.HistoryPage{Anchor: "snapshot", Entries: []scm.HistoryEntry{{Ref: "opaque", Label: "label", Message: "captured"}}}, nil
	}
	cp := NewChangesPanelWithProvider(&testSCMProvider{repo: r}, root)
	cp.groups = []changesGroup{{Dir: root, Repository: r}}
	poster := newTestEventPoster()
	cp.Screen = poster
	defer cp.Shutdown()
	cp.refreshCommitLog()
	<-started
	cp.groups[0].Repository = &testSCMRepository{root: root}
	close(release)
	result := poster.await(t).(*CommitLogResult)
	if result.Err != nil || len(result.Entries) != 1 || result.Entries[0].Message != "captured" {
		t.Fatalf("worker lost captured repository: %+v", result)
	}
}
