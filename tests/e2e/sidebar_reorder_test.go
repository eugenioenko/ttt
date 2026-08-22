package e2e

import (
	"slices"
	"strings"
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
	changesX := strings.Index(row, "Changes")
	explorerX := strings.Index(row, "Explore")
	if changesX < 0 || explorerX < 0 {
		t.Fatalf("tab labels not visible in %q", row)
	}

	if got := h.app.Root.HandleEvent(tcell.NewEventMouse(changesX+2, tabsY, tcell.Button1, 0)); got != ui.EventConsumed {
		t.Fatalf("mouse down result = %v, want EventConsumed", got)
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

func TestSidebarPendingTabDragOwnsCrossDividerRelease(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	h.exec("sidebar.changes")

	tabsY := h.app.Sidebar.Tabs.GetRect().Y
	row := h.screenRow(tabsY)
	changesX := strings.Index(row, "Changes")
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
	explorerX := strings.Index(row, "Explore")
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
