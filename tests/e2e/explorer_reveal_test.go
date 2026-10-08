package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

func TestExplorerRevealActiveFileSelectsNestedFileAndKeepsEditorFocus(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.search")
	nested := filepath.Join(h.dir, "subdir", "nested.txt")
	h.app.EditorGroup.OpenFile(nested)
	h.app.Root.SetFocus(h.app.EditorGroup)

	h.exec("explorer.revealActiveFile")

	if h.app.Sidebar.ActivePanel != "explorer" {
		t.Errorf("active panel = %q, want explorer", h.app.Sidebar.ActivePanel)
	}
	if selected := h.app.Explorer.Tree.Selected(); selected == nil || selected.ID != nested {
		t.Fatalf("selected %+v, want %s", selected, nested)
	}
	if h.app.Root.Focused != h.app.EditorGroup {
		t.Errorf("focus = %T, want the editor", h.app.Root.Focused)
	}
	h.assertContains("nested.txt")
}

func TestExplorerRevealActiveFileScrollsRowIntoView(t *testing.T) {
	h := newTestHarness(t, 80, 12)
	defer h.stop()

	deep := filepath.Join(h.dir, "zz-last")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 30 {
		if err := os.MkdirAll(filepath.Join(h.dir, fmt.Sprintf("dir-%02d", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(deep, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.exec("explorer.refresh")
	h.app.EditorGroup.OpenFile(target)
	h.app.Root.SetFocus(h.app.EditorGroup)

	h.exec("explorer.revealActiveFile")

	if h.app.Explorer.Tree.ScrollTop() == 0 {
		t.Fatal("tree did not scroll to the revealed row")
	}
	h.assertContains("target.txt")
}

func TestExplorerRevealActiveFileWithoutFileLeavesTree(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	before := h.app.Explorer.Tree.ItemCount()
	h.exec("explorer.revealActiveFile")

	if h.app.Explorer.Tree.ItemCount() != before {
		t.Errorf("tree changed without an active file: %d -> %d items", before, h.app.Explorer.Tree.ItemCount())
	}
}

func TestExplorerRevealActiveFileIgnoresDiffTab(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	nested := filepath.Join(h.dir, "subdir", "nested.txt")
	h.app.EditorGroup.OpenDiff(nested, diff.FileDiff{}, []string{"old"}, []string{"nested"}, true)
	h.redraw()
	before := h.app.Explorer.Tree.ItemCount()
	selectedBefore := h.app.Explorer.Tree.Selected()

	h.exec("explorer.revealActiveFile")

	if h.app.Explorer.Tree.ItemCount() != before {
		t.Errorf("diff tab expanded the tree: %d -> %d items", before, h.app.Explorer.Tree.ItemCount())
	}
	if selected := h.app.Explorer.Tree.Selected(); selected != selectedBefore {
		t.Errorf("diff tab moved the selection from %+v to %+v", selectedBefore, selected)
	}
}
