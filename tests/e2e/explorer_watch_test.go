package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gdamore/tcell/v3"
)

// The explorer must re-render when the filesystem underneath it changes, not
// only when the user hits `r`. HandleExplorerDirChanged is what the file
// watcher posts to the event loop.
func TestExplorerReloadsOnDirChange(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.explorer")
	h.redraw()

	before := h.app.Explorer.Tree.ItemCount()
	if err := os.WriteFile(filepath.Join(h.dir, "watched-new.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	h.app.HandleExplorerDirChanged(h.dir)
	h.redraw()

	if got := h.app.Explorer.Tree.ItemCount(); got <= before {
		t.Fatalf("expected the tree to grow after a dir change: had %d, got %d", before, got)
	}
	h.assertContains("watched-new.txt")
}

// WatchedDirs is the set the watcher subscribes to: the roots plus every
// expanded folder, and nothing that is collapsed.
func TestExplorerWatchedDirsTracksExpandedFolders(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.explorer")
	h.redraw()

	dirs := h.app.Explorer.WatchedDirs()
	if !slices.Contains(dirs, h.dir) {
		t.Fatalf("expected root %q in watched dirs, got %v", h.dir, dirs)
	}
	sub := filepath.Join(h.dir, "subdir")
	if slices.Contains(dirs, sub) {
		t.Fatalf("collapsed subdir should not be watched yet, got %v", dirs)
	}

	subIdx := -1
	for i, n := range h.app.Explorer.Tree.FlatList() {
		if n.ID == sub {
			subIdx = i
			break
		}
	}
	if subIdx < 0 {
		t.Fatalf("subdir node not found in tree")
	}
	h.app.Explorer.Tree.SetSelectedIndex(subIdx)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.redraw()

	if dirs := h.app.Explorer.WatchedDirs(); !slices.Contains(dirs, sub) {
		t.Fatalf("expected expanded subdir %q in watched dirs, got %v", sub, dirs)
	}
}

// Switching away to another sidebar tab and back must show files that appeared
// while the explorer was hidden.
func TestExplorerReloadsWhenSwitchingBackToTab(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.explorer")
	h.redraw()

	h.app.Sidebar.SetActivePanel("search")
	h.redraw()

	if err := os.WriteFile(filepath.Join(h.dir, "while-hidden.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	h.app.Sidebar.SetActivePanel("explorer")
	h.redraw()

	h.assertContains("while-hidden.txt")
}
