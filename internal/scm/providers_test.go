package scm

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	repo Repository
	err  error
	hits *int
}

func (f fakeProvider) Discover(context.Context, string) (Repository, error) {
	*f.hits++
	return f.repo, f.err
}

type fakeRepository struct{ root string }

func (f fakeRepository) Root() string                             { return f.root }
func (fakeRepository) Identity(context.Context) (Identity, error) { return Identity{}, nil }
func (fakeRepository) Changes(context.Context) ([]Change, error)  { return nil, nil }
func (fakeRepository) Revision(context.Context) (string, error)   { return "", nil }

func TestProvidersUsesFirstClaimingProvider(t *testing.T) {
	var firstHits, secondHits int
	first := fakeProvider{err: ErrNotRepository, hits: &firstHits}
	second := fakeProvider{repo: fakeRepository{root: "/second"}, hits: &secondHits}
	repo, err := Providers{first, second}.Discover(context.Background(), "/x")
	if err != nil || repo.Root() != "/second" || firstHits != 1 || secondHits != 1 {
		t.Fatalf("repo=%v err=%v hits=%d/%d", repo, err, firstHits, secondHits)
	}

	firstHits, secondHits = 0, 0
	first = fakeProvider{repo: fakeRepository{root: "/first"}, hits: &firstHits}
	repo, err = Providers{first, second}.Discover(context.Background(), "/x")
	if err != nil || repo.Root() != "/first" || secondHits != 0 {
		t.Fatalf("precedence: repo=%v err=%v second=%d", repo, err, secondHits)
	}
}

func TestProvidersStopsOnRealErrors(t *testing.T) {
	var firstHits, secondHits int
	boom := errors.New("boom")
	_, err := Providers{fakeProvider{err: boom, hits: &firstHits}, fakeProvider{repo: fakeRepository{}, hits: &secondHits}}.Discover(context.Background(), "/x")
	if !errors.Is(err, boom) || secondHits != 0 {
		t.Fatalf("err=%v second=%d", err, secondHits)
	}
}

func TestProvidersReportsNotRepository(t *testing.T) {
	var hits int
	_, err := Providers{fakeProvider{err: ErrNotRepository, hits: &hits}}.Discover(context.Background(), "/x")
	if !errors.Is(err, ErrNotRepository) {
		t.Fatalf("err=%v", err)
	}
	if _, err := (Providers{}).Discover(context.Background(), "/x"); !errors.Is(err, ErrNotRepository) {
		t.Fatalf("empty providers err=%v", err)
	}
}
