package config

import (
	"encoding/json"
	"maps"
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
	s.Appearance.Icons = IconsNone
	enabled := false
	s.Editor.SyntaxHighlight = &enabled
	s.Welcome.RecentFolders = &enabled
	s.Terminal.Shell = "/bin/zsh"
	s.Terminal.CtrlVPastes = false
	s.Editor.SelectionStyle = SelectionStyleInvert
	s.Terminal.SelectionStyle = SelectionStyleInvert

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
	if got.Appearance.Icons != IconsNone {
		t.Errorf("appearance.icons = %q; \"none\" did not round-trip", got.Appearance.Icons)
	}
	if got.Editor.IsSyntaxHighlightEnabled() {
		t.Error("syntaxHighlight=false did not round-trip; tri-state pointer lost")
	}
	if got.Welcome.IsRecentFoldersEnabled() {
		t.Error("welcome.recentFolders=false did not round-trip; tri-state pointer lost")
	}
	if got.Terminal.Shell != "/bin/zsh" {
		t.Errorf("shell = %q, want /bin/zsh", got.Terminal.Shell)
	}
	if got.Terminal.CtrlVPastes {
		t.Error("terminal.ctrlVPastes=false did not round-trip")
	}
	if got.Editor.SelectionStyle != SelectionStyleInvert {
		t.Errorf("editor.selectionStyle = %q, want %q", got.Editor.SelectionStyle, SelectionStyleInvert)
	}
	if got.Terminal.SelectionStyle != SelectionStyleInvert {
		t.Errorf("terminal.selectionStyle = %q, want %q", got.Terminal.SelectionStyle, SelectionStyleInvert)
	}
}

func TestLoadTerminalCtrlVPastes(t *testing.T) {
	dir := withTempConfigDir(t)
	for _, tt := range []struct {
		name string
		json string
		want bool
	}{
		{"no terminal settings", `{}`, true},
		{"existing terminal settings", `{"terminal":{"scrollback":2000}}`, true},
		{"enabled", `{"terminal":{"ctrlVPastes":true}}`, true},
		{"disabled", `{"terminal":{"ctrlVPastes":false}}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(tt.json), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := LoadSettings().Terminal.CtrlVPastes; got != tt.want {
				t.Errorf("CtrlVPastes = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuitSaveAllSettings(t *testing.T) {
	withTempConfigDir(t)
	for _, enabled := range []bool{false, true} {
		for _, exception := range []bool{false, true} {
			s := DefaultSettings()
			if s.Editor.SaveAllOnQuit || s.Editor.SaveAllOnQuitExceptUnnamed {
				t.Fatal("both quit settings must default to off")
			}
			s.Editor.SaveAllOnQuit = enabled
			s.Editor.SaveAllOnQuitExceptUnnamed = exception
			if err := SaveSettings(s); err != nil {
				t.Fatal(err)
			}
			got := LoadSettings().Editor
			if got.SaveAllOnQuit != enabled || got.SaveAllOnQuitExceptUnnamed != exception {
				t.Fatalf("quit settings did not round-trip: enabled=%v, exception=%v", enabled, exception)
			}
		}
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
	s.Appearance.Icons = "bogus"
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
	if s.Appearance.Icons != IconsNone {
		t.Errorf("appearance.icons = %q, want the none default", s.Appearance.Icons)
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
		s.Appearance.Icons = v
		normalizeSettings(&s)
		if s.Appearance.Icons != v {
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
	if got.Appearance.Icons != IconsNone {
		t.Fatalf("icons with no icons key = %q, want none", got.Appearance.Icons)
	}
}

func TestLegacyChevronsMigrateToOverrides(t *testing.T) {
	s := DefaultSettings()
	s.Appearance.Chevrons = &LegacyChevrons{Collapsed: "»", Expanded: "▼"}
	normalizeSettings(&s)
	if s.Appearance.Chevrons != nil {
		t.Errorf("chevrons = %+v, want removed", s.Appearance.Chevrons)
	}
	want := map[string]string{"chevron-right": "»"}
	if !maps.Equal(s.Appearance.Overrides, want) {
		t.Errorf("overrides = %v, want %v (old defaults dropped)", s.Appearance.Overrides, want)
	}
}

func TestDiffSettingsAreAlwaysWritten(t *testing.T) {
	s := DefaultSettings()
	s.Editor.DiffSigns = false
	s.Editor.DiffSignsColor = false
	s.Editor.DiffGutterHighlight = false
	s.Editor.DiffColorGutterNumbers = false
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Editor map[string]json.RawMessage `json:"editor"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"diffSigns", "diffSignsColor", "diffGutterHighlight", "diffColorGutterNumbers", "diffHighContrast", "diffEmphasizeCollapsedRows"} {
		if string(raw.Editor[key]) != "false" {
			t.Errorf("editor.%s = %s, want false written to settings.json", key, raw.Editor[key])
		}
	}
}

func TestDiffGutterSettingsDefaultWhenMissing(t *testing.T) {
	dir := withTempConfigDir(t)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"editor": {"tabSize": 2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings().Editor
	if got.DiffSigns || !got.DiffSignsColor || !got.DiffGutterHighlight || got.DiffColorGutterNumbers {
		t.Errorf("missing keys loaded as diffSigns=%v diffSignsColor=%v diffGutterHighlight=%v diffColorGutterNumbers=%v, want false, true, true, false", got.DiffSigns, got.DiffSignsColor, got.DiffGutterHighlight, got.DiffColorGutterNumbers)
	}
}

func TestHighlightTimeLimitIsHiddenUntilSet(t *testing.T) {
	editorKeys := func(s Settings) map[string]json.RawMessage {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var raw struct {
			Editor map[string]json.RawMessage `json:"editor"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		return raw.Editor
	}
	if v, ok := editorKeys(DefaultSettings())["highlightTimeLimit"]; ok {
		t.Fatalf("default settings wrote editor.highlightTimeLimit = %s; it should stay out of settings.json", v)
	}
	s := DefaultSettings()
	s.Editor.HighlightTimeLimit = 5000
	if v := string(editorKeys(s)["highlightTimeLimit"]); v != "5000" {
		t.Fatalf("editor.highlightTimeLimit = %q, want 5000 written once set", v)
	}
}
