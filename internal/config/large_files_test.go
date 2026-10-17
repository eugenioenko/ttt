package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLargeFileThresholds(t *testing.T) {
	for _, group := range []*LargeFileSettings{nil, {}} {
		e := EditorSettings{LargeFiles: group}
		size, on := e.LogModeMinBytes()
		if e.WordWrapMaxBytes() != 32<<20 || e.AutoReloadMaxBytes() != 64<<20 || size != 64<<20 || !on {
			t.Fatalf("defaults = %+v, %d, %v", e, size, on)
		}
	}
	for _, value := range []int{-1, 0, 100} {
		e := EditorSettings{LargeFiles: &LargeFileSettings{WordWrapMaxMB: &value, AutoReloadMaxMB: &value}}
		want := int64(max(value, 0)) << 20
		if e.WordWrapMaxBytes() != want || e.AutoReloadMaxBytes() != want {
			t.Fatalf("threshold %d resolved incorrectly", value)
		}
	}
}
func TestLogThresholdJSON(t *testing.T) {
	for _, test := range []struct {
		value     string
		bytes     int64
		on, unset bool
	}{
		{"false", 0, false, false}, {"0", 0, true, false}, {"256", 256 << 20, true, false},
		{"true", 64 << 20, true, true}, {`"x"`, 64 << 20, true, true}, {"-1", 64 << 20, true, true},
	} {
		t.Run(test.value, func(t *testing.T) {
			var e EditorSettings
			if err := json.Unmarshal([]byte(`{"largeFiles":{"logModeMinMB":`+test.value+`}}`), &e); err != nil {
				t.Fatal(err)
			}
			n, on := e.LogModeMinBytes()
			if n != test.bytes || on != test.on {
				t.Fatalf("got %d, %v", n, on)
			}
			if test.unset {
				if e.LargeFiles.LogModeMinMB != nil {
					t.Fatal("invalid threshold not discarded")
				}
				return
			}
			data, err := json.Marshal(e.LargeFiles.LogModeMinMB)
			if err != nil || string(data) != test.value {
				t.Fatalf("roundtrip = %s, %v", data, err)
			}
		})
	}
}
func TestLargeFileSettingsSave(t *testing.T) {
	dir := withTempConfigDir(t)
	s := DefaultSettings()
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "largeFiles") {
		t.Fatal("defaults pinned a largeFiles group")
	}
	zero := 0
	s.Editor.LargeFiles = &LargeFileSettings{WordWrapMaxMB: &zero, AutoReloadMaxMB: &zero, LogModeMinMB: &MBThreshold{Off: true}}
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	loaded := LoadSettings().Editor
	size, on := loaded.LogModeMinBytes()
	if loaded.WordWrapMaxBytes() != 0 || loaded.AutoReloadMaxBytes() != 0 || size != 0 || on {
		t.Fatalf("saved settings did not roundtrip: %+v", loaded.LargeFiles)
	}
}
