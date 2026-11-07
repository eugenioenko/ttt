package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/ui"
)

type changeReadRepository struct {
	testSCMRepository
	result   scm.ChangeDiff
	err      error
	requests []scm.Change
}

func (r *changeReadRepository) ReadChange(ctx context.Context, change scm.Change) (scm.ChangeDiff, error) {
	r.requests = append(r.requests, change)
	return r.result, errors.Join(r.err, ctx.Err())
}

func providerChangeDiff() scm.ChangeDiff {
	return scm.ChangeDiff{
		Before: scm.FileContent{Content: "provider before\n", Exists: true},
		After:  scm.FileContent{Content: "provider after\n", Exists: true},
		Patch:  "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-provider before\n+provider after\n",
	}
}

func TestProviderChangeReadUsesTypedPanelSelection(t *testing.T) {
	r := &changeReadRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}, result: providerChangeDiff()}
	a, _ := newBaselineTestApp(t, r)
	a.Changes.OnOpenDiff = a.OpenChangeDiff
	change := scm.Change{Path: "[日本語].txt", OldPath: "old.txt", Kind: scm.Renamed, Group: "pending", Detail: "provider detail"}
	a.Changes.openDiff(r.root, change, true)
	d := a.EditorGroup.ActiveDiffWidget()
	if d == nil || d.Editable() || !reflect.DeepEqual(d.LeftLines(), []string{"provider before"}) || !reflect.DeepEqual(d.RightLines(), []string{"provider after"}) {
		t.Fatal("provider content did not open a read-only diff")
	}
	if len(r.requests) != 1 || r.requests[0] != change {
		t.Fatalf("selection lost metadata: %+v", r.requests)
	}
	change.Group = scm.WorkingTree
	a.OpenChangeDiff(r.root, change, true)
	d = a.EditorGroup.ActiveDiffWidget()
	if d == nil || d.Editable() || len(r.requests) != 2 {
		t.Fatal("a provider without editable baselines could not open a read-only diff")
	}
	before := a.EditorGroup.ActiveFilePath()
	r.err = errors.New("provider read failed")
	a.OpenChangeDiff(r.root, change, true)
	if a.EditorGroup.ActiveFilePath() != before || a.EditorGroup.ActiveDiffWidget() != d {
		t.Fatal("failed read replaced the open diff")
	}
}

func TestProviderChangeReadOpensEmptyDeletion(t *testing.T) {
	r := &changeReadRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}, result: scm.ChangeDiff{
		Before: scm.FileContent{Exists: true}, Patch: "diff --git a/empty.txt b/empty.txt\ndeleted file mode 100644\n",
	}}
	a, _ := newBaselineTestApp(t, r)
	a.OpenChangeDiff(r.root, scm.Change{Path: "empty.txt", Kind: scm.Deleted}, false)
	d := a.EditorGroup.ActiveDiffWidget()
	if d == nil || d.Editable() || d.ContextMode() != ui.DiffContextFullFile {
		t.Fatal("metadata-only deletion did not open a full-file diff")
	}
}

func TestProviderChangeReadUnsupportedDoesNotFallBackToGit(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a
	root, _ := a.Repository.ActiveRepository()
	a.Repository.Close()
	a.Repository = NewRepositoryState(NewChangesPanelWithProvider(&testSCMProvider{repo: testSCMRepository{root: root}}, root), []string{root})
	a.OpenChangeDiff(root, scm.Change{Path: "file.txt", Group: scm.Staged}, false)
	if a.EditorGroup.ActiveDiffWidget() != nil {
		t.Fatal("unsupported provider fell back to Git")
	}
}

func TestProviderChangeReadQueuedOpenCanceledByPlainFileOpen(t *testing.T) {
	r := &changeReadRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}, result: providerChangeDiff()}
	a, _ := newBaselineTestApp(t, r)
	h := &diffOpenTestHarness{a: a}
	a.tasks.post = func(fn func()) error { h.deliveries = append(h.deliveries, fn); return nil }
	a.diffOpenTask = NewTask[*diffOpenResult](a, asynctask.TaskOptions{Run: func(fn func()) { h.work = append(h.work, fn) }})
	a.OpenChangeDiff(r.root, scm.Change{Path: "[日本語].txt", Group: scm.Staged}, false)
	stale := h.run(t)
	a.OpenChangeDiff(r.root, scm.Change{Path: "[日本語].txt", Kind: scm.Untracked}, false)
	stale()
	if a.EditorGroup.ActiveDiffWidget() != nil || diffOpenStatus(a) != "" {
		t.Fatal("queued diff overrode a newer file open")
	}
}

func TestProviderChangeReadTimeoutShowsWarning(t *testing.T) {
	r := &changeReadRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}}
	a, _ := newBaselineTestApp(t, r)
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	result := readChangeDiff(ctx, a.Repository.repositorySource(r.root), scm.Change{Path: "file.txt", Group: scm.Staged}, false)
	a.applyDiffOpen(a.EditorGroup.ActiveFilePath(), result)
	if !strings.Contains(a.Status.Notification, context.DeadlineExceeded.Error()) || a.EditorGroup.ActiveDiffWidget() != nil {
		t.Fatal("timed-out diff read did not preserve the current view and show its warning")
	}
}
