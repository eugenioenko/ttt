package e2e

import (
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
)

func TestCopyAbsolutePathFromEditor(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	clipboard.DisableSystem()

	h.openFile(filepath.Join(h.dir, "alpha.txt"))
	h.redraw()

	h.exec("file.copyAbsolutePath")

	got := clipboard.Get()
	want := filepath.Join(h.dir, "alpha.txt")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCopyRelativePathFromEditor(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	clipboard.DisableSystem()

	h.openFile(filepath.Join(h.dir, "subdir", "nested.txt"))
	h.redraw()

	h.exec("file.copyRelativePath")

	got := clipboard.Get()
	want := filepath.Join("subdir", "nested.txt")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCopyAbsolutePathFromExplorer(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	clipboard.DisableSystem()

	h.exec("sidebar.explorer")

	var fileIdx int
	for i, node := range h.app.Explorer.Tree.FlatList() {
		if !node.Expandable {
			fileIdx = i
			break
		}
	}
	h.app.Explorer.Tree.SetSelectedIndex(fileIdx)
	h.app.ExplorerContextNode = h.app.Explorer.Tree.FlatList()[fileIdx]
	h.redraw()

	h.exec("explorer.copyAbsolutePath")

	got := clipboard.Get()
	want := h.app.Explorer.Tree.FlatList()[fileIdx].ID
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCopyPathNoFileOpen(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	clipboard.DisableSystem()
	clipboard.Set("")

	h.exec("file.copyAbsolutePath")

	got := clipboard.Get()
	if got != "" {
		t.Errorf("expected empty clipboard when no file open, got %q", got)
	}
}
