package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoClosingSettingsDefaultsAndPartialObjects(t *testing.T) {
	dir := withTempConfigDir(t)
	defaults := DefaultAutoClosingSettings()
	if defaults != (AutoClosingSettings{SkipClosing: true, DeletePairs: true}) {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}
	for _, tc := range []struct {
		name, raw string
		want      AutoClosingSettings
	}{
		{"missing", `{}`, defaults},
		{"empty editor", `{"editor":{}}`, defaults},
		{"empty object", `{"editor":{"autoClosing":{}}}`, defaults},
		{"null object", `{"editor":{"autoClosing":null}}`, defaults},
		{"opt in", `{"editor":{"autoClosing":{"brackets":true}}}`, AutoClosingSettings{Brackets: true, SkipClosing: true, DeletePairs: true}},
		{"quotes only", `{"editor":{"autoClosing":{"quotes":true}}}`, AutoClosingSettings{Quotes: true, SkipClosing: true, DeletePairs: true}},
		{"disable skipping", `{"editor":{"autoClosing":{"skipClosing":false}}}`, AutoClosingSettings{DeletePairs: true}},
		{"disable deletion", `{"editor":{"autoClosing":{"deletePairs":false}}}`, AutoClosingSettings{SkipClosing: true}},
		{"plain text", `{"editor":{"autoClosing":{"plainText":true}}}`, AutoClosingSettings{SkipClosing: true, DeletePairs: true, PlainText: true}},
		{"strings", `{"editor":{"autoClosing":{"inStrings":true}}}`, AutoClosingSettings{SkipClosing: true, DeletePairs: true, InStrings: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(tc.raw), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := LoadSettings().Editor.AutoClosing; got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestAutoClosingSettingsRoundTripEveryCombination(t *testing.T) {
	withTempConfigDir(t)
	for mask := 0; mask < 64; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			want := AutoClosingSettings{Brackets: mask&1 != 0, Quotes: mask&2 != 0, SkipClosing: mask&4 != 0, DeletePairs: mask&8 != 0, InStrings: mask&16 != 0, PlainText: mask&32 != 0}
			s := DefaultSettings()
			s.Editor.AutoClosing = want
			if err := SaveSettings(s); err != nil {
				t.Fatal(err)
			}
			if got := LoadSettings().Editor.AutoClosing; got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
			raw, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			decoded := DefaultSettings()
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Editor.AutoClosing != want {
				t.Fatal("JSON round trip changed settings")
			}
		})
	}
}
