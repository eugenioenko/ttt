package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

func TestGitGutterSkipsIgnoredFiles(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	os.WriteFile(filepath.Join(h.dir, ".gitignore"), []byte("*.log\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	os.WriteFile(filepath.Join(h.dir, "app.log"), []byte("one\ntwo\n"), 0644)
	os.WriteFile(filepath.Join(h.dir, "new.txt"), []byte("one\ntwo\n"), 0644)

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "new.txt"))
	h.app.SyncRepositoryBranch()
	h.app.RequestGitGutterForActiveFile()
	h.redraw()
	changes := h.app.EditorGroup.Editor.LineChanges
	lines := h.app.EditorGroup.ActiveBuffer().Lines
	if len(changes) != len(lines) || slices.ContainsFunc(changes, func(c diff.LineChangeKind) bool { return c != diff.LineAdded }) {
		t.Fatalf("untracked file gutter = %v for %d lines, want every line added", changes, len(lines))
	}

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "app.log"))
	h.app.SyncRepositoryBranch()
	h.app.RequestGitGutterForActiveFile()
	h.redraw()
	if changes := h.app.EditorGroup.Editor.LineChanges; len(changes) != 0 {
		t.Fatalf("ignored file gutter = %v, want no markers", changes)
	}
}
