package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	s.Editor.DiffMode = DiffModeUnified
	s.Editor.DiffWordWrap = true
	s.Editor.DiffHighContrast = true
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
	if got.Editor.DiffMode != DiffModeUnified {
		t.Errorf("diffMode = %q, want %q", got.Editor.DiffMode, DiffModeUnified)
	}
	if !got.Editor.DiffWordWrap {
		t.Error("diffWordWrap did not round-trip")
	}
	if !got.Editor.DiffHighContrast {
		t.Error("diffHighContrast did not round-trip")
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
	normalizeSettings(&s)

	if s.Editor.GutterStyle != "compact" {
		t.Errorf("gutterStyle = %q, want compact", s.Editor.GutterStyle)
	}
	if s.Editor.BorderStyle != "default" {
		t.Errorf("borderStyle = %q, want default", s.Editor.BorderStyle)
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
}
