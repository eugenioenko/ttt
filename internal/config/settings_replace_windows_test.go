//go:build windows

package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsSettingsLongPath(t *testing.T) {
	dir := t.TempDir()
	for len(dir) < 300 {
		dir = filepath.Join(dir, strings.Repeat("segment", 8))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "settings.json")
	for _, size := range []int{3, 9} {
		settings := DefaultSettings()
		settings.Editor.TabSize = size
		if err := SaveSettingsFile(path, settings); err != nil {
			t.Fatal(err)
		}
		got, err := ReadSettingsFile(path)
		if err != nil || got.Editor.TabSize != size {
			t.Fatalf("long-path round trip: tabSize=%d, err=%v", got.Editor.TabSize, err)
		}
	}
}

func TestWindowsSettingsReplacementRetries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		releaseAt int
		injected  error
		attempts  int
	}{
		{"sharing recovers", 3, nil, 3},
		{"sharing exhausted", 0, nil, 20},
		{"access denied", 0, windows.ERROR_ACCESS_DENIED, 20},
		{"invalid name", 0, windows.ERROR_INVALID_NAME, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "settings.json")
			if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
				t.Fatal(err)
			}
			name, err := settingsLongPathName(path)
			if err != nil {
				t.Fatal(err)
			}
			handle, err := windows.CreateFile(name, windows.GENERIC_READ,
				windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if handle != windows.InvalidHandle {
					_ = windows.CloseHandle(handle)
				}
			}()
			attempts := 0
			err = writeFileAtomicallyWithRename(path, []byte("new"), 0o644, func(from, to string) error {
				attempts++
				if attempts == tc.releaseAt {
					if err := windows.CloseHandle(handle); err != nil {
						t.Fatal(err)
					}
					handle = windows.InvalidHandle
				}
				if tc.injected != nil {
					return &os.LinkError{Op: "rename", Old: from, New: to, Err: tc.injected}
				}
				return os.Rename(from, to)
			})
			if attempts != tc.attempts {
				t.Fatalf("attempts = %d, want %d; err=%v", attempts, tc.attempts, err)
			}
			want := "old"
			if tc.releaseAt != 0 {
				want = "new"
				if err != nil {
					t.Fatal(err)
				}
			} else {
				cause := tc.injected
				if cause == nil {
					cause = windows.ERROR_SHARING_VIOLATION
				}
				if !errors.Is(err, cause) && !(tc.injected == nil && errors.Is(err, windows.ERROR_ACCESS_DENIED)) {
					t.Fatalf("error = %v, want %v", err, cause)
				}
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != want {
				t.Fatalf("destination = %q, %v; want %q", got, err, want)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary file leaked: %v, %v", entries, err)
			}
		})
	}
}
