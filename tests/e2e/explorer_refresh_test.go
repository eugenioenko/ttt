package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// The explorer.refresh command existed but had no key binding, so a file
// created outside the editor could only be surfaced through the context menu
// or the command palette. `r` matches the changes panel.
func TestExplorerRefreshKeyPicksUpNewFile(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.explorer")
	h.app.Explorer.Tree.SetSelectedIndex(0)
	h.redraw()

	before := h.app.Explorer.Tree.ItemCount()
	if err := os.WriteFile(filepath.Join(h.dir, "created-outside.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	h.pressRune('r')
	h.redraw()

	if got := h.app.Explorer.Tree.ItemCount(); got <= before {
		t.Fatalf("expected the tree to grow after refresh: had %d, got %d", before, got)
	}
	h.assertContains("created-outside.txt")
}

// Refresh must not eat the key that moves the selection, so `r` sits alongside
// the tree's own j/k/h/l navigation rather than replacing any of it.
func TestExplorerRefreshKeyKeepsNavigation(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.explorer")
	if h.app.Explorer.Tree.ItemCount() < 3 {
		t.Skipf("expected at least 3 explorer items, got %d", h.app.Explorer.Tree.ItemCount())
	}
	h.app.Explorer.Tree.SetSelectedIndex(0)

	h.pressRune('j')
	if got := h.app.Explorer.Tree.SelectedIndex(); got != 1 {
		t.Fatalf("j should still move down: got index %d, want 1", got)
	}

	h.pressRune('r')
	if got := h.app.Explorer.Tree.SelectedIndex(); got != 1 {
		t.Fatalf("refresh should not move the selection: got index %d, want 1", got)
	}
}

// The shortcut is only useful if it is discoverable from the panel's help.
func TestExplorerHelpListsRefreshKey(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.explorer")
	h.exec("explorer.help")
	h.redraw()

	h.assertContains("Refresh explorer")
}
