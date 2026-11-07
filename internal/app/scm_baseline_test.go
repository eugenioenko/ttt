package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/scm"
)

type baselineRepository struct {
	testSCMRepository
	baseline scm.Baseline
	err      error
	paths    []string
}

func (r *baselineRepository) ReadBaseline(ctx context.Context, path string) (scm.Baseline, error) {
	r.paths = append(r.paths, path)
	return r.baseline, errors.Join(r.err, ctx.Err())
}

func newBaselineTestApp(t *testing.T, repo scm.Repository) (*App, *testSCMProvider) {
	t.Helper()
	path := filepath.Join(repo.Root(), "[日本語].txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	a.Repository.Close()
	provider := &testSCMProvider{repo: repo}
	a.Changes = NewChangesPanelWithProvider(provider, repo.Root())
	a.Repository = NewRepositoryState(a.Changes, []string{repo.Root()})
	a.EditorGroup.OpenFile(path)
	a.SyncRepositoryBranch()
	t.Cleanup(func() {
		a.CloseTasks()
		a.Repository.Close()
		a.EditorGroup.CloseAllTabs()
	})
	return a, provider
}

func TestProviderBaselineDrivesGutterAndEditableDiff(t *testing.T) {
	repo := &baselineRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}, baseline: scm.Baseline{Content: "one\nprovider\n", Exists: true}}
	a, provider := newBaselineTestApp(t, repo)
	discoveries := len(provider.calls)
	a.RequestGitGutterForActiveFile()
	want := []diff.LineChangeKind{diff.LineUnchanged, diff.LineModified, diff.LineUnchanged}
	if got := a.EditorGroup.Editor.LineChanges; !reflect.DeepEqual(got, want) {
		t.Fatalf("provider gutter = %v, want %v", got, want)
	}
	a.ToggleInlineDiff()
	d := a.EditorGroup.ActiveInlineDiff()
	if d == nil || !reflect.DeepEqual(d.LeftLines(), []string{"one", "provider", ""}) {
		t.Fatalf("editable diff did not use provider baseline: %v", d)
	}
	if len(provider.calls) != discoveries {
		t.Fatal("baseline reads rediscovered an already known repository")
	}
	for _, path := range repo.paths {
		if path != "[日本語].txt" {
			t.Fatalf("nonliteral repository-relative path: %q", path)
		}
	}
	// A failed refresh must preserve the last valid gutter and editable base.
	repo.err = errors.New("provider read failed")
	a.RequestGitGutterForActiveFile()
	if !reflect.DeepEqual(a.EditorGroup.Editor.LineChanges, want) || !reflect.DeepEqual(d.LeftLines(), []string{"one", "provider", ""}) {
		t.Fatal("failed refresh replaced the last valid baseline")
	}
	// A conflict still closes the diff if its optional marker fallback fails.
	repo.baseline.Conflicted = true
	a.RequestGitGutterForActiveFile()
	if a.EditorGroup.IsInlineDiffActive() || !reflect.DeepEqual(a.EditorGroup.Editor.LineChanges, want) {
		t.Fatal("conflict must close the editable diff and retain fallback markers")
	}
	if opened, conflict, err := a.openInlineDiff(a.EditorGroup.ActiveFilePath(), repo.Root(), "[日本語].txt"); opened || !conflict || err != nil {
		t.Fatalf("conflict with failed fallback: %v, %v, %v", opened, conflict, err)
	}
}

func TestProviderBaselineStates(t *testing.T) {
	for _, tt := range []struct {
		name string
		base scm.Baseline
		want []diff.LineChangeKind
	}{
		{"missing", scm.Baseline{}, []diff.LineChangeKind{diff.LineAdded, diff.LineAdded, diff.LineAdded}},
		{"ignored", scm.Baseline{Ignored: true}, nil},
		{"conflict", scm.Baseline{Content: "one\nprovider\n", Exists: true, Conflicted: true}, []diff.LineChangeKind{diff.LineUnchanged, diff.LineModified, diff.LineUnchanged}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &baselineRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}, baseline: tt.base}
			a, _ := newBaselineTestApp(t, repo)
			a.RequestGitGutterForActiveFile()
			if got := a.EditorGroup.Editor.LineChanges; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("gutter = %v, want %v", got, tt.want)
			}
			path := a.EditorGroup.ActiveFilePath()
			opened, conflict, err := a.openInlineDiff(path, repo.Root(), filepath.Base(path))
			if err != nil || conflict != tt.base.Conflicted || opened == conflict {
				t.Fatalf("open diff = %v, %v, %v", opened, conflict, err)
			}
		})
	}
}

func TestProviderBaselineUnsupportedDoesNotFallBackToGit(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a
	path := a.EditorGroup.ActiveFilePath()
	repo := testSCMRepository{root: filepath.Dir(path)}
	a.Repository = NewRepositoryState(NewChangesPanelWithProvider(&testSCMProvider{repo: repo}, repo.Root()), []string{repo.Root()})
	a.SyncRepositoryBranch()
	opened, conflict, err := a.openInlineDiff(path, repo.Root(), filepath.Base(path))
	if opened || conflict || !errors.Is(err, errors.ErrUnsupported) || a.EditorGroup.IsInlineDiffActive() {
		t.Fatalf("unsupported provider fell back to Git: %v, %v, %v", opened, conflict, err)
	}
}

func TestProviderBaselineCancellation(t *testing.T) {
	repo := &baselineRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}}
	source := baselineSource{repository: repo}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := source.read(ctx, "file.txt"); !errors.Is(err, context.Canceled) || len(repo.paths) != 0 {
		t.Fatalf("canceled read reached provider: %v, %v", repo.paths, err)
	}
}
