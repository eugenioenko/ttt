package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewTabsDefaultsToTrue(t *testing.T) {
	dir := withTempConfigDir(t)
	for _, tc := range []struct {
		name, raw string
		want      bool
	}{
		{"missing", `{}`, true},
		{"empty editor", `{"editor":{}}`, true},
		{"explicit true", `{"editor":{"previewTabs":true}}`, true},
		{"disabled", `{"editor":{"previewTabs":false}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(tc.raw), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := LoadSettings().Editor.PreviewTabsEnabled(); got != tc.want {
				t.Fatalf("PreviewTabsEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}
