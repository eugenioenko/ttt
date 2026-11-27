package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestSidebarTabClick(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.app.SplitPanel.DividerPos = 40
	h.app.Root.SetSize(80, 24)
	h.redraw()

	if h.app.Sidebar.ActivePanel != "explorer" {
		t.Fatalf("expected active panel 'explorer', got %q", h.app.Sidebar.ActivePanel)
	}

	sidebarY := h.app.Sidebar.GetRect().Y
	sidebarX := h.app.Sidebar.GetRect().X

	h.click(sidebarX+12, sidebarY)
	if h.app.Sidebar.ActivePanel != "search" {
		t.Errorf("expected active panel 'search' after click, got %q", h.app.Sidebar.ActivePanel)
	}

	h.click(sidebarX+3, sidebarY)
	if h.app.Sidebar.ActivePanel != "explorer" {
		t.Errorf("expected active panel 'explorer' after click, got %q", h.app.Sidebar.ActivePanel)
	}
}

func TestToggleSidebar(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.assertContains("Explore")

	h.exec("sidebar.toggle")
	h.assertNotContains("Explore")

	h.exec("sidebar.toggle")
	h.assertContains("Explore")
}

func TestSidebarPanelSwitching(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.explorer")
	if h.app.Sidebar.ActivePanel != "explorer" {
		t.Errorf("expected active panel 'explorer', got %q", h.app.Sidebar.ActivePanel)
	}

	h.exec("sidebar.search")
	if h.app.Sidebar.ActivePanel != "search" {
		t.Errorf("expected active panel 'search', got %q", h.app.Sidebar.ActivePanel)
	}

	h.exec("sidebar.changes")
	if h.app.Sidebar.ActivePanel != "changes" {
		t.Errorf("expected active panel 'changes', got %q", h.app.Sidebar.ActivePanel)
	}
}

func TestSidebarWidth(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	initial := h.app.SplitPanel.DividerPos

	h.exec("sidebar.wider")
	if h.app.SplitPanel.DividerPos != initial+1 {
		t.Errorf("expected width %d, got %d", initial+1, h.app.SplitPanel.DividerPos)
	}

	h.exec("sidebar.narrower")
	if h.app.SplitPanel.DividerPos != initial {
		t.Errorf("expected width %d, got %d", initial, h.app.SplitPanel.DividerPos)
	}
}

func TestFocusEditor(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.focus")
	if h.app.Root.Focused == h.app.EditorGroup {
		t.Error("focus should not be on editor after sidebar.focus")
	}

	h.exec("editor.focus")
	if h.app.Root.Focused != h.app.EditorGroup {
		t.Error("focus should be on editor after editor.focus")
	}
}

func TestSidebarTabOverflow(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.app.SplitPanel.DividerPos = 20
	h.app.Root.SetSize(80, 24)
	h.redraw()

	sidebarY := h.app.Sidebar.GetRect().Y
	row := h.screenRow(sidebarY)
	t.Logf("sidebar row (w=20): %q", row)

	if !strings.Contains(row, "»") {
		t.Errorf("expected overflow » indicator in narrow sidebar, got: %s", row)
	}

	if len(h.app.Sidebar.Tabs.HiddenTabs()) == 0 {
		t.Error("expected at least one hidden tab due to overflow")
	}

	h.app.SplitPanel.DividerPos = 50
	h.app.Root.SetSize(100, 24)
	h.redraw()

	row = h.screenRow(sidebarY)
	t.Logf("sidebar row (w=50): %q", row)

	if strings.Contains(row, "»") {
		t.Errorf("expected no overflow with wide sidebar, got: %s", row)
	}

	if len(h.app.Sidebar.Tabs.HiddenTabs()) != 0 {
		t.Errorf("expected 0 hidden tabs with wide sidebar, got %d", len(h.app.Sidebar.Tabs.HiddenTabs()))
	}
}

