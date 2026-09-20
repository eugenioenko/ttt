package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func withTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	OverrideConfigDir = dir
	t.Cleanup(func() { OverrideConfigDir = "" })
	return dir
}

func TestSaveSettingsRoundTrips(t *testing.T) {
	withTempConfigDir(t)

	s := DefaultSettings()
	s.Editor.TabSize = 7
	s.Editor.WordWrap = true
	s.Sidebar.PanelOrder = []string{"changes", "plugin.todo", "explorer"}
	s.Sidebar.Width = 22
	s.Sidebar.CommitHistoryHeight = 17
	s.Git.FileView = GitFileViewTree
	s.Explorer.Icons = IconsNone
	s.Git.Icons = IconsNone
	enabled := false
	s.Editor.SyntaxHighlight = &enabled
	s.Terminal.Shell = "/bin/zsh"

	if err := SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	got := LoadSettings()
	if got.Editor.TabSize != 7 {
		t.Errorf("tabSize = %d, want 7", got.Editor.TabSize)
	}
	if !got.Editor.WordWrap {
		t.Error("wordWrap did not round-trip")
	}
	if !slices.Equal(got.Sidebar.PanelOrder, []string{"changes", "plugin.todo", "explorer"}) {
		t.Errorf("sidebar.panelOrder = %v", got.Sidebar.PanelOrder)
	}
	if got.Sidebar.Width != 22 {
		t.Errorf("sidebar.width = %d, want 22", got.Sidebar.Width)
	}
	if got.Sidebar.CommitHistoryHeight != 17 {
		t.Errorf("sidebar.commitHistoryHeight = %d, want 17", got.Sidebar.CommitHistoryHeight)
	}
	if got.Git.FileView != GitFileViewTree {
		t.Errorf("git.fileView = %q, want %q", got.Git.FileView, GitFileViewTree)
	}
	if got.Explorer.Icons != IconsNone || got.Git.Icons != IconsNone {
		t.Errorf("icons = explorer %q, git %q; \"none\" did not round-trip", got.Explorer.Icons, got.Git.Icons)
	}
	if got.Editor.IsSyntaxHighlightEnabled() {
		t.Error("syntaxHighlight=false did not round-trip; tri-state pointer lost")
	}
	if got.Terminal.Shell != "/bin/zsh" {
		t.Errorf("shell = %q, want /bin/zsh", got.Terminal.Shell)
	}
}

func TestSaveSettingsWritesValidJSON(t *testing.T) {
	dir := withTempConfigDir(t)

	if err := SaveSettings(DefaultSettings()); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json unreadable: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(data, &probe); err != nil {
		t.Errorf("settings.json is not valid JSON: %v", err)
	}
}

// A section whose fields are all false/zero must still be written. With
// `omitzero` on these sections the whole block was dropped, so turning both
// explorer toggles off silently reverted to the defaults on the next load.
func TestAllZeroSectionsSurviveRoundTrip(t *testing.T) {
	withTempConfigDir(t)

	s := DefaultSettings()
	s.Explorer = ExplorerSettings{ShowHidden: false, ShowGitIgnored: false}
	s.Autocomplete = AutocompleteSettings{}
	s.Search = SearchSettings{}

	if err := SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got := LoadSettings()

	if got.Explorer.ShowHidden || got.Explorer.ShowGitIgnored {
		t.Errorf("explorer flags reverted to defaults: %+v", got.Explorer)
	}
	if got.Autocomplete.Enabled || got.Autocomplete.AutoSuggest || got.Autocomplete.SignatureHelp {
		t.Errorf("autocomplete flags reverted to defaults: %+v", got.Autocomplete)
	}
	if got.Search.Debounce != 0 {
		t.Errorf("search.debounce = %d, want 0", got.Search.Debounce)
	}
}

