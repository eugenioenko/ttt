package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestChangesAndExplorerThreeDotMenusExposeBulkActions(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()

	h.exec("sidebar.changes")
	h.app.ShowSidebarMoreMenu(4, 3)
	h.redraw()
	h.assertContains("Git Files")
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyRight, tcell.ModNone)
	h.assertContains("Expand All")
	h.assertContains("Collapse All")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	h.exec("sidebar.explorer")
	h.app.ShowSidebarMoreMenu(4, 3)
	h.redraw()
	h.assertContains("Expand All")
	h.assertContains("Collapse All")
}

func TestChangesRightClickAndThreeDotMenusHaveGitFileParity(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	panel := h.app.BuildChangesPanelMenu()
	context := h.app.BuildChangesContextMenu()
	for _, command := range []string{"options.useGitFileTree", "options.useGitFileList", "changes.expandAll", "changes.collapseAll"} {
		panelItem, panelOK := findMenuCommand(panel, command)
		contextItem, contextOK := findMenuCommand(context, command)
		if !panelOK || !contextOK || panelItem.Checked != contextItem.Checked {
			t.Errorf("menu parity for %s: panel=%+v/%v context=%+v/%v", command, panelItem, panelOK, contextItem, contextOK)
		}
	}

	h.exec("sidebar.changes")
	h.app.ShowChangesContextMenu(5, 5)
	h.redraw()
	if _, ok := h.app.Root.Focused.(*ui.ContextMenuWidget); !ok {
		t.Fatalf("right-click menu focus = %T", h.app.Root.Focused)
	}
	h.assertContains("Git Files")
}
