package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveKeybindingsReset(t *testing.T) {
	previous := OverrideConfigDir
	OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { OverrideConfigDir = previous })
	path := filepath.Join(OverrideConfigDir, "keybindings.json")
	if err := SaveKeybindings(DefaultKeybindings()); err != nil {
		t.Fatalf("reset with no file: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"file.save":"ctrl+alt+s"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveKeybindings(DefaultKeybindings()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("override file remains after reset: %v", err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(path, "keep")
	if err := os.WriteFile(blocker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveKeybindings(DefaultKeybindings()); err == nil {
		t.Fatal("reset reported success when removal failed")
	}
	if data, err := os.ReadFile(blocker); err != nil || string(data) != "keep" {
		t.Fatal("failed reset changed destination")
	}
}