func TestNormalizeRejectsUnknownEnumValues(t *testing.T) {
	s := DefaultSettings()
	s.Editor.GutterStyle = "bogus"
	s.Editor.BorderStyle = "bogus"
	s.Git.FileView = "bogus"
	s.Explorer.Icons = "bogus"
	s.Git.Icons = "bogus"
	normalizeSettings(&s)

	if s.Editor.GutterStyle != "compact" {
		t.Errorf("gutterStyle = %q, want compact", s.Editor.GutterStyle)
	}
	if s.Editor.BorderStyle != "default" {
		t.Errorf("borderStyle = %q, want default", s.Editor.BorderStyle)
	}
	if s.Git.FileView != GitFileViewList {
		t.Errorf("git.fileView = %q, want list", s.Git.FileView)
	}
	if s.Explorer.Icons != IconsNone || s.Git.Icons != IconsNone {
		t.Errorf("explorer.icons = %q, git.icons = %q, want the none default", s.Explorer.Icons, s.Git.Icons)
	}

	for _, v := range GutterStyles {
		s.Editor.GutterStyle = v
		normalizeSettings(&s)
		if s.Editor.GutterStyle != v {
			t.Errorf("normalize rejected valid gutter style %q", v)
		}
	}
	for _, v := range BorderStyles {
		s.Editor.BorderStyle = v
		normalizeSettings(&s)
		if s.Editor.BorderStyle != v {
			t.Errorf("normalize rejected valid border style %q", v)
		}
	}
	for _, v := range GitFileViews {
		s.Git.FileView = v
		normalizeSettings(&s)
		if s.Git.FileView != v {
			t.Errorf("normalize rejected valid Git file view %q", v)
		}
	}
	for _, v := range IconModes {
		s.Explorer.Icons = v
		s.Git.Icons = v
		normalizeSettings(&s)
		if s.Explorer.Icons != v || s.Git.Icons != v {
			t.Errorf("normalize rejected valid icon mode %q", v)
		}
	}
}

func TestIconsDefaultToNoneWhenUnset(t *testing.T) {
	dir := withTempConfigDir(t)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"explorer": {"showHidden": true}, "git": {"fileView": "tree"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings()
	if got.Explorer.Icons != IconsNone || got.Git.Icons != IconsNone {
		t.Fatalf("icons with no icons key = explorer %q, git %q, want none", got.Explorer.Icons, got.Git.Icons)
	}
}

func TestSidebarChevronsDefaultAndValidate(t *testing.T) {
	d := DefaultSettings()
	if d.Sidebar.TreeChevronCollapsed != "▶" || d.Sidebar.TreeChevronExpanded != "▼" {
		t.Fatalf("defaults = %q/%q", d.Sidebar.TreeChevronCollapsed, d.Sidebar.TreeChevronExpanded)
	}

	for name, value := range map[string]string{"empty": "", "two runes": "ab", "wide": "日", "grapheme": "é"} {
		s := DefaultSettings()
		s.Sidebar.TreeChevronCollapsed = value
		normalizeSettings(&s)
		if s.Sidebar.TreeChevronCollapsed != "▶" {
			t.Errorf("%s: collapsed = %q, want the default", name, s.Sidebar.TreeChevronCollapsed)
		}
	}

	s := DefaultSettings()
	s.Sidebar.TreeChevronCollapsed = ""
	s.Sidebar.TreeChevronExpanded = ">"
	normalizeSettings(&s)
	collapsed, expanded := s.Sidebar.Chevrons()
	if collapsed != '' || expanded != '>' {
		t.Errorf("chevrons = %q/%q, want the configured glyphs", collapsed, expanded)
	}
}

func TestFoldChevronsDefaultAndValidate(t *testing.T) {
	d := DefaultSettings()
	if d.Editor.FoldChevronCollapsed != "▶" || d.Editor.FoldChevronExpanded != "▼" {
		t.Fatalf("defaults = %q/%q", d.Editor.FoldChevronCollapsed, d.Editor.FoldChevronExpanded)
	}

	s := DefaultSettings()
	s.Editor.FoldChevronCollapsed = "ab"
	s.Editor.FoldChevronExpanded = "\ueab4"
	normalizeSettings(&s)
	collapsed, expanded := s.Editor.FoldChevrons()
	if collapsed != '▶' || expanded != '\ueab4' {
		t.Errorf("chevrons = %q/%q, want the default and the configured glyph", collapsed, expanded)
	}
}
