//go:build windows

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSaveSettingsPreservesWindowsDACL(t *testing.T) {
	dir := withTempConfigDir(t)
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	before, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	after, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if before.String() != after.String() {
		t.Fatalf("settings DACL changed after save:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestWindowsSettingsReplacementConcurrentReaders(t *testing.T) {
	dir := withTempConfigDir(t)
	path := filepath.Join(dir, "settings.json")
	data, err := json.Marshal(DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	started := make(chan struct{})
	errors := make(chan error, 1)
	var saving atomic.Bool
	var readsDuringSaves atomic.Int64
	var readers sync.WaitGroup
	var startedOnce sync.Once
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				startedSaving := saving.Load()
				contents, err := readSettingsFileBytes(path)
				if err == nil {
					var settings Settings
					err = json.Unmarshal(contents, &settings)
				}
				if err != nil {
					select {
					case errors <- err:
					default:
					}
					return
				}
				if startedSaving {
					readsDuringSaves.Add(1)
				}
				startedOnce.Do(func() { close(started) })
			}
		}()
	}
	select {
	case <-started:
	case err := <-errors:
		close(stop)
		readers.Wait()
		t.Fatalf("reader failed before startup completed: %v", err)
	}
	saving.Store(true)
	for i := range 80 {
		settings := DefaultSettings()
		settings.Editor.TabSize = i + 1
		contents, err := json.Marshal(settings)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeFileAtomically(path, contents, 0o644); err != nil {
			saving.Store(false)
			close(stop)
			readers.Wait()
			t.Fatalf("replacement %d: %v", i, err)
		}
	}
	saving.Store(false)
	close(stop)
	readers.Wait()
	select {
	case err := <-errors:
		t.Fatalf("reader observed missing or invalid settings: %v", err)
	default:
	}
	if readsDuringSaves.Load() == 0 {
		t.Fatal("no reader read settings during the replacement loop")
	}
}
