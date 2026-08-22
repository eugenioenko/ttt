package e2e

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestSidebarHeaderDragReordersAndPersists(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	h.exec("sidebar.changes")

	tabsY := h.app.Sidebar.Tabs.GetRect().Y
	row := h.screenRow(tabsY)
	changesX := displayColumnOf(row, "Changes")
	explorerX := displayColumnOf(row, "Explore")
	if changesX < 0 || explorerX < 0 {
		t.Fatalf("tab labels not visible in %q", row)
	}

	if got := h.app.Root.HandleEvent(tcell.NewEventMouse(changesX+2, tabsY, tcell.Button1, 0)); got != ui.EventConsumed {
		t.Fatalf("mouse down result = %v, want EventConsumed", got)
	}
	if got := h.app.Sidebar.ActivePanel; got != "changes" {
		t.Fatalf("pressed source = %q, want changes", got)
	}
	dropX := h.app.Sidebar.Tabs.GetRect().X
	h.app.Root.HandleEvent(tcell.NewEventMouse(dropX, tabsY, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(dropX, tabsY, tcell.ButtonNone, 0))
	h.redraw()

	want := []string{"changes", "explorer", "search", "outline"}
	if got := h.app.Sidebar.PanelIDs(); !slices.Equal(got, want) {
		t.Fatalf("panel order = %v, want %v", got, want)
	}
	if h.app.Sidebar.ActivePanel != "changes" {
		t.Fatalf("active panel = %q, want changes", h.app.Sidebar.ActivePanel)
	}
	if got := config.LoadSettings().Sidebar.PanelOrder; !slices.Equal(got, want) {
		t.Fatalf("saved panel order = %v, want %v", got, want)
	}

	h.exec("sidebar.movePanelRight")
	want = []string{"explorer", "changes", "search", "outline"}
	if got := h.app.Sidebar.PanelIDs(); !slices.Equal(got, want) {
		t.Fatalf("command panel order = %v, want %v", got, want)
	}
}

func TestSidebarRemovedDragSourceDoesNotRetargetSameIndex(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	h.redraw()

	tabsY := h.app.Sidebar.Tabs.GetRect().Y
	searchX := displayColumnOf(h.screenRow(tabsY), "Find")
	if searchX < 0 {
		t.Fatal("Find panel tab is not visible")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(searchX+2, tabsY, tcell.Button1, 0))
	if got := h.app.Sidebar.ActivePanel; got != "search" {
		t.Fatalf("pressed source = %q, want search", got)
	}

	h.app.Sidebar.RemovePanel("search")
	postRemoval := slices.Clone(h.app.Sidebar.PanelIDs())
	if slices.Contains(postRemoval, "search") || len(postRemoval) < 2 || postRemoval[1] != "changes" {
		t.Fatalf("test setup did not replace numeric index 1 with changes: %v", postRemoval)
	}
	if h.app.Root.PointerCaptureActive() || h.app.Sidebar.Tabs.PointerGestureActive() {
		t.Fatal("removing search retained pointer capture")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(searchX+15, tabsY, tcell.ButtonNone, 0))
	if got := h.app.Sidebar.PanelIDs(); !slices.Equal(got, postRemoval) {
		t.Fatalf("release reordered search's replacement: got %v, want %v", got, postRemoval)
	}
}

func TestHiddenSidebarCancelsGestureAndReleaseIsNoOp(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	h.redraw()

	tabsY := h.app.Sidebar.Tabs.GetRect().Y
	changesX := displayColumnOf(h.screenRow(tabsY), "Changes")
	if changesX < 0 {
		t.Fatal("Changes panel tab is not visible")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(changesX+2, tabsY, tcell.Button1, 0))
	if got := h.app.Sidebar.ActivePanel; got != "changes" {
		t.Fatalf("pressed source = %q, want changes", got)
	}
	before := slices.Clone(h.app.Sidebar.PanelIDs())

	h.app.HideSidebar()
	h.redraw()
	if h.app.Root.PointerCaptureActive() || h.app.Sidebar.Tabs.PointerGestureActive() {
		t.Fatal("hidden sidebar retained pointer capture")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(changesX+15, tabsY, tcell.ButtonNone, 0))
	if got := h.app.Sidebar.PanelIDs(); !slices.Equal(got, before) {
		t.Fatalf("release after hide reordered panels: got %v, want %v", got, before)
	}
}

func TestSidebarPendingTabDragOwnsCrossDividerRelease(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	h.exec("sidebar.changes")

	tabsY := h.app.Sidebar.Tabs.GetRect().Y
	row := h.screenRow(tabsY)
	changesX := displayColumnOf(row, "Changes")
	if changesX < 0 {
		t.Fatalf("Changes tab not visible in %q", row)
	}

	h.app.Root.HandleEvent(tcell.NewEventMouse(changesX+2, tabsY, tcell.Button1, 0))
	crossX := h.app.SplitPanel.DividerScreenX() + 2
	h.app.Root.HandleEvent(tcell.NewEventMouse(crossX, tabsY, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(crossX, tabsY, tcell.ButtonNone, 0))
	h.redraw()

	if h.app.Sidebar.Tabs.PointerGestureActive() {
		t.Fatal("cross-divider release left a stale sidebar tab gesture")
	}
	afterRelease := slices.Clone(h.app.Sidebar.PanelIDs())
	row = h.screenRow(tabsY)
	explorerX := displayColumnOf(row, "Explore")
	if explorerX < 0 {
		t.Fatalf("Explore tab not visible after release in %q", row)
	}
	h.click(explorerX+2, tabsY)

	if h.app.Sidebar.ActivePanel != "explorer" {
		t.Fatalf("active panel = %q, want explorer", h.app.Sidebar.ActivePanel)
	}
	if got := h.app.Sidebar.PanelIDs(); !slices.Equal(got, afterRelease) {
		t.Fatalf("ordinary Explore click reordered panels: got %v, want %v", got, afterRelease)
	}
}

func TestSidebarDuplicatePluginAddDuringDragCannotRetargetRelease(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	original := newEmptyWidget()
	replacement := newEmptyWidget()
	h.app.Sidebar.SetPanelOrder([]string{"changes", "plugin.late", "explorer", "search", "outline"})
	h.app.Sidebar.AddPanel("plugin.late", "Late", original)
	h.app.Sidebar.SetActivePanel("plugin.late")
	h.redraw()

	tabsY := h.app.Sidebar.Tabs.GetRect().Y
	lateX := displayColumnOf(h.screenRow(tabsY), "Late")
	if lateX < 0 {
		t.Fatal("late plugin tab is not visible")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(lateX+2, tabsY, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(lateX+8, tabsY, tcell.Button1, 0))
	if !h.app.Root.PointerCaptureActive() || !h.app.Sidebar.Tabs.PointerGestureActive() {
		t.Fatal("test setup did not establish a captured plugin tab drag")
	}
	h.app.Sidebar.AddPanel("plugin.late", "Replacement", replacement)

	wantWithPlugin := []string{"changes", "plugin.late", "explorer", "search", "outline"}
	if got := h.app.Sidebar.PanelIDs(); !slices.Equal(got, wantWithPlugin) {
		t.Fatalf("duplicate add panel order = %v, want %v", got, wantWithPlugin)
	}
	if h.app.Sidebar.ActivePanel != "plugin.late" || h.app.Sidebar.ActiveWidget() != original {
		t.Fatal("duplicate add replaced the active plugin surface")
	}

	h.app.Sidebar.RemovePanel("plugin.late")
	afterRemoval := slices.Clone(h.app.Sidebar.PanelIDs())
	if h.app.Root.PointerCaptureActive() || h.app.Sidebar.Tabs.PointerGestureActive() {
		t.Fatal("removing duplicate-add source retained pointer capture")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(lateX+15, tabsY, tcell.ButtonNone, 0))
	if got := h.app.Sidebar.PanelIDs(); !slices.Equal(got, afterRemoval) {
		t.Fatalf("release retargeted surviving panel: got %v, want %v", got, afterRemoval)
	}

	h.app.Sidebar.AddPanel("plugin.late", "Reloaded", replacement)
	if got := h.app.Sidebar.PanelIDs(); !slices.Equal(got, wantWithPlugin) {
		t.Fatalf("reloaded plugin order = %v, want %v", got, wantWithPlugin)
	}
	h.app.Sidebar.SetActivePanel("plugin.late")
	if h.app.Sidebar.ActiveWidget() != replacement {
		t.Fatal("remove and re-add did not wire the reloaded plugin surface")
	}
}
