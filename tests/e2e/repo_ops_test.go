package e2e

import (
	"errors"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/gdamore/tcell/v3"
)

// awaitRepoOp drains the event the background task posts and applies it the way
// the event loop would.
func (h *testHarness) awaitRepoOp() *app.RepoOpResult {
	h.t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-h.screen.EventQ():
			// The harness's initial SetSize leaves a resize event queued.
			iev, ok := ev.(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			res, ok := iev.Data().(*app.RepoOpResult)
			if !ok {
				continue
			}
			h.app.HandleRepoOpResult(res)
			h.redraw()
			return res
		case <-deadline:
			h.t.Fatal("timed out waiting for the repo task to finish")
			return nil
		}
	}
}

func blockingTask(started, release chan struct{}, dir string) app.RepoTask {
	return app.RepoTask{
		Progress: "Pulling",
		Done:     "Pulled successfully",
		Dirs:     []string{dir},
		Ops: []app.RepoOp{{
			Name: "fake",
			Run: func(string) error {
				close(started)
				<-release
				return nil
			},
		}},
	}
}

// The UI must stay live while the operation runs, with progress on the status
// bar rather than a frozen screen (#236).
func TestRepoTaskShowsProgressWhileRunning(t *testing.T) {
	h := newTestHarness(t, 100, 30)

	started := make(chan struct{})
	release := make(chan struct{})
	h.app.RunRepoTask(blockingTask(started, release, h.dir))
	<-started

	h.redraw()
	h.assertContains("Pulling…")

	close(release)
	if res := h.awaitRepoOp(); res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	h.assertContains("Pulled successfully")
	h.assertNotContains("Pulling…")
}

// git refuses concurrent writes to a repo with an index.lock error, so a second
// task must be turned away rather than started.
func TestRepoTaskRefusesConcurrentRun(t *testing.T) {
	h := newTestHarness(t, 100, 30)

	started := make(chan struct{})
	release := make(chan struct{})
	h.app.RunRepoTask(blockingTask(started, release, h.dir))
	<-started

	ran := false
	h.app.RunRepoTask(app.RepoTask{
		Progress: "Pushing",
		Done:     "Pushed successfully",
		Dirs:     []string{h.dir},
		Ops:      []app.RepoOp{{Name: "second", Run: func(string) error { ran = true; return nil }}},
	})
	if ran {
		t.Error("a second task ran while one was already in flight")
	}
	h.redraw()
	h.assertContains("already running")

	close(release)
	h.awaitRepoOp()
}

// The guard must clear once the task finishes, or git commands stop working for
// the rest of the session.
func TestRepoTaskGuardClearsAfterCompletion(t *testing.T) {
	h := newTestHarness(t, 100, 30)

	started := make(chan struct{})
	release := make(chan struct{})
	h.app.RunRepoTask(blockingTask(started, release, h.dir))
	<-started
	close(release)
	h.awaitRepoOp()

	secondStarted := make(chan struct{})
	secondRelease := make(chan struct{})
	h.app.RunRepoTask(blockingTask(secondStarted, secondRelease, h.dir))
	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("a task after a completed one never started")
	}
	close(secondRelease)
	h.awaitRepoOp()
}

func TestRepoTaskReportsFailure(t *testing.T) {
	h := newTestHarness(t, 100, 30)

	h.app.RunRepoTask(app.RepoTask{
		Progress: "Pushing",
		Done:     "Pushed successfully",
		Dirs:     []string{h.dir},
		Ops: []app.RepoOp{{
			Name: "push",
			Run:  func(string) error { return errors.New("no upstream") },
		}},
	})

	res := h.awaitRepoOp()
	if res.Err == nil {
		t.Fatal("expected an error")
	}
	// The op name is kept so the message says which step failed.
	h.assertContains("Pushing failed")
	h.assertNotContains("Pushed successfully")
	h.assertNotContains("Pushing…")
}

// A later dir must not run once an earlier one failed, so a failed pull does
// not get followed by a push over a stale tree.
func TestRepoTaskStopsAtFirstFailure(t *testing.T) {
	h := newTestHarness(t, 100, 30)

	secondRan := false
	h.app.RunRepoTask(app.RepoTask{
		Progress: "Syncing",
		Done:     "Synced successfully",
		Dirs:     []string{h.dir},
		Ops: []app.RepoOp{
			{Name: "pull", Run: func(string) error { return errors.New("conflict") }},
			{Name: "push", Run: func(string) error { secondRan = true; return nil }},
		},
	})

	h.awaitRepoOp()
	if secondRan {
		t.Error("the second op ran after the first failed")
	}
}
