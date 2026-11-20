package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAutoSurroundSettings(t *testing.T) {
	withTempConfigDir(t)
	s := DefaultSettings()
	if !s.Editor.AutoSurround.IsEnabled() {
		t.Fatal("must be enabled by default")
	}
	raw, _ := json.Marshal(s.Editor.AutoSurround)
	if string(raw) != `{"enabled":true}` {
		t.Fatalf("defaults = %s", raw)
	}
	err := json.Unmarshal([]byte(`{"editor":{"autoSurround":{"extraPairs":[["%","%"]]},"languageOverrides":{"markdown":{"autoSurround":{"extraPairs":[["*","*"],["%","!"]]}},"go":{"autoSurround":{"enabled":false}}}}}`), &s)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		lang       string
		open, want rune
	}{
		{"", '%', '%'}, {"", '*', 0}, {"markdown", '*', '*'}, {"markdown", '%', '!'},
		{"markdown", '(', ')'}, {"go", '(', 0}, {"go", '%', 0},
	} {
		if got := s.Editor.ResolveAutoSurround(tc.lang).Closer(tc.open); got != tc.want {
			t.Fatalf("%s %q: got %q, want %q", tc.lang, tc.open, got, tc.want)
		}
	}
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	loaded := LoadSettings()
	if !reflect.DeepEqual(s.Editor.AutoSurround, loaded.Editor.AutoSurround) || !reflect.DeepEqual(s.Editor.LanguageOverrides, loaded.Editor.LanguageOverrides) {
		t.Fatal("round trip changed settings")
	}
	s.Editor.AutoSurround.Enabled = boolPtr(false)
	if s.Editor.ResolveAutoSurround("markdown").IsEnabled() {
		t.Fatal("omitted enabled must inherit false")
	}
	s.Editor.LanguageOverrides["markdown"].AutoSurround.Enabled = boolPtr(true)
	if !s.Editor.ResolveAutoSurround("markdown").IsEnabled() {
		t.Fatal("explicit true must override false")
	}
	s.Editor.AutoSurround.ExtraPairs = []SurroundingPair{{"(", "!"}}
	s.Editor.AutoSurround.Enabled = boolPtr(true)
	if s.Editor.AutoSurround.Closer('(') != ')' {
		t.Fatal("extras must not replace built-in pairs")
	}
}

func TestAutoSurroundRejectsInvalidPairs(t *testing.T) {
	for _, raw := range []string{`["**","**"]`, `["",""]`, `["x"]`, `["x","y","z"]`, `["\n","x"]`, `[" "," "]`, `null`} {
		var s AutoSurroundSettings
		err := json.Unmarshal([]byte(`{"extraPairs":[`+raw+`]}`), &s)
		if err == nil || !strings.Contains(err.Error(), "autoSurround.extraPairs") {
			t.Errorf("%s: expected explanatory error, got %v", raw, err)
		}
	}
	var s AutoSurroundSettings
	if err := json.Unmarshal([]byte(`{"extraPairs":[["«","»"]]}`), &s); err != nil || s.Closer('«') != '»' {
		t.Fatalf("Unicode pair: %+v, %v", s, err)
	}
}
