package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWordWrapOverridesRoundTrip(t *testing.T) {
	dir := withTempConfigDir(t)
	input := `{"editor":{"wordWrap":true,"wordWrapColumn":80,"languageOverrides":{"markdown":{"wordWrapColumn":0},"typescript":{"wordWrap":false},"go":{},"unknown":{"wordWrap":true,"wordWrapColumn":65}}}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(input), 0644); err != nil {
		t.Fatal(err)
	}
	s := LoadSettings()
	for _, tc := range []struct {
		language string
		enabled  bool
		column   int
	}{
		{"markdown", true, 0}, {"typescript", false, 80}, {"go", true, 80},
		{"unknown", true, 65}, {"python", true, 80}, {"", true, 80},
	} {
		if enabled, column := s.Editor.ResolveWordWrap(tc.language); enabled != tc.enabled || column != tc.column {
			t.Errorf("%q: got %v/%d, want %v/%d", tc.language, enabled, column, tc.enabled, tc.column)
		}
	}
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	loaded := LoadSettings()
	if !reflect.DeepEqual(loaded.Editor.LanguageOverrides, s.Editor.LanguageOverrides) {
		t.Fatal("round trip lost explicit values or omitted fields")
	}
	for _, input := range []string{`{"wordWrap":true}`, `{"wordWrap":true,"languageOverrides":{}}`} {
		var e EditorSettings
		if err := json.Unmarshal([]byte(input), &e); err != nil {
			t.Fatal(err)
		}
		if enabled, column := e.ResolveWordWrap("markdown"); !enabled || column != 0 {
			t.Fatalf("legacy config = %v/%d", enabled, column)
		}
	}
}

func TestDefaultMarkdownWordWrap(t *testing.T) {
	editor := DefaultEditorSettings()
	if enabled, column := editor.ResolveWordWrap("markdown"); !enabled || column != 0 {
		t.Fatalf("Markdown defaults = %v/%d, want true/0", enabled, column)
	}
	if enabled, _ := editor.ResolveWordWrap("go"); enabled {
		t.Fatal("Markdown default enabled wrapping for Go")
	}
	editor.WordWrapColumn = 90
	editor.WordWrapBreak = WordWrapBreakCharacter
	if enabled, column := editor.ResolveWordWrap("markdown"); !enabled || column != 90 {
		t.Fatalf("Markdown wrapping = %v/%d, want true/90", enabled, column)
	}
	if got := editor.ResolveWordWrapBreak("markdown"); got != WordWrapBreakCharacter {
		t.Fatalf("Markdown break mode = %q, want inherited character mode", got)
	}
}

func TestMarkdownWordWrapDefaultPreservesExistingOverrides(t *testing.T) {
	wrapColumn := 75
	for _, tc := range []struct {
		name    string
		input   string
		want    EditorLanguageOverride
		enabled bool
		column  int
	}{
		{name: "missing overrides", input: `{"editor":{"wordWrap":false}}`, want: EditorLanguageOverride{WordWrap: boolPtr(true)}, enabled: true},
		{name: "other language only", input: `{"editor":{"languageOverrides":{"go":{"wordWrap":true}}}}`, want: EditorLanguageOverride{WordWrap: boolPtr(true)}, enabled: true},
		{name: "explicit off", input: `{"editor":{"languageOverrides":{"markdown":{"wordWrap":false}}}}`, want: EditorLanguageOverride{WordWrap: boolPtr(false)}},
		{name: "explicit inheritance", input: `{"editor":{"languageOverrides":{"markdown":{}}}}`},
		{name: "width only", input: `{"editor":{"languageOverrides":{"markdown":{"wordWrapColumn":75}}}}`, want: EditorLanguageOverride{WordWrapColumn: &wrapColumn}, column: 75},
		{name: "existing enabled and width", input: `{"editor":{"languageOverrides":{"markdown":{"wordWrap":true,"wordWrapColumn":75}}}}`, want: EditorLanguageOverride{WordWrap: boolPtr(true), WordWrapColumn: &wrapColumn}, enabled: true, column: 75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := withTempConfigDir(t)
			if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(tc.input), 0o644); err != nil {
				t.Fatal(err)
			}
			settings := LoadSettings()
			if got := settings.Editor.LanguageOverrides["markdown"]; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Markdown override = %+v, want %+v", got, tc.want)
			}
			if enabled, column := settings.Editor.ResolveWordWrap("markdown"); enabled != tc.enabled || column != tc.column {
				t.Fatalf("Markdown wrapping = %v/%d, want %v/%d", enabled, column, tc.enabled, tc.column)
			}
		})
	}
}

func TestWordWrapNegativeColumns(t *testing.T) {
	s := DefaultSettings()
	if err := json.Unmarshal([]byte(`{"editor":{"wordWrapColumn":-1,"languageOverrides":{"markdown":{"wordWrapColumn":-3}}}}`), &s); err != nil {
		t.Fatal(err)
	}
	normalizeSettings(&s)
	o := s.Editor.LanguageOverrides["markdown"]
	if s.Editor.WordWrapColumn != 0 || o.WordWrapColumn == nil || *o.WordWrapColumn != 0 {
		t.Fatalf("negative columns not normalized: %+v", s.Editor)
	}
}

func TestWordWrapBreakOverrides(t *testing.T) {
	s := DefaultSettings()
	if err := json.Unmarshal([]byte(`{"editor":{"languageOverrides":{"log":{"wordWrapBreak":"character"},"go":{},"bad":{"wordWrapBreak":"nope"}}}}`), &s); err != nil {
		t.Fatal(err)
	}
	normalizeSettings(&s)
	for _, tc := range []struct{ language, want string }{
		{"log", WordWrapBreakCharacter}, {"go", WordWrapBreakWord}, {"bad", WordWrapBreakWord}, {"", WordWrapBreakWord},
	} {
		if got := s.Editor.ResolveWordWrapBreak(tc.language); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.language, got, tc.want)
		}
	}
	if s.Editor.LanguageOverrides["bad"].WordWrapBreak != "" {
		t.Fatal("invalid override not cleared")
	}

	s = DefaultSettings()
	if err := json.Unmarshal([]byte(`{"editor":{"wordWrapBreak":"character","languageOverrides":{"markdown":{"wordWrapBreak":"word"}}}}`), &s); err != nil {
		t.Fatal(err)
	}
	normalizeSettings(&s)
	if got := s.Editor.ResolveWordWrapBreak("go"); got != WordWrapBreakCharacter {
		t.Fatalf("global character: got %q", got)
	}
	if got := s.Editor.ResolveWordWrapBreak("markdown"); got != WordWrapBreakWord {
		t.Fatalf("markdown override: got %q", got)
	}

	s = DefaultSettings()
	if err := json.Unmarshal([]byte(`{"editor":{"wordWrapBreak":"bogus"}}`), &s); err != nil {
		t.Fatal(err)
	}
	normalizeSettings(&s)
	if s.Editor.WordWrapBreak != WordWrapBreakWord {
		t.Fatalf("invalid global not normalized: %q", s.Editor.WordWrapBreak)
	}
}
