package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

// renameVia drives the explorer rename dialog for the node at path, replacing
// its name with newName.
func (h *testHarness) renameVia(path, newName string) {
	h.t.Helper()
	h.app.ExplorerContextNode = &widgets.TreeNode{ID: path}
	h.exec("explorer.rename")
	h.redraw()

	// The dialog is pre-filled with the current basename and places no
	// selection, so clear it before typing the replacement.
	for range filepath.Base(path) {
		h.pressKey(tcell.KeyBackspace, tcell.ModNone)
	}
	h.typeText(newName)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.redraw()
}

// Renaming a file in the explorer used to leave the open tab pointing at the
// old path, so Ctrl+S wrote the buffer back out under the name the file had
// been renamed away from, leaving a ghost duplicate on disk. Issue #284.
func TestExplorerRenameUpdatesOpenTabAndSavesToNewPath(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	old := filepath.Join(h.dir, "alpha.txt")
	h.app.EditorGroup.OpenFile(old)
	h.redraw()

	h.renameVia(old, "renamed.txt")

	newPath := filepath.Join(h.dir, "renamed.txt")
	if got := h.app.EditorGroup.ActiveFilePath(); got != newPath {
		t.Fatalf("tab still points at the old path: got %q, want %q", got, newPath)
	}

	h.app.EditorGroup.ActiveBuffer().Lines = []string{"edited"}
	h.app.EditorGroup.ActiveBuffer().Dirty = true
	h.exec("file.save")

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("save recreated %s as a ghost duplicate", old)
	}
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read renamed file: %v", err)
	}
	if got := string(data); got != "edited\n" && got != "edited" {
		t.Fatalf("renamed file contents: got %q, want edited", got)
	}
}

// A folder rename has to carry every tab beneath it, not just an exact match.
func TestExplorerRenameFolderUpdatesNestedTab(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	nested := filepath.Join(h.dir, "subdir", "nested.txt")
	h.app.EditorGroup.OpenFile(nested)
	h.redraw()

	h.renameVia(filepath.Join(h.dir, "subdir"), "renamed-dir")

	want := filepath.Join(h.dir, "renamed-dir", "nested.txt")
	if got := h.app.EditorGroup.ActiveFilePath(); got != want {
		t.Fatalf("nested tab path: got %q, want %q", got, want)
	}
}

// A file that is not open must still rename cleanly, and must not disturb the
// tab that is open.
func TestExplorerRenameLeavesOtherTabsAlone(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	openPath := filepath.Join(h.dir, "alpha.txt")
	h.app.EditorGroup.OpenFile(openPath)
	h.redraw()

	h.renameVia(filepath.Join(h.dir, "beta.txt"), "beta-renamed.txt")

	if got := h.app.EditorGroup.ActiveFilePath(); got != openPath {
		t.Fatalf("unrelated rename moved the open tab: got %q, want %q", got, openPath)
	}
	if _, err := os.Stat(filepath.Join(h.dir, "beta-renamed.txt")); err != nil {
		t.Fatalf("expected beta-renamed.txt on disk: %v", err)
	}
}
