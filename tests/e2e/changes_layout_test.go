package e2e

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestChangesHistoryResizeKeepsMinimumsAndTreeScrolling(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.exec("sidebar.changes")

	cp := h.app.Changes
	items := make([]*widgets.TreeNode, 20)
	for i := range items {
		items[i] = &widgets.TreeNode{ID: fmt.Sprintf("history:%d", i), Label: fmt.Sprintf("commit %02d", i)}
	}
	cp.CommitLog.SetItems(items)
	h.redraw()

	drag := func(fromY, toY int) {
		t.Helper()
		x := cp.Split.GetRect().X + 2
		h.app.Root.HandleEvent(tcell.NewEventMouse(x, fromY, tcell.Button1, tcell.ModNone))
		h.app.Root.HandleEvent(tcell.NewEventMouse(x, toY, tcell.Button1, tcell.ModNone))
		h.app.Root.HandleEvent(tcell.NewEventMouse(x, toY, tcell.ButtonNone, tcell.ModNone))
		h.redraw()
	}

	// Dragging below the panel clamps the history to its title plus three log
	// rows instead of collapsing it. The working tree receives the remainder.
	drag(cp.Split.DividerScreenY(), cp.Split.GetRect().Y+cp.Split.GetRect().H)
	if cp.Split.BottomH != 4 {
		t.Fatalf("history height = %d, want minimum 4", cp.Split.BottomH)
	}
	if got := cp.CommitLog.GetRect().H; got != 3 {
		t.Fatalf("commit log height = %d, want 3 usable rows", got)
	}

	// Dragging above the panel leaves the input, divider, and three working-tree
	// rows rather than allowing history to consume the whole sidebar.
	drag(cp.Split.DividerScreenY(), cp.Split.GetRect().Y-2)
	if h.app.Sidebar.ActivePanel != "changes" {
		t.Fatalf("drag across sidebar tabs switched to %q", h.app.Sidebar.ActivePanel)
	}
	if got := cp.Tree.GetRect().H; got != 3 {
		t.Fatalf("working-tree height = %d, want minimum 3 rows", got)
	}

	// Shrink again, then prove the nested TreeWidget still owns wheel scrolling
	// after repeated split captures. This guards against leaving capture or
	// routing on the divider.
	drag(cp.Split.DividerScreenY(), cp.Split.GetRect().Y+cp.Split.GetRect().H)
	r := cp.CommitLog.GetRect()
	for range 10 {
		h.app.Root.HandleEvent(tcell.NewEventMouse(r.X+1, r.Y+1, tcell.WheelDown, tcell.ModNone))
	}
	h.redraw()
	if cp.CommitLog.ScrollTop() == 0 {
		t.Fatal("commit log did not scroll after resizing")
	}
	h.assertContains("commit 19")
}
