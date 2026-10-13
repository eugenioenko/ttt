package app

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/git"
)

func newBlameTestHarness(t *testing.T) *gitGutterTestHarness {
	t.Helper()
	h := newGitGutterTestHarness(t)
	h.a.Repository.SetDirs([]string{filepath.Dir(h.a.EditorGroup.ActiveFilePath())})
	h.a.Repository.RefreshNow(RepositoryWorktree)
	h.a.blame.debounce = NewTask[blameKey](h.a, asynctask.TaskOptions{
		Delay: blameDelay, Clock: &h.clock,
		Run: func(fn func()) { h.work = append(h.work, fn) },
	})
	h.a.blame.read = NewTask[*git.BlameInfo](h.a, asynctask.TaskOptions{
		Run: func(fn func()) { h.work = append(h.work, fn) },
	})
	return h
}

func blameText(a *App) string {
	for _, segment := range a.Status.LeftSegments() {
		if segment.ID == "blame" {
			return segment.Text
		}
	}
	return ""
}

func settleBlame(t *testing.T, h *gitGutterTestHarness) {
	t.Helper()
	h.clock.advance(blameDelay)
	h.run(t)()
	h.run(t)()
}

func TestBlameKeyDeduplicatesPendingAndCompletedWork(t *testing.T) {
	h := newBlameTestHarness(t)
	a := h.a
	a.SyncBlame()
	h.clock.advance(blameDelay - time.Millisecond)
	a.SyncBlame()
	if len(h.work) != 0 {
		t.Fatal("blame started before the delay elapsed")
	}
	h.clock.advance(time.Millisecond)
	trigger := h.run(t)
	a.SyncBlame()
	if a.blame.read.Pending() {
		t.Fatal("blame read started before UI delivery")
	}
	trigger()
	a.SyncBlame()
	h.run(t)()
	if !strings.Contains(blameText(a), "Test") {
		t.Fatalf("committed line blame = %q", blameText(a))
	}
	a.EditorGroup.Editor.Cursor.Col++
	a.SyncBlame()
	a.Repository.RefreshNow(RepositoryWorktree)
	a.SyncBlame()
	h.clock.advance(blameDelay)
	if len(h.work) != 0 || a.blame.debounce.Pending() || a.blame.read.Pending() {
		t.Fatal("cursor column or unchanged repository refresh restarted completed blame")
	}
}

func TestBlameCurrentLineEditClearsAuthorAndDebounces(t *testing.T) {
	h := newBlameTestHarness(t)
	a := h.a
	a.SyncBlame()
	settleBlame(t, h)
	a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
	a.SyncBlame()
	if blameText(a) != "" {
		t.Fatal("edit retained the committed author")
	}
	h.clock.advance(blameDelay - time.Millisecond)
	a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 1, Rune: 'Y'})
	a.SyncBlame()
	h.clock.advance(blameDelay - time.Millisecond)
	if len(h.work) != 0 {
		t.Fatal("typing restarted blame before the trailing delay")
	}
	h.clock.advance(time.Millisecond)
	h.run(t)()
	h.run(t)()
	if blameText(a) != "" {
		t.Fatalf("uncommitted line blame = %q", blameText(a))
	}
}

func TestBlameDirtyStateRefreshesAfterSave(t *testing.T) {
	h := newBlameTestHarness(t)
	a := h.a
	a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 1, Col: 0, Rune: 'X'})
	a.SyncBlame()
	settleBlame(t, h)
	if !strings.Contains(blameText(a), "Test") {
		t.Fatal("editing another line cleared the committed line's author")
	}
	buf := a.EditorGroup.ActiveBuffer()
	version := buf.Version()
	if !a.EditorGroup.Save() {
		t.Fatal("save failed")
	}
	if buf.Version() != version || buf.Dirty {
		t.Fatal("save changed the version or retained dirty state")
	}
	a.SyncBlame()
	if blameText(a) != "" || !a.blame.debounce.Pending() {
		t.Fatal("dirty-state change did not refresh blame")
	}
	settleBlame(t, h)
	if !strings.Contains(blameText(a), "Test") {
		t.Fatalf("saved committed line blame = %q", blameText(a))
	}
}

func TestBlameRepositoryChangesRefreshSamePosition(t *testing.T) {
	h := newBlameTestHarness(t)
	a := h.a
	a.SyncBlame()
	settleBlame(t, h)
	path := a.EditorGroup.ActiveFilePath()
	repo := filepath.Dir(path)
	if err := os.WriteFile(path, []byte("changed\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTreeGit(t, repo, "-c", "user.name=New Author", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qam", "new author")
	a.Repository.RefreshNow(RepositoryWorktree)
	a.SyncBlame()
	if blameText(a) != "" {
		t.Fatal("repository refresh retained old author")
	}
	settleBlame(t, h)
	if !strings.Contains(blameText(a), "New Author") {
		t.Fatalf("refreshed blame = %q", blameText(a))
	}
}

func TestBlameDropsQueuedWorkAfterEditOrShutdown(t *testing.T) {
	for _, stage := range []string{"debounce", "read"} {
		for _, action := range []string{"edit", "shutdown"} {
			t.Run(stage+"/"+action, func(t *testing.T) {
				h := newBlameTestHarness(t)
				a := h.a
				a.SyncBlame()
				h.clock.advance(blameDelay)
				if stage == "read" {
					h.run(t)()
				}
				stale := h.run(t)
				if action == "shutdown" {
					a.CloseTasks()
				} else {
					a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
					a.SyncBlame()
				}
				stale()
				if len(h.work) != 0 || blameText(a) != "" {
					t.Fatal("stale blame started or applied")
				}
			})
		}
	}
}

func TestBlameTargetChangesDropQueuedRead(t *testing.T) {
	for _, target := range []string{"line", "file", "virtual", "outside repo"} {
		t.Run(target, func(t *testing.T) {
			h := newBlameTestHarness(t)
			a := h.a
			a.SyncBlame()
			h.clock.advance(blameDelay)
			h.run(t)()
			stale := h.run(t)
			switch target {
			case "line":
				a.EditorGroup.Editor.Cursor.Line++
			case "virtual":
				a.EditorGroup.NewFile()
			default:
				dir := filepath.Dir(a.EditorGroup.ActiveFilePath())
				if target == "outside repo" {
					dir = t.TempDir()
				}
				path := filepath.Join(dir, "other.txt")
				if err := os.WriteFile(path, []byte("other\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				a.EditorGroup.OpenFile(path)
			}
			a.SyncBlame()
			stale()
			if blameText(a) != "" || a.blame.read.Pending() {
				t.Fatal("target change retained or applied old blame")
			}
		})
	}
}

func TestBlameDebounceDoesNotCopyBufferOnEachEdit(t *testing.T) {
	h := newBlameTestHarness(t)
	a := h.a
	lines := make([]string, 100_000)
	for i := range lines {
		lines[i] = "line"
	}
	a.EditorGroup.Editor.ExecCommand(&undo.ReplaceLinesCommand{
		OldLines: slices.Clone(a.EditorGroup.ActiveBuffer().Lines), NewLines: lines,
	})
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := range 20 {
		a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: i, Rune: 'X'})
		a.SyncBlame()
	}
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
		t.Fatalf("debouncing allocated %d bytes, want less than one 100k-line snapshot", allocated)
	}
}
