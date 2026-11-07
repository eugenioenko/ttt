package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
)

type diffOpenTestHarness struct {
	a          *App
	work       []func()
	deliveries []func()
}

func newDiffOpenTestHarness(t *testing.T) *diffOpenTestHarness {
	t.Helper()
	h := &diffOpenTestHarness{a: prepareListenTestApp(t)}
	h.a.tasks.post = func(fn func()) error { h.deliveries = append(h.deliveries, fn); return nil }
	h.a.diffOpenTask = NewTask[*diffOpenResult](h.a, asynctask.TaskOptions{
		Run: func(fn func()) { h.work = append(h.work, fn) },
	})
	t.Cleanup(h.a.CloseTasks)
	return h
}

func (h *diffOpenTestHarness) run(t *testing.T) func() {
	t.Helper()
	if len(h.work) != 1 {
		t.Fatalf("queued reads = %d, want 1", len(h.work))
	}
	work := h.work[0]
	h.work = nil
	work()
	if len(h.deliveries) != 1 {
		t.Fatalf("queued deliveries = %d, want 1", len(h.deliveries))
	}
	deliver := h.deliveries[0]
	h.deliveries = nil
	return deliver
}

func diffOpenReply(name string) func(context.Context) *diffOpenResult {
	return func(context.Context) *diffOpenResult {
		return &diffOpenResult{TabName: name, Path: name, OldLines: []string{"old"}, NewLines: []string{"new"}}
	}
}

func diffOpenStatus(a *App) string {
	for _, segment := range a.Status.LeftSegments() {
		if segment.ID == "diffopen" {
			return segment.Text
		}
	}
	return ""
}

func TestDiffOpenLatestReplyWinsAtUIDelivery(t *testing.T) {
	h := newDiffOpenTestHarness(t)
	h.a.startDiffOpen(diffOpenReply("old diff"))
	stale := h.run(t)
	if h.a.EditorGroup.ActiveDiffWidget() != nil {
		t.Fatal("diff opened before UI delivery")
	}
	h.a.startDiffOpen(diffOpenReply("new diff"))
	stale()
	if diffOpenStatus(h.a) != "Opening diff…" || !h.a.diffOpenTask.Pending() {
		t.Fatal("superseded reply cleared the newer request's loading state")
	}
	h.run(t)()
	if h.a.EditorGroup.DiffWidgetByTab("old diff") != nil || h.a.EditorGroup.DiffWidgetByTab("new diff") == nil {
		t.Fatal("latest diff did not replace the superseded request")
	}
	if diffOpenStatus(h.a) != "" || h.a.diffOpenTask.Pending() {
		t.Fatal("completed diff left its loading state active")
	}
}

func TestDiffOpenQueuedReplyInvalidation(t *testing.T) {
	for _, action := range []string{"cancel", "navigation", "shutdown", "git shutdown"} {
		t.Run(action, func(t *testing.T) {
			h := newDiffOpenTestHarness(t)
			h.a.startDiffOpen(diffOpenReply("late diff"))
			stale := h.run(t)
			switch action {
			case "cancel":
				h.a.cancelPendingDiff()
			case "navigation":
				h.a.EditorGroup.NewFile()
			case "shutdown":
				h.a.CloseTasks()
			case "git shutdown":
				h.a.ShutdownGitReads()
			}
			stale()
			if h.a.EditorGroup.DiffWidgetByTab("late diff") != nil {
				t.Fatal("invalidated diff opened a tab")
			}
			if action != "shutdown" && diffOpenStatus(h.a) != "" {
				t.Fatal("discarded diff left its loading state active")
			}
		})
	}
}

func TestDiffOpenCancellationReachesRunningRead(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "shutdown"}[shutdown], func(t *testing.T) {
			h := newDiffOpenTestHarness(t)
			started := make(chan context.Context, 1)
			finished := make(chan struct{})
			h.a.startDiffOpen(func(ctx context.Context) *diffOpenResult {
				started <- ctx
				<-ctx.Done()
				return &diffOpenResult{Canceled: true}
			})
			go func() { h.work[0](); close(finished) }()
			var ctx context.Context
			select {
			case ctx = <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("diff read did not start")
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("diff read lost its timeout")
			}
			if shutdown {
				h.a.CloseTasks()
			} else {
				h.a.cancelPendingDiff()
			}
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("diff read did not observe cancellation")
			}
			if ctx.Err() == nil || len(h.deliveries) != 0 || h.a.diffOpenTask.Pending() {
				t.Fatal("cancelled read retained work or queued UI delivery")
			}
		})
	}
}

func TestDiffOpenHeadlessAndRetry(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	a.startDiffOpen(diffOpenReply("first diff"))
	if a.EditorGroup.DiffWidgetByTab("first diff") == nil {
		t.Fatal("headless diff did not open synchronously")
	}
	a.cancelPendingDiff()
	a.startDiffOpen(diffOpenReply("second diff"))
	if a.EditorGroup.DiffWidgetByTab("second diff") == nil || diffOpenStatus(a) != "" {
		t.Fatal("cancelled task could not open a later diff")
	}
}

func TestDiffOpenFailuresClearLoadingState(t *testing.T) {
	for _, action := range []string{"warning", "panic", "cancelled", "nil"} {
		t.Run(action, func(t *testing.T) {
			h := newDiffOpenTestHarness(t)
			h.a.startDiffOpen(func(context.Context) *diffOpenResult {
				switch action {
				case "warning":
					return &diffOpenResult{Warn: "No line changes"}
				case "panic":
					panic("read failed")
				case "cancelled":
					return &diffOpenResult{Canceled: true}
				default:
					return nil
				}
			})
			h.run(t)()
			if diffOpenStatus(h.a) != "" || h.a.diffOpenTask.Pending() || h.a.EditorGroup.ActiveDiffWidget() != nil {
				t.Fatal("failed read opened a diff or retained loading state")
			}
			if action == "warning" && h.a.Status.Notification != "No line changes" {
				t.Fatal("diff warning was lost")
			}
			if action == "panic" && !strings.Contains(h.a.Status.Notification, "read failed") {
				t.Fatal("recovered panic was not reported")
			}
		})
	}
}
