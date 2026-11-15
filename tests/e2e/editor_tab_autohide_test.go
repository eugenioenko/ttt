package e2e

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestAutoHideTabsRevealEachSplitAndCollapseOutside(t *testing.T) {
	h := newTestHarness(t, 140, 40)
	defer h.stop()
	s := *h.app.Settings
	s.Editor.TabBarStyle = "auto-hide"
	h.app.ApplySettings(s)
	h.exec("editor.split")
	h.exec("editor.splitDown")
	groups := h.app.EditorSplit.Groups()
	move := func(x, y int) {
		h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, 0))
		h.redraw()
	}
	for _, group := range groups {
		if group.TabBar.GetRect().H != 1 {
			t.Fatal("auto-hide did not start with a divider")
		}
	}
	for _, group := range groups {
		r := group.TabBar.GetRect()
		move(r.X+5, r.Y)
		if group.TabBar.GetRect().H != 3 || group.ContentRect().Y != r.Y+3 {
			t.Fatal("hovering the top edge did not reveal the extended tabs")
		}
		move(r.X+5, r.Y+2)
		if group.TabBar.GetRect().H != 3 {
			t.Fatal("moving within the revealed tabs collapsed them")
		}
		for _, other := range groups {
			if other != group && other.TabBar.GetRect().H != 1 {
				t.Fatal("hovering one group revealed or retained another group's tabs")
			}
		}
	}
	move(0, 0)
	for _, group := range groups {
		if group.TabBar.GetRect().H != 1 || group.ContentRect().Y != group.GetRect().Y+1 {
			t.Fatal("moving outside the editors did not collapse their headers")
		}
	}
}

func TestAutoHideTabsStayOpenForMenuAndDrag(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	s := *h.app.Settings
	s.Editor.TabBarStyle = "auto-hide"
	h.app.ApplySettings(s)
	h.redraw()
	group := h.app.EditorGroup
	r := group.TabBar.GetRect()
	move := func(x, y int, buttons tcell.ButtonMask) {
		h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, buttons, 0))
		h.redraw()
	}
	move(r.X+5, r.Y, tcell.ButtonNone)
	more := group.TabBar.MoreButton.GetRect()
	h.click(more.X+1, more.Y)
	if !h.app.Root.HasOverlay() {
		t.Fatal("tab menu did not open")
	}
	move(more.X, more.Y+4, tcell.ButtonNone)
	if group.TabBar.GetRect().H != 3 {
		t.Fatal("using a tab menu collapsed the header underneath it")
	}
	h.pressKey(tcell.KeyEscape, 0)
	move(0, 0, tcell.ButtonNone)
	if group.TabBar.GetRect().H != 1 {
		t.Fatal("header did not collapse after leaving the closed menu")
	}
	move(r.X+5, r.Y, tcell.ButtonNone)
	move(r.X+5, r.Y+1, tcell.Button1)
	move(r.X+20, r.Y+8, tcell.Button1)
	if group.TabBar.GetRect().H != 3 {
		t.Fatal("dragging a tab collapsed the source header")
	}
	move(r.X+20, r.Y+8, tcell.ButtonNone)
	if group.TabBar.GetRect().H != 1 {
		t.Fatal("header remained open after release without another mouse move")
	}
}
