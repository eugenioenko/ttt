package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistryMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.ttt.json")
	r, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Entries) != 0 {
		t.Errorf("expected empty registry, got %d entries", len(r.Entries))
	}
}

func TestRegistryRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.ttt.json")
	r := &Registry{path: path}
	r.AddOrUpdate("test", "Test Plugin", "github.com/test/test", "", "1.0.0", PermissionSet{PanelSidebar: true})

	if err := r.Save(); err != nil {
		t.Fatalf("save error: %v", err)
	}

	r2, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(r2.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r2.Entries))
	}
	if r2.Entries[0].Name != "test" {
		t.Errorf("expected name test, got %s", r2.Entries[0].Name)
	}
	if r2.Entries[0].DisplayName != "Test Plugin" {
		t.Errorf("expected displayName to round-trip, got %q", r2.Entries[0].DisplayName)
	}
	if r2.Entries[0].Title() != "Test Plugin" {
		t.Errorf("expected Title() to use displayName, got %q", r2.Entries[0].Title())
	}
	if !r2.Entries[0].Permissions.PanelSidebar {
		t.Error("expected panel.sidebar to be true")
	}
}

func TestRegistryFind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.ttt.json")
	r := &Registry{path: path}
	r.AddOrUpdate("alpha", "", "", "", "1.0", PermissionSet{})
	r.AddOrUpdate("beta", "", "", "", "2.0", PermissionSet{})

	if r.Find("alpha") == nil {
		t.Error("expected to find alpha")
	}
	if r.Find("gamma") != nil {
		t.Error("expected nil for gamma")
	}
}

func TestRegistrySetEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.ttt.json")
	r := &Registry{path: path}
	r.AddOrUpdate("test", "", "", "", "1.0", PermissionSet{})

	r.SetEnabled("test", false)
	entry := r.Find("test")
	if entry.Enabled {
		t.Error("expected disabled")
	}
}

func TestRegistryUpdatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.ttt.json")
	r := &Registry{path: path}
	r.AddOrUpdate("test", "", "", "", "1.0", PermissionSet{})

	r.UpdatePermissions("test", PermissionSet{Commands: true})
	entry := r.Find("test")
	if !entry.Permissions.Commands {
		t.Error("expected commands to be true after update")
	}
}

func TestLoadRegistryInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.ttt.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := LoadRegistry(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestIsMaintained(t *testing.T) {
	no, yes := false, true
	entries := []RemoteRegistryEntry{
		{Name: "db", Repo: "https://github.com/someone/ttt-db", Community: &no},
		{Name: "pdf", Repo: "https://github.com/someone/ttt-pdf", Community: &yes},
		{Name: "save", Repo: "https://github.com/someone/ttt-save"},
	}
	tests := []struct {
		repo string
		want bool
	}{
		{"https://github.com/eugenioenko/ttt-plugins", true},
		{"https://github.com/eugenioenko/ttt-vim", true},
		{"https://github.com/EugenioEnko/ttt-plugins.git", true},
		{"https://www.github.com/eugenioenko/ttt-plugins/", true},
		{"https://github.com/eugenioenko-evil/ttt-plugins", false},
		{"https://github.com/eugenioenko/../evil/ttt", false},
		{"https://github.com/eugenioenko/ttt-x/../../evil/ttt", false},
		{"https://github.com/eugenioenko/ttt-", false},
		{"https://github.com/eugenioenko/forked-plugin", false},
		{"https://evil.com/github.com/eugenioenko/ttt-plugins", false},
		{"https://github.com/someone/ttt-db", true},
		{"https://github.com/someone/ttt-db.git", true},
		{"https://github.com/someone/ttt-pdf", false},
		{"https://github.com/someone/ttt-save", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsMaintained(tt.repo, entries); got != tt.want {
			t.Errorf("IsMaintained(%q) = %v, want %v", tt.repo, got, tt.want)
		}
	}
	if IsMaintained("https://github.com/someone/ttt-db", nil) {
		t.Error("registry flag must not apply without registry entries")
	}
}

func TestRemoteRegistryEntryCommunityDefaultsToUnset(t *testing.T) {
	var entries []RemoteRegistryEntry
	if err := json.Unmarshal([]byte(`[{"name":"a","repo":"r"},{"name":"b","repo":"r","community":false}]`), &entries); err != nil {
		t.Fatal(err)
	}
	if entries[0].Community != nil {
		t.Errorf("missing community = %v, want nil", *entries[0].Community)
	}
	if entries[1].Community == nil || *entries[1].Community {
		t.Errorf("community:false decoded as %v", entries[1].Community)
	}
}
