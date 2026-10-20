package config

import (
	"testing"
)

func TestAutoReloadMaxBytes(t *testing.T) {
	for _, group := range []*LargeFileSettings{nil, {}} {
		if got := (EditorSettings{LargeFiles: group}).AutoReloadMaxBytes(); got != 64<<20 {
			t.Fatalf("default limit = %d, want %d", got, 64<<20)
		}
	}
	for _, value := range []int{-1, 0, 100} {
		e := EditorSettings{LargeFiles: &LargeFileSettings{AutoReloadMaxMB: &value}}
		if got, want := e.AutoReloadMaxBytes(), int64(max(value, 0))<<20; got != want {
			t.Errorf("limit %d = %d bytes, want %d", value, got, want)
		}
	}
	maxInt := int(^uint(0) >> 1)
	if int64(maxInt) > ((1<<63)-1)>>20 {
		e := EditorSettings{LargeFiles: &LargeFileSettings{AutoReloadMaxMB: &maxInt}}
		if got := e.AutoReloadMaxBytes(); got != (1<<63)-1 {
			t.Fatalf("overflowing limit = %d, want MaxInt64", got)
		}
	}
}

func TestAutoReloadLimitPreservesExplicitZero(t *testing.T) {
	withTempConfigDir(t)
	s := DefaultSettings()
	zero := 0
	s.Editor.LargeFiles = &LargeFileSettings{AutoReloadMaxMB: &zero}
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	loaded := LoadSettings().Editor
	if loaded.LargeFiles == nil || loaded.LargeFiles.AutoReloadMaxMB == nil || loaded.AutoReloadMaxBytes() != 0 {
		t.Fatalf("explicit zero did not survive settings round trip: %+v", loaded.LargeFiles)
	}
	if loaded.WordWrapMaxBytes() != 32<<20 {
		t.Fatal("setting the reload limit changed the word-wrap default")
	}
}
