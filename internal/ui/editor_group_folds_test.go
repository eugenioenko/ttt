package ui

import (
	"path/filepath"
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
	if !g.Editor.Folds.Pending() {
		t.Fatal("folds computed before an edit were applied, want them recomputed")
	}

	g.scheduleFolds()
	finish()
	if g.Editor.Folds.Pending() || g.Editor.Folds.FoldAt(0) == nil {
		t.Fatal("folds not applied after the background computation")
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
