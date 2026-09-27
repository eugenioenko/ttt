package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestSidebarDragPersistsOnlyOnRelease(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	mouse := func(x int, btn tcell.ButtonMask) {
		h.app.Root.HandleEvent(tcell.NewEventMouse(x, 5, btn, tcell.ModNone))
	}

	mouse(h.app.SplitPanel.DividerScreenX(), tcell.Button1)
	for _, x := range []int{25, 12, 5, 0} {
		mouse(x, tcell.Button1)
		if got := config.LoadState(); got.SidebarHidden || got.SidebarWidth != 0 {
			t.Fatalf("state saved mid-drag at x=%d: %+v", x, got)
		}
	}
	if h.app.Sidebar.Visible {
		t.Fatal("dragging to the edge should hide the sidebar")
	}
	mouse(0, tcell.ButtonNone)
	if got := config.LoadState(); !got.SidebarHidden || (got.SidebarWidth != 0 && got.SidebarWidth < ui.MinSidebarWidth) {
		t.Fatalf("state after drag-close = %+v, want hidden with no width below the minimum", got)
	}

	mouse(0, tcell.Button1)
	mouse(20, tcell.Button1)
	mouse(20, tcell.ButtonNone)
	if got := config.LoadState(); got.SidebarHidden || got.SidebarWidth != 19 {
		t.Fatalf("state after drag-open = %+v, want visible with width 19", got)
	}
}
