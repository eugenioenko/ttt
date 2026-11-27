package e2e

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestInactiveSplitTabContextMenuTargetsClickedTab(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()

	left := h.app.EditorGroup
	h.openFile(filepath.Join(h.dir, "alpha.txt"))
	h.exec("editor.split")
	right := h.app.EditorGroup
	h.openFile(filepath.Join(h.dir, "beta.txt"))
	h.app.Root.SetFocus(left)
	h.redraw()

	r := right.TabBar.GetRect()
	h.rightClick(r.X+4, r.Y+1)
	menu, ok := h.app.Root.TopOverlayWidget().(*ui.ContextMenuWidget)
	if !ok {
		t.Fatal("right-click did not open a menu for the inactive group's tab")
	}
	menu.Selected = slices.IndexFunc(menu.Items, func(item ui.ContextMenuItem) bool {
		return item.Command == "tab.pin"
	})
	if menu.Selected < 0 {
		t.Fatal("tab menu did not contain the Pin command")
	}
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	if !right.IsActiveTabPinned() || left.IsActiveTabPinned() || h.app.Root.Focused != right {
		t.Fatal("menu did not pin and focus the tab in the clicked group")
	}
}
