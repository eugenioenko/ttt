package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDebugPluginManifestUsesManifestNextToFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "init.lua")
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(`{
		"name": "my-tool",
		"entry": "init.lua",
		"permissions": {"commands": true, "system.exec": ["git", "gh"]}
	}`), 0644); err != nil {
		t.Fatal(err)
	}

	m, granted, err := debugPluginManifest(src)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "my-tool" {
		t.Fatalf("name = %q, want my-tool", m.Name)
	}
	if !slices.Equal(granted.SystemExec, []string{"git", "gh"}) {
		t.Fatalf("system.exec = %v", granted.SystemExec)
	}
	if !granted.Commands || granted.EditorWrite || granted.Input {
		t.Fatalf("granted %+v, want exactly the manifest permissions", granted)
	}
}

func TestDebugPluginManifestWithoutManifestNamesAfterFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "scratch.lua")

	m, granted, err := debugPluginManifest(src)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "scratch" {
		t.Fatalf("name = %q, want scratch", m.Name)
	}
	if !granted.EditorWrite || !granted.Input || !granted.NetworkHTTP.All || len(granted.SystemExec) != 0 {
		t.Fatalf("granted %+v, want every permission but system.exec", granted)
	}
}

func TestDebugPluginManifestRejectsInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(`{"entry": "init.lua"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := debugPluginManifest(filepath.Join(dir, "init.lua")); err == nil {
		t.Fatal("manifest without a name loaded")
	}
}
