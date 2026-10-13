package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func TestOpenFileComputesFoldsAfterFirstFrame(t *testing.T) {
	var runs, deliveries []func()
	g := newRenameGroup(t, filepath.Join(t.TempDir(), "a.go"), "func a() {\n\tx := 1\n}\n")
	g.FoldTask = asynctask.New[[]fold.Range](asynctask.TaskOptions{Run: func(fn func()) { runs = append(runs, fn) }},
		func(fn func()) error { deliveries = append(deliveries, fn); return nil })
	g.OpenFile(g.ActiveFilePath())
	finish := func() {
		t.Helper()
		if len(runs) != 1 {
			t.Fatalf("%d fold computations scheduled, want 1", len(runs))
		}
		runs[0]()
		runs = nil
		for _, deliver := range deliveries {
			deliver()
		}
		deliveries = nil
	}

	if !g.Editor.Folds.Pending() {
		t.Fatal("folds computed while opening, want them after the first frame")
	}
	g.scheduleFolds()
	g.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 1, Col: 0, Rune: ' '})
	finish()
	if g.Editor.Folds.Pending() || g.Editor.Folds.FoldAt(0) == nil {
		t.Fatal("folds not applied after typing on a line, want them applied")
	}
}

func TestBackgroundFoldsYieldToLineCountEdit(t *testing.T) {
	var runs, deliveries []func()
	g := newRenameGroup(t, filepath.Join(t.TempDir(), "a.go"), "func a() {\n\tx := 1\n}\n")
	g.FoldTask = asynctask.New[[]fold.Range](asynctask.TaskOptions{Run: func(fn func()) { runs = append(runs, fn) }},
		func(fn func()) error { deliveries = append(deliveries, fn); return nil })
	g.OpenFile(g.ActiveFilePath())

	g.scheduleFolds()
	g.Editor.ExecCommand(&undo.InsertLineCommand{Idx: 0, Text: "// header"})
	runs[0]()
	deliveries[0]()
	if g.Editor.Folds.FoldAt(1) == nil || g.Editor.Folds.FoldAt(0) != nil {
		t.Fatal("background folds for the old line count replaced the ones the edit computed")
	}
}

func TestWrappedPaneDefersFolds(t *testing.T) {
	scheduled := 0
	g := newRenameGroup(t, filepath.Join(t.TempDir(), "a.go"), "func a() {\n\tx := 1\n}\n")
	g.FoldTask = asynctask.New[[]fold.Range](asynctask.TaskOptions{Run: func(func()) { scheduled++ }},
		func(func()) error { return nil })

	g.Editor.WordWrap = true
	g.scheduleFolds()
	if scheduled != 0 {
		t.Fatal("folds computed for a wrapped pane, which cannot fold")
	}
	g.Editor.WordWrap = false
	g.scheduleFolds()
	if scheduled != 1 {
		t.Fatalf("%d fold computations after turning wrap off, want 1", scheduled)
	}
}

func newLargeFoldGroup(t *testing.T) *EditorGroupWidget {
	t.Helper()
	g := newRenameGroup(t, filepath.Join(t.TempDir(), "a.go"), strings.Repeat("func a() {\n\tx := 1\n}\n", asyncFoldLines/3+1))
	g.Editor.Folds.Resolve()
	return g
}

func TestLargeFileRecomputesFoldsOnceEditsPause(t *testing.T) {
	g := newLargeFoldGroup(t)
	var delays, delayDeliveries, runs, deliveries []func()
	g.FoldDelay = asynctask.New[struct{}](asynctask.TaskOptions{Run: func(fn func()) { delays = append(delays, fn) }},
		func(fn func()) error { delayDeliveries = append(delayDeliveries, fn); return nil })
	g.FoldTask = asynctask.New[[]fold.Range](asynctask.TaskOptions{Run: func(fn func()) { runs = append(runs, fn) }},
		func(fn func()) error { deliveries = append(deliveries, fn); return nil })

	for range 2 {
		g.Editor.ExecCommand(&undo.InsertLineCommand{Idx: 0, Text: ""})
		g.scheduleFolds()
	}
	if !g.Editor.Folds.Stale() || g.Editor.Folds.FoldAt(0) == nil {
		t.Fatal("an edit to a large file recomputed folds on the keystroke, want the previous ranges kept")
	}
	if len(runs) != 0 {
		t.Fatal("folds computed before the edits paused")
	}
	delays[len(delays)-1]()
	for _, deliver := range delayDeliveries {
		deliver()
	}
	if len(runs) != 1 {
		t.Fatalf("%d fold computations after the pause, want 1", len(runs))
	}
	runs[0]()
	for _, deliver := range deliveries {
		deliver()
	}
	if g.Editor.Folds.Pending() || g.Editor.Folds.FoldAt(2) == nil || g.Editor.Folds.FoldAt(0) != nil {
		t.Fatal("recomputed folds not applied for the shifted lines")
	}
}

func TestLargeFileWithCollapsedFoldRecomputesAtOnce(t *testing.T) {
	g := newLargeFoldGroup(t)
	g.FoldDelay = asynctask.New[struct{}](asynctask.TaskOptions{Run: func(func()) { t.Fatal("collapsed folds waited for the delay") }},
		func(func()) error { return nil })
	g.Editor.Folds.Collapse(3)

	g.Editor.ExecCommand(&undo.InsertLineCommand{Idx: 0, Text: ""})
	if g.Editor.Folds.Pending() || g.Editor.Folds.FoldAt(4) == nil {
		t.Fatal("folds not recomputed on the edit while one was collapsed")
	}
}
