package app

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

type gitGutterTestTimer struct {
	fakeRepositoryTimer
	due time.Time
}

type gitGutterTestClock struct {
	now    time.Time
	timers []*gitGutterTestTimer
}

func (c *gitGutterTestClock) Now() time.Time { return c.now }

func (c *gitGutterTestClock) AfterFunc(delay time.Duration, fn func()) asynctask.Timer {
	timer := &gitGutterTestTimer{fakeRepositoryTimer: fakeRepositoryTimer{fn: fn}, due: c.now.Add(delay)}
	c.timers = append(c.timers, timer)
	return timer
}

func (c *gitGutterTestClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
	for _, timer := range c.timers {
		if !timer.stopped && !timer.due.After(c.now) {
			timer.fire()
		}
	}
}

type gitGutterTestHarness struct {
	a        *App
	clock    gitGutterTestClock
	work     []func()
	delivery []func()
}

func newGitGutterTestHarness(t *testing.T) *gitGutterTestHarness {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTreeGit(t, dir, "init", "-q")
	runTreeGit(t, dir, "add", ".")
	runTreeGit(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "initial")
	a := buildTestApp(t, config.DefaultSettings())
	a.EditorGroup.OpenFile(path)
	a.SyncRepositoryBranch()
	h := &gitGutterTestHarness{a: a, clock: gitGutterTestClock{now: time.Unix(0, 0)}}
	a.tasks.post = func(fn func()) error {
		h.delivery = append(h.delivery, fn)
		return nil
	}
	a.gitGutterDebounce = NewTask[struct{}](a, asynctask.TaskOptions{
		Delay: gitGutterDelay,
		Clock: &h.clock,
		Run:   func(fn func()) { h.work = append(h.work, fn) },
	})
	a.gitGutterTask = NewTask[*gitGutterResult](a, asynctask.TaskOptions{
		Run: func(fn func()) { h.work = append(h.work, fn) },
	})
	t.Cleanup(func() {
		a.CloseTasks()
		a.EditorGroup.CloseAllTabs()
	})
	return h
}

func (h *gitGutterTestHarness) run(t *testing.T) func() {
	t.Helper()
	if len(h.work) != 1 {
		t.Fatalf("queued work = %d, want 1", len(h.work))
	}
	fn := h.work[0]
	h.work = nil
	fn()
	if len(h.delivery) != 1 {
		t.Fatalf("queued deliveries = %d, want 1", len(h.delivery))
	}
	fn = h.delivery[0]
	h.delivery = nil
	return fn
}

func TestGitGutterDebouncesLatestBuffer(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a
	a.EditorGroup.Editor.ExecCommand(&undo.InsertLineCommand{Idx: 1, Text: "added"})
	a.ScheduleGitGutter()
	h.clock.advance(499 * time.Millisecond)
	if len(h.work) != 0 {
		t.Fatal("gutter ran before 500ms")
	}
	a.EditorGroup.Editor.ExecCommand(&undo.InsertLineCommand{Idx: 2, Text: "latest"})
	a.ScheduleGitGutter()
	h.clock.advance(499 * time.Millisecond)
	if len(h.work) != 0 {
		t.Fatal("edit did not restart the delay")
	}
	h.clock.advance(time.Millisecond)
	trigger := h.run(t)
	if a.gitGutterTask.Pending() {
		t.Fatal("diff started before the debounce reached the UI goroutine")
	}
	trigger()
	deliver := h.run(t)
	if len(a.EditorGroup.Editor.LineChanges) != 0 || !a.gitGutterTask.Pending() {
		t.Fatal("gutter applied before UI delivery")
	}
	deliver()
	changes := a.EditorGroup.Editor.LineChanges
	if len(changes) < 3 || changes[1] != diff.LineAdded || changes[2] != diff.LineAdded {
		t.Fatalf("latest gutter changes = %v, want added lines 1 and 2", changes)
	}
	if a.gitGutterTask.Pending() || a.gitGutterDebounce.Pending() {
		t.Fatal("gutter pending after delivery")
	}
}

func TestGitGutterSaveFlushesPostCleanupBuffer(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a
	a.EditorGroup.ActiveBuffer().TrimTrailingWhitespace = true
	a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 3, Rune: ' '})
	a.ScheduleGitGutter()
	a.SaveFile()
	data, err := os.ReadFile(a.EditorGroup.ActiveFilePath())
	if err != nil || string(data) != "one\ntwo\n" {
		t.Fatalf("saved buffer = %q, err = %v", data, err)
	}
	h.run(t)()
	h.run(t)()
	if len(a.EditorGroup.Editor.LineChanges) != len(a.EditorGroup.ActiveBuffer().Lines) {
		t.Fatalf("save gutter = %v, want one entry per buffer line", a.EditorGroup.Editor.LineChanges)
	}
	if slices.Contains(a.EditorGroup.Editor.LineChanges, diff.LineModified) {
		t.Fatalf("gutter used the pre-save whitespace: %v", a.EditorGroup.Editor.LineChanges)
	}
	h.clock.advance(time.Second)
	if len(h.work) != 0 {
		t.Fatal("save left a redundant delayed refresh")
	}
}

