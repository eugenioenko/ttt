package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
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

	a.persistCommitHistoryHeight(12)
	if got := a.Settings.Sidebar.CommitHistoryHeight; got != 12 {
		t.Fatalf("commitHistoryHeight = %d, want 12", got)
	}
	if got := config.LoadSettings().Sidebar.CommitHistoryHeight; got != 12 {
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

	a.persistSidebarWidth(18)
	if got := a.Settings.Sidebar.Width; got != 18 {
		t.Fatalf("sidebar width = %d, want 18", got)
	}
	if got := config.LoadSettings().Sidebar.Width; got != 18 {
		t.Fatalf("persisted sidebar width = %d, want 18", got)
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

func TestDiffContextSettingLivesInAppearance(t *testing.T) {
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
	if count != 1 || found != "Appearance" {
		t.Fatalf("Diff context count = %d, category = %q; want one under Appearance", count, found)
	}
}

func TestCollapsedDiffEmphasisSettingLivesInAppearance(t *testing.T) {
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
	if count != 1 || found != "Appearance" {
		t.Fatalf("collapsed diff emphasis count = %d, category = %q; want one under Appearance", count, found)
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
