package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestExplorerContextMenuOffersDuplicateForFilesAndFolders(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	h.exec("sidebar.explorer")

	for _, tc := range []struct {
		name  string
		label string
	}{
		{name: "file", label: "alpha.txt"},
		{name: "folder", label: "subdir"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var node *widgets.TreeNode
			for _, n := range h.app.Explorer.Tree.FlatList() {
				if n.Label == tc.label {
					node = n
					break
				}
			}
			if node == nil {
				t.Fatalf("explorer node %q not found", tc.label)
			}

			h.app.Explorer.OnRightClick(node, 10, 5)
			h.redraw()

			menu, ok := h.app.Root.Focused.(*ui.ContextMenuWidget)
			if !ok {
				t.Fatalf("context menu did not take focus: %T", h.app.Root.Focused)
			}
			item, found := findMenuCommand(menu.Items, "explorer.duplicate")
			if !found {
				t.Fatal("context menu has no explorer.duplicate item")
			}
			if item.Label != "Duplicate" {
				t.Errorf("duplicate label = %q, want %q", item.Label, "Duplicate")
			}
			if item.Disabled {
				t.Error("duplicate must be enabled for both files and folders")
			}
			menu.OnDismiss()
		})
	}
}
