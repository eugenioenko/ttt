package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestSidebarWidthRestoresAndPersists(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	settings := config.DefaultSettings()
	settings.Sidebar.Width = 22
	a := buildTestApp(t, settings)
	if a.SplitPanel.DividerPos != 22 {
		t.Fatalf("restored sidebar width = %d, want 22", a.SplitPanel.DividerPos)
	}

	a.SetSidebarWidth(18)
	if got := a.State.SidebarWidth; got != 18 {
		t.Fatalf("sidebar width = %d, want 18", got)
	}
	if got := config.LoadState().SidebarWidth; got != 18 {
		t.Fatalf("persisted sidebar width = %d, want 18", got)
	}
}

func TestSidebarDraggedClosedRestoresUsableWidth(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	a := buildTestApp(t, config.DefaultSettings())
	a.SetSidebarWidth(18)
	for w := 5; w >= 0; w-- {
		a.resizeSidebar(w)
	}
	a.persistSidebarLayout()
	if got := config.LoadState(); got.SidebarWidth != 18 || !got.SidebarHidden {
		t.Fatalf("persisted state = %+v, want width 18 and hidden", got)
	}

	if err := config.SaveState(config.State{SidebarWidth: 2}); err != nil {
		t.Fatal(err)
	}
	a = buildTestApp(t, config.DefaultSettings())
	if a.SplitPanel.DividerPos != ui.DefaultSidebarWidth {
		t.Fatalf("restored sidebar width = %d, want %d", a.SplitPanel.DividerPos, ui.DefaultSidebarWidth)
	}
}

func TestSidebarClosedStateRestores(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	folder := t.TempDir()
	build := func() *App {
		cfg := config.AppConfig{
			Keybindings: config.DefaultKeybindings(),
			Settings:    config.DefaultSettings(),
			Theme:       config.DefaultTheme(),
		}
		borders := BuildBorderSet(cfg.Theme.Borders)
		return BuildAppFromConfig(&cfg, &borders, workspace.New([]string{folder}), nil)
	}

	a := build()
	if !a.Sidebar.Visible {
		t.Fatal("sidebar should start visible with a folder open")
	}
	a.ToggleSidebar()
	if a = build(); a.Sidebar.Visible || a.SplitPanel.ShowLeft {
		t.Fatal("sidebar closed in the previous session should stay closed")
	}

	a.ToggleSidebar()
	if a = build(); !a.Sidebar.Visible {
		t.Fatal("sidebar reopened in the previous session should start visible")
	}

	a.Reg = command.NewRegistry()
	RegisterCommands(a)
	a.ShowEmptyState()
	if a = build(); !a.Sidebar.Visible {
		t.Fatal("hiding the sidebar for the empty state must not persist")
	}
}

func TestPanelSizeRestoresAndPersists(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	a := buildTestApp(t, config.DefaultSettings())
	a.ContentSplit.BottomH = 12
	a.ContentSplit.RightW = 45
	a.persistPanelSize()

	a.ContentSplit.BottomH = 2
	a.ContentSplit.RightW = 10
	a.persistPanelSize()
	if got := config.LoadState(); got.PanelHeight != 12 || got.PanelWidth != 45 {
		t.Fatalf("persisted panel size = %dx%d, want 12x45", got.PanelHeight, got.PanelWidth)
	}

	a = buildTestApp(t, config.DefaultSettings())
	if a.ContentSplit.BottomH != 12 || a.ContentSplit.RightW != 45 {
		t.Fatalf("restored panel size = %dx%d, want 12x45", a.ContentSplit.BottomH, a.ContentSplit.RightW)
	}

	if err := config.SaveState(config.State{PanelHeight: 2, PanelWidth: 10}); err != nil {
		t.Fatal(err)
	}
	a = buildTestApp(t, config.DefaultSettings())
	if a.ContentSplit.BottomH < ui.MinPanelHeight || a.ContentSplit.RightW < ui.MinPanelWidth {
		t.Fatalf("restored tiny panel size = %dx%d", a.ContentSplit.BottomH, a.ContentSplit.RightW)
	}
}

func TestReopeningTinyPanelResetsSize(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.ContentSplit.SetRect(ui.Rect{W: 100, H: 40})
	a.ContentSplit.BottomH = 2
	a.ContentSplit.RightW = 3
	a.ensureUsablePanelSize()
	if a.ContentSplit.BottomH != 20 || a.ContentSplit.RightW != ui.DefaultPanelWidth {
		t.Fatalf("panel size = %dx%d, want 20x%d", a.ContentSplit.BottomH, a.ContentSplit.RightW, ui.DefaultPanelWidth)
	}
}
