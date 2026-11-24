package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAllReportsUserDirectoryCreationFailure(t *testing.T) {
	dir := t.TempDir()
	plugins := filepath.Join(dir, "plugins")
	if err := os.WriteFile(plugins, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(plugins, filepath.Join(dir, "plugins.ttt.json"))
	t.Cleanup(m.Shutdown)
	failures := 0
	m.OnDiscoveryError = func(path string, err error) {
		if path != plugins || err == nil {
			t.Fatalf("wrong discovery error: %s, %v", path, err)
		}
		failures++
	}
	m.LoadAll()
	if failures != 1 || m.registry == nil {
		t.Fatalf("discovery failed silently or skipped registry initialization: failures=%d", failures)
	}
	if err := os.Remove(plugins); err != nil {
		t.Fatal(err)
	}
	m.LoadAll()
	if failures != 1 {
		t.Fatal("healthy retry reported another directory failure")
	}
}
