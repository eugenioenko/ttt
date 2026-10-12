package e2e

import (
	"os"
	"path/filepath"
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
	awaitGitGutter(t, h)
	changes := h.app.EditorGroup.Editor.LineChanges
	if len(changes) == 0 || changes[0] != diff.LineAdded {
		t.Fatalf("untracked file gutter = %v, want every line added", changes)
	}

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "app.log"))
	h.app.SyncRepositoryBranch()
	h.app.RequestGitGutterForActiveFile()
	awaitGitGutter(t, h)
	if changes := h.app.EditorGroup.Editor.LineChanges; len(changes) != 0 {
		t.Fatalf("ignored file gutter = %v, want no markers", changes)
	}
}
