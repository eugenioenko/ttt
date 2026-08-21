package e2e

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestChangesAndExplorerPanelMenusExposeTreeActions(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()

	h.exec("sidebar.changes")
	h.app.ShowSidebarMoreMenu(4, 3)
	h.redraw()
	h.assertContains("Expand All")
	h.assertContains("Collapse All")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	h.exec("sidebar.explorer")
	h.app.ShowSidebarMoreMenu(4, 3)
	h.redraw()
	h.assertContains("Expand All")
	h.assertContains("Collapse All")
}
