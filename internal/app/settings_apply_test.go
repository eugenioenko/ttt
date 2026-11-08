package app

import (
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestCommitHistoryHeightRestoresAndPersists(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	settings := config.DefaultSettings()
	settings.Sidebar.CommitHistoryHeight = 17
	a := buildTestApp(t, settings)
	if a.Changes.Split.BottomH != 17 || a.Changes.Split.BottomRatio != 0 {
		t.Fatalf("restored split = height %d ratio %v, want height 17 ratio 0", a.Changes.Split.BottomH, a.Changes.Split.BottomRatio)
	}

	a.Changes.Split.BottomH = 12
	a.persistCommitHistoryHeight()
	if got := a.State.CommitHistoryHeight; got != 12 {
		t.Fatalf("commitHistoryHeight = %d, want 12", got)
	}
	if got := config.LoadState().CommitHistoryHeight; got != 12 {
		t.Fatalf("persisted commitHistoryHeight = %d, want 12", got)
	}
}

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

// commitTo writes only the fields the form owns. Settings changed elsewhere
// while the tab sat open — including ones the form never shows — must survive.
// Assigning the whole working struct instead would roll them back.
func TestCommitToLeavesUnownedSettingsAlone(t *testing.T) {
	opened := config.DefaultSettings()
	v := &settingsView{working: opened, categories: settingsCategories()}
	v.working.Editor.TabSize = 7

	live := opened
	live.LSP.HoverDelay = 1234
	live.Formatters = map[string]string{"go": "gofmt"}
	live.LSP.Servers = map[string]config.LSPServerConfig{"go": {Command: []string{"gopls"}}}

	v.commitTo(&live)

	if live.Editor.TabSize != 7 {
		t.Errorf("form edit not committed: TabSize = %d, want 7", live.Editor.TabSize)
	}
	if live.LSP.HoverDelay != 1234 {
		t.Errorf("clobbered an unowned setting: HoverDelay = %d, want 1234", live.LSP.HoverDelay)
	}
	if live.Formatters["go"] != "gofmt" {
		t.Error("clobbered the formatters map")
	}
	if _, ok := live.LSP.Servers["go"]; !ok {
		t.Error("clobbered the lsp servers map")
	}
}

// Every field in the table must be reachable through commitTo, or an edit made
// in the form would be silently dropped on Apply.
func TestCommitToCoversEveryField(t *testing.T) {
	for _, cat := range settingsCategories() {
		for _, f := range cat.Fields {
			switch f.Kind {
			case settingSection:
			case settingBool:
				if f.GetBool == nil || f.SetBool == nil {
					t.Errorf("%s → %s: bool field missing an accessor", cat.Title, f.Label)
				}
			case settingInt:
				if f.GetInt == nil || f.SetInt == nil {
					t.Errorf("%s → %s: int field missing an accessor", cat.Title, f.Label)
				}
			default:
				if f.GetString == nil || f.SetString == nil {
					t.Errorf("%s → %s: string field missing an accessor", cat.Title, f.Label)
				}
			}
		}
	}
}

func TestToggleIndentGuidesPersistsAndAppliesToEditorAndDiff(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	a := buildTestApp(t, config.DefaultSettings())
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-        old\n+        new\n")
	a.EditorGroup.OpenDiffTab("diff", "Diff", "test.go", fd, nil, nil, false)
	a.ToggleIndentGuides()

	if !a.Settings.Editor.IndentGuides || !a.EditorGroup.Editor.IndentGuides {
		t.Fatal("indent guides were not enabled on the editor")
	}
	d := a.EditorGroup.ActiveDiffWidget()
	if d == nil {
		t.Fatal("diff tab was not active")
	}
	const width, height = 60, 6
	grid := make([][]term.Cell, height)
	for y := range grid {
		grid[y] = make([]term.Cell, width)
	}
	d.SetRect(ui.Rect{W: width, H: height})
	d.Render(ui.NewRenderSurface(grid, ui.Rect{W: width, H: height}))
	guides := 0
	for _, row := range grid {
		for _, cell := range row {
			if cell.Ch == '│' && cell.Style == term.StyleMuted {
				guides++
			}
		}
	}
	if guides < 2 {
		t.Fatalf("diff pane rendered %d indent guides, want at least 2", guides)
	}
	if !config.LoadSettings().Editor.IndentGuides {
		t.Fatal("indent guides were not persisted")
	}
}

func TestDiffContextSettingLivesInDiff(t *testing.T) {
	found := ""
	count := 0
	for _, category := range settingsCategories() {
		for _, field := range category.Fields {
			if field.Label == "Diff context" {
				found = category.Title
				count++
			}
		}
	}
	if count != 1 || found != "Diff" {
		t.Fatalf("Diff context count = %d, category = %q; want one under Diff", count, found)
	}
}

func TestCollapsedDiffEmphasisSettingLivesInDiff(t *testing.T) {
	found := ""
	count := 0
	for _, category := range settingsCategories() {
		for _, field := range category.Fields {
			if field.Label == "Emphasize collapsed diff rows" {
				found = category.Title
				count++
			}
		}
	}
	if count != 1 || found != "Diff" {
		t.Fatalf("collapsed diff emphasis count = %d, category = %q; want one under Diff", count, found)
	}
}

func TestShowSettingsReopenPreservesPendingWorkingView(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.EditorGroup.OnContentTabClose = func(id string) {
		a.cleanupPluginDetailTab(id)
		a.cleanupSettingsTab(id)
	}
	a.ShowSettings()
	first := a.settingsView
	if first == nil {
		t.Fatal("opening settings did not create a working view")
	}
	first.working.Editor.TabSize = 7

	a.ShowSettings()

	if a.settingsView != first {
		t.Fatalf("reopening settings discarded working view: got %p, want %p", a.settingsView, first)
	}
	if got := a.settingsView.working.Editor.TabSize; got != 7 {
		t.Fatalf("reopening settings discarded pending tab size: got %d, want 7", got)
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

func TestSidebarTabStyleAppliesAtStartupAndLive(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	settings := config.DefaultSettings()
	settings.Sidebar.TabStyle = config.SidebarTabStyleIcon
	a := buildTestApp(t, settings)
	explorerLabel := func() string {
		for _, item := range a.Sidebar.Tabs.Config.Items {
			if item.ID == "explorer" {
				return item.Label
			}
		}
		return ""
	}
	icon := string(icons.Rune(icons.PanelExplorer))
	if got := explorerLabel(); got != icon {
		t.Fatalf("startup label = %q, want %q", got, icon)
	}

	next := *a.Settings
	next.Sidebar.TabStyle = config.SidebarTabStyleLabelIcon
	a.ApplySettings(next)
	if got, want := explorerLabel(), icon+" Explore"; got != want {
		t.Fatalf("label after apply = %q, want %q", got, want)
	}

	next.Sidebar.TabStyle = config.SidebarTabStyleIcon
	next.Appearance.Icons = config.IconsNerdFont
	a.ApplySettings(next)
	if got, want := explorerLabel(), icons.Get(config.IconsNerdFont, icons.PanelExplorer); got != want {
		t.Fatalf("label after icon mode change = %q, want %q", got, want)
	}
}

func TestPluginSidebarIconFollowsIconMode(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	settings := config.DefaultSettings()
	settings.Sidebar.TabStyle = config.SidebarTabStyleIcon
	a := buildTestApp(t, settings)
	a.PluginManager = plugin.NewManager(t.TempDir(), filepath.Join(t.TempDir(), "plugins.ttt.json"))
	a.PluginManager.SidebarPanels = []plugin.SidebarRegistration{
		{ID: "plugin.both", Title: "Both", Icon: plugin.PanelIcon{Plain: "T", Nerd: "N"}},
		{ID: "plugin.nerdonly", Title: "NerdOnly", Icon: plugin.PanelIcon{Nerd: "N"}},
	}
	a.Sidebar.AddPanel("plugin.both", "Both", a.Changes.Adapter)
	a.Sidebar.AddPanel("plugin.nerdonly", "NerdOnly", a.Changes.Adapter)
	labels := func() map[string]string {
		got := map[string]string{}
		for _, item := range a.Sidebar.Tabs.Config.Items {
			got[item.ID] = item.Label
		}
		return got
	}

	next := *a.Settings
	a.ApplySettings(next)
	if got := labels(); got["plugin.both"] != "T" || got["plugin.nerdonly"] != "NerdOnly" {
		t.Fatalf("plain mode labels = %v", got)
	}

	next.Appearance.Icons = config.IconsNerdFont
	a.ApplySettings(next)
	if got := labels(); got["plugin.both"] != "N" || got["plugin.nerdonly"] != "N" {
		t.Fatalf("nerd mode labels = %v", got)
	}
}

func TestApplySettingsPointerShapes(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	settings := config.DefaultSettings()
	a := buildTestApp(t, settings)
	sim := term.NewSimScreen()
	a.Screen = term.NewTcellScreenFrom(sim)

	if !a.Screen.PointerShapesEnabled() {
		t.Fatal("pointer shapes should default to enabled")
	}

	next := *a.Settings
	disabled := false
	next.Appearance.PointerShapes = &disabled
	a.ApplySettings(next)

	if a.Screen.PointerShapesEnabled() {
		t.Fatal("pointer shapes should be disabled after ApplySettings")
	}

	enabled := true
	next.Appearance.PointerShapes = &enabled
	a.ApplySettings(next)

	if !a.Screen.PointerShapesEnabled() {
		t.Fatal("pointer shapes should be enabled after ApplySettings")
	}
}