func TestGitGutterDropsQueuedResultOnEdit(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a
	a.RequestGitGutterForActiveFile()
	h.run(t)()
	stale := h.run(t)
	a.EditorGroup.Editor.ExecCommand(&undo.InsertLineCommand{Idx: 1, Text: "latest"})
	a.ScheduleGitGutter()
	stale()
	if len(a.EditorGroup.Editor.LineChanges) != 0 {
		t.Fatal("superseded queued gutter result applied")
	}
	h.clock.advance(500 * time.Millisecond)
	h.run(t)()
	h.run(t)()
	changes := a.EditorGroup.Editor.LineChanges
	if len(changes) < 2 || changes[1] != diff.LineAdded {
		t.Fatalf("gutter changes = %v, want added line 1", changes)
	}
}

func TestGitGutterStopsWhenTargetCannotUseGutter(t *testing.T) {
	for _, target := range []string{"virtual", "outside repo", "disabled"} {
		t.Run(target, func(t *testing.T) {
			h := newGitGutterTestHarness(t)
			a := h.a
			a.RequestGitGutterForActiveFile()
			h.run(t)()
			stale := h.run(t)
			switch target {
			case "virtual":
				a.EditorGroup.NewFile()
			case "outside repo":
				path := filepath.Join(t.TempDir(), "plain.txt")
				if err := os.WriteFile(path, []byte("plain\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				a.EditorGroup.OpenFile(path)
			case "disabled":
				enabled := false
				a.Settings.Editor.GitGutter = &enabled
				a.ApplySettings(*a.Settings)
			}
			if target != "disabled" {
				a.RequestGitGutterForActiveFile()
			}
			stale()
			if a.gitGutterTask.Pending() || a.gitGutterDebounce.Pending() || len(a.EditorGroup.Editor.LineChanges) != 0 {
				t.Fatal("invalid target retained or applied gutter work")
			}
		})
	}
}

func TestGitGutterDebounceDoesNotCopyBufferOnEachEdit(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a
	lines := make([]string, 100_000)
	for i := range lines {
		lines[i] = "line"
	}
	a.EditorGroup.Editor.ExecCommand(&undo.ReplaceLinesCommand{
		OldLines: slices.Clone(a.EditorGroup.ActiveBuffer().Lines),
		NewLines: lines,
	})
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range 20 {
		a.ScheduleGitGutter()
	}
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("20 debounce resets allocated %d bytes", allocated)
	if allocated > 1<<20 {
		t.Fatalf("debouncing allocated %d bytes, want less than one 100k-line snapshot", allocated)
	}
}

func TestGitGutterShutdownDropsQueuedDelivery(t *testing.T) {
	for _, stage := range []string{"debounce", "diff"} {
		t.Run(stage, func(t *testing.T) {
			h := newGitGutterTestHarness(t)
			a := h.a
			a.RequestGitGutterForActiveFile()
			if stage == "diff" {
				h.run(t)()
			}
			stale := h.run(t)
			a.CloseTasks()
			stale()
			if len(h.work) != 0 || a.gitGutterTask.Pending() || a.gitGutterDebounce.Pending() || len(a.EditorGroup.Editor.LineChanges) != 0 {
				t.Fatal("shutdown started or applied gutter work")
			}
		})
	}
}

func TestGitGutterDropsQueuedDebounceOnEdit(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a
	a.ScheduleGitGutter()
	h.clock.advance(gitGutterDelay)
	stale := h.run(t)
	a.EditorGroup.Editor.ExecCommand(&undo.InsertLineCommand{Idx: 1, Text: "latest"})
	a.ScheduleGitGutter()
	stale()
	if len(h.work) != 0 || a.gitGutterTask.Pending() {
		t.Fatal("superseded queued debounce started a diff")
	}
	h.clock.advance(gitGutterDelay)
	h.run(t)()
	h.run(t)()
	changes := a.EditorGroup.Editor.LineChanges
	if len(changes) < 2 || changes[1] != diff.LineAdded {
		t.Fatalf("gutter changes = %v, want added line 1", changes)
	}
}
