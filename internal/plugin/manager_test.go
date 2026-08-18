package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePluginDir creates a minimal valid plugin (manifest + entry) in a new
// subdir of parent and returns its path.
func writePluginDir(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"` + name + `","version":"1.0.0","entry":"main.ttt.lua"}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ttt.lua"), []byte("-- noop"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	m := NewManager(pluginsDir, regPath)
	m.LoadAll() // initializes m.registry
	return m, pluginsDir
}

// TestDenyPluginDeletesFreshInstall: a plugin ttt cloned into its own
// pluginsDir with no registry entry must be removed on deny (#358).
func TestDenyPluginDeletesFreshInstall(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	dir := writePluginDir(t, pluginsDir, "fresh")

	p := &Plugin{Name: "fresh", Dir: dir, Manifest: Manifest{Name: "fresh", Version: "1.0.0"}}
	if err := m.DenyPlugin(p); err != nil {
		t.Fatalf("DenyPlugin: %v", err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("denied fresh install should be deleted from disk")
	}
	if m.registry.Find("fresh") != nil {
		t.Fatal("a deleted fresh install should leave no registry entry")
	}
}

// TestDenyPluginDisablesOutsideDir: a plugin living outside pluginsDir (e.g.
// workspace-local) must never be deleted; it's disabled in the registry instead.
func TestDenyPluginDisablesOutsideDir(t *testing.T) {
	m, _ := newTestManager(t)
	externalRoot := t.TempDir()
	dir := writePluginDir(t, externalRoot, "local")

	p := &Plugin{Name: "local", Dir: dir, Manifest: Manifest{Name: "local", Version: "1.0.0"}}
	if err := m.DenyPlugin(p); err != nil {
		t.Fatalf("DenyPlugin: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("workspace-local plugin must not be deleted: %v", err)
	}
	entry := m.registry.Find("local")
	if entry == nil {
		t.Fatal("denied on-disk plugin should get a registry entry")
	}
	if entry.Enabled {
		t.Fatal("denied plugin's registry entry must be disabled")
	}
}

// TestDenyStopsReprompt is the end-to-end regression for #358: after
// denying a rediscovered on-disk plugin, a fresh LoadAll must not surface it
// for approval again.
func TestDenyStopsReprompt(t *testing.T) {
	m, _ := newTestManager(t)
	externalRoot := t.TempDir()
	dir := writePluginDir(t, externalRoot, "local")
	m.extraDirs = []string{externalRoot}

	needs := m.LoadAll()
	if len(needs) != 1 || needs[0].Name != "local" {
		t.Fatalf("expected 'local' to need approval on first discovery, got %v", needs)
	}

	if err := m.DenyPlugin(needs[0]); err != nil {
		t.Fatalf("DenyPlugin: %v", err)
	}

	if needs := m.LoadAll(); len(needs) != 0 {
		t.Fatalf("denied plugin must not re-prompt, got %d pending", len(needs))
	}
	_ = dir
}

func TestWithinDir(t *testing.T) {
	base := filepath.Join("home", "user", "plugins")
	cases := []struct {
		child string
		want  bool
	}{
		{filepath.Join(base, "foo"), true},
		{filepath.Join(base, "foo", "bar"), true},
		{base, false}, // the dir itself
		{filepath.Join("home", "user", "other"), false}, // sibling
		{filepath.Join("home", "user"), false},          // parent
		{filepath.Join(base, "..", "escape"), false},    // traversal
	}
	for _, c := range cases {
		if got := withinDir(base, c.child); got != c.want {
			t.Errorf("withinDir(%q, %q) = %v, want %v", base, c.child, got, c.want)
		}
	}
}

// A plugin that fails to compile reports the failure from inside Init, and is
// never added to m.plugins for a later SetLogFactory backfill to reach. If the
// log sink is not wired before Init, the error is lost entirely (#395).
func TestLoadAllLogsInitFailure(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")

	dir := writePluginDir(t, pluginsDir, "broken")
	entry := filepath.Join(dir, "main.ttt.lua")
	if err := os.WriteFile(entry, []byte("this is not lua !!"), 0644); err != nil {
		t.Fatal(err)
	}
	reg := `[{"name":"broken","version":"1.0.0","enabled":true,"permissions":{}}]`
	if err := os.WriteFile(regPath, []byte(reg), 0644); err != nil {
		t.Fatal(err)
	}

	var logged []string
	m := NewManager(pluginsDir, regPath)
	m.SetLogFactory(func(name string) func(string, string) {
		return func(level, message string) {
			logged = append(logged, name+" "+level+" "+message)
		}
	})
	m.LoadAll()

	if len(logged) == 0 {
		t.Fatal("a plugin that fails to compile logged nothing")
	}
	if !strings.Contains(logged[0], "broken error init:") {
		t.Errorf("unexpected log line: %q", logged[0])
	}
	if len(m.Plugins()) != 0 {
		t.Errorf("a plugin that failed to init should not be loaded, got %d", len(m.Plugins()))
	}
}

// A plugin that loads cleanly must still get its log sink, so ttt.log works
// from the plugin's own init.
func TestLoadAllWiresLogForHealthyPlugin(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")

	dir := writePluginDir(t, pluginsDir, "healthy")
	entry := filepath.Join(dir, "main.ttt.lua")
	if err := os.WriteFile(entry, []byte(`local ttt = require("ttt")`+"\n"+`ttt.log("hello from init")`), 0644); err != nil {
		t.Fatal(err)
	}
	reg := `[{"name":"healthy","version":"1.0.0","enabled":true,"permissions":{}}]`
	if err := os.WriteFile(regPath, []byte(reg), 0644); err != nil {
		t.Fatal(err)
	}

	var logged []string
	m := NewManager(pluginsDir, regPath)
	m.SetLogFactory(func(name string) func(string, string) {
		return func(level, message string) { logged = append(logged, message) }
	})
	m.LoadAll()

	if len(logged) != 1 || logged[0] != "hello from init" {
		t.Errorf("logged = %v, want a single %q", logged, "hello from init")
	}
}