func TestSidebarOverflowLabelsFollowLiveIconSettings(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.app.PluginManager = plugin.NewManager(t.TempDir(), filepath.Join(t.TempDir(), "plugins.ttt.json"))
	h.app.PluginManager.SidebarPanels = []plugin.SidebarRegistration{
		{ID: "plugin.todo", Title: "Todo", Icon: plugin.PanelIcon{Plain: "T", Nerd: "N"}},
		{ID: "plugin.fallback", Title: "Fallback", Icon: plugin.PanelIcon{Nerd: "F"}},
	}
	h.app.Sidebar.AddPanel("plugin.todo", "Todo", h.app.Changes.Adapter)
	h.app.Sidebar.AddPanel("plugin.fallback", "Fallback", h.app.Changes.Adapter)
	h.app.SplitPanel.DividerPos = 18
	settings := *h.app.Settings
	settings.Sidebar.TabStyle = config.SidebarTabStyleLabelIcon
	settings.Appearance.Icons = config.IconsNone
	h.app.ApplySettings(settings)
	defer icons.Configure(config.IconsNone, nil)

	checkMenu := func(wantChanges, wantTodo, wantFallback string) {
		t.Helper()
		h.redraw()
		r := h.app.Sidebar.GetRect()
		x := displayColumnOf(h.screenRow(r.Y), "»")
		if x < 0 {
			t.Fatal("sidebar has no overflow button")
		}
		h.click(x, r.Y)
		menu, ok := h.app.Root.TopOverlayWidget().(*ui.ContextMenuWidget)
		if !ok {
			t.Fatalf("overflow opened %T, want context menu", h.app.Root.TopOverlayWidget())
		}
		labels := map[string]string{}
		for _, item := range menu.Items {
			labels[item.Command] = item.Label
		}
		for id, want := range map[string]string{"changes": wantChanges, "plugin.todo": wantTodo, "plugin.fallback": wantFallback} {
			if got := labels["sidebar.overflow."+id]; got != want {
				t.Errorf("overflow label for %s = %q, want %q", id, got, want)
			}
		}
		h.pressKey(tcell.KeyEscape, tcell.ModNone)
	}
	checkMenu(icons.Get(config.IconsNone, icons.PanelChanges)+" Changes", "T Todo", "Fallback")

	settings.Appearance.Icons = config.IconsNerdFont
	settings.Appearance.Overrides = map[string]string{string(icons.PanelChanges): "C"}
	h.app.ApplySettings(settings)
	checkMenu("C Changes", "N Todo", "F Fallback")

	settings.Sidebar.TabStyle = config.SidebarTabStyleLabel
	h.app.ApplySettings(settings)
	checkMenu("Changes", "Todo", "Fallback")

	settings.Sidebar.TabStyle = config.SidebarTabStyleIcon
	h.app.SplitPanel.DividerPos = 12
	h.app.ApplySettings(settings)
	checkMenu("Changes", "Todo", "Fallback")
}

func TestSidebarCollapseWidthReset(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	// Simulate dragging sidebar to a very small width (below MinSidebarWidth)
	h.app.SetSidebarWidth(2)
	if h.app.SplitPanel.DividerPos != 2 {
		t.Fatalf("expected divider at 2, got %d", h.app.SplitPanel.DividerPos)
	}

	// Toggle sidebar off
	h.exec("sidebar.toggle")
	if h.app.Sidebar.Visible {
		t.Fatal("sidebar should be hidden after toggle")
	}

	// Toggle sidebar back on — width should reset to default
	h.exec("sidebar.toggle")
	if !h.app.Sidebar.Visible {
		t.Fatal("sidebar should be visible after second toggle")
	}
	if h.app.SplitPanel.DividerPos != ui.DefaultSidebarWidth {
		t.Errorf("expected sidebar width to reset to %d after collapse, got %d",
			ui.DefaultSidebarWidth, h.app.SplitPanel.DividerPos)
	}

	// Verify a width at or above the minimum is preserved
	h.app.SetSidebarWidth(ui.MinSidebarWidth)
	h.exec("sidebar.toggle")
	h.exec("sidebar.toggle")
	if h.app.SplitPanel.DividerPos != ui.MinSidebarWidth {
		t.Errorf("expected sidebar width %d to be preserved, got %d",
			ui.MinSidebarWidth, h.app.SplitPanel.DividerPos)
	}
}
