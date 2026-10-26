package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
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

func TestDebugPluginManifestUnreadableManifestIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read the directory regardless of its mode")
	}
	dir := filepath.Join(t.TempDir(), "plugin")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(`{"name": "scoped"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })
	if _, _, err := debugPluginManifest(filepath.Join(dir, "init.lua")); err == nil {
		t.Fatal("an unreadable manifest fell back to the default permissions")
	}
}

func TestLoadPluginFromFileReportsAnInvalidManifest(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	dir := t.TempDir()
	src := filepath.Join(dir, "init.lua")
	if err := os.WriteFile(src, []byte(`local ttt = require("ttt")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(`{"entry": "init.lua"}`), 0644); err != nil {
		t.Fatal(err)
	}
	before := len(a.Output.Lines)
	LoadPluginFromFile(a, src)
	reported := false
	for _, line := range a.Output.Lines[before:] {
		if strings.Contains(line.Message, "--plugin") {
			reported = true
		}
	}
	if !reported {
		t.Fatal("the invalid manifest was not reported in the Output panel")
	}
	if a.PluginManager != nil && a.PluginManager.FindPlugin("init") != nil {
		t.Fatal("a plugin was registered despite the invalid manifest")
	}
}
