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
	defaults := DefaultEditorSettings()
	if enabled, column := defaults.ResolveWordWrap("markdown"); enabled || column != 0 {
		t.Fatalf("defaults = %v/%d", enabled, column)
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
