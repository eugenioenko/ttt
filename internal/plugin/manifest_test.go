package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	data := `{
		"name": "test-plugin",
		"description": "A test plugin",
		"version": "1.0.0",
		"author": "tester",
		"entry": "main.ttt.lua",
		"permissions": {
			"panel.sidebar": true,
			"commands": true,
			"system.exec": ["docker"]
		}
	}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "test-plugin" {
		t.Errorf("expected name test-plugin, got %s", m.Name)
	}
	if m.Entry != "main.ttt.lua" {
		t.Errorf("expected entry main.ttt.lua, got %s", m.Entry)
	}
	if !m.Permissions.PanelSidebar {
		t.Error("expected panel.sidebar to be true")
	}
	if len(m.Permissions.SystemExec) != 1 || m.Permissions.SystemExec[0] != "docker" {
		t.Errorf("expected system.exec [docker], got %v", m.Permissions.SystemExec)
	}
}

func TestLoadManifestDisplayName(t *testing.T) {
	dir := t.TempDir()
	data := `{"name":"my-plugin","displayName":"My Plugin","entry":"init.lua"}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.DisplayName != "My Plugin" {
		t.Errorf("expected displayName 'My Plugin', got %q", m.DisplayName)
	}
	if m.Title() != "My Plugin" {
		t.Errorf("Title() should use displayName, got %q", m.Title())
	}
}

func TestManifestTitleFallsBackToName(t *testing.T) {
	m := Manifest{Name: "my-plugin"}
	if m.Title() != "my-plugin" {
		t.Errorf("Title() should fall back to name, got %q", m.Title())
	}
}

func TestLoadManifestMissingName(t *testing.T) {
	dir := t.TempDir()
	data := `{"entry": "main.ttt.lua"}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadManifestMissingEntry(t *testing.T) {
	dir := t.TempDir()
	data := `{"name": "test"}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for missing entry")
	}
}

func TestLoadManifestMissingFile(t *testing.T) {
	_, err := LoadManifest(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadManifestAPIVersionDefaultsToOne(t *testing.T) {
	dir := t.TempDir()
	data := `{"name": "test", "entry": "init.lua"}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.API != 1 {
		t.Errorf("expected missing api to default to 1, got %d", m.API)
	}
}

func TestLoadManifestAPIVersionSupported(t *testing.T) {
	dir := t.TempDir()
	data := `{"name": "test", "entry": "init.lua", "api": 1}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.API != 1 {
		t.Errorf("expected api 1, got %d", m.API)
	}
}

func TestLoadManifestAPIVersion2Supported(t *testing.T) {
	dir := t.TempDir()
	data := `{"name": "test", "entry": "init.lua", "api": 2}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("api 2 should be supported: %v", err)
	}
	if m.API != 2 {
		t.Errorf("expected api 2, got %d", m.API)
	}
}

func TestLoadManifestAPIVersionTooNew(t *testing.T) {
	dir := t.TempDir()
	// One past the supported version, whatever that currently is.
	data := fmt.Sprintf(`{"name": "test", "entry": "init.lua", "api": %d}`, SupportedAPIVersion+1)
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for unsupported api version")
	}
}

func TestLoadManifestAPIVersionInvalid(t *testing.T) {
	dir := t.TempDir()
	data := `{"name": "test", "entry": "init.lua", "api": -1}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for invalid api version")
	}
}

func TestLoadManifestNetworkHosts(t *testing.T) {
	dir := t.TempDir()
	data := `{"name": "t", "entry": "init.lua", "permissions": {"network.http": ["api.github.com"]}}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Permissions.NetworkHTTP.All {
		t.Error("array form must not be all-hosts")
	}
	if !m.Permissions.NetworkHTTP.AllowsHost("api.github.com") {
		t.Error("declared host should be allowed")
	}
}

func TestLoadManifestNetworkInvalid(t *testing.T) {
	dir := t.TempDir()
	data := `{"name": "t", "entry": "init.lua", "permissions": {"network.http": 42}}`
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for non-bool/non-array network.http")
	}
}

func TestLoadManifestNetworkEntries(t *testing.T) {
	for _, tt := range []struct {
		hosts string
		ok    bool
	}{
		{`["api.github.com", "localhost", "127.0.0.1:4250", "[::1]:4250", "[::1]", "192.168.1.50:8080", "my_host.lan", "bücher.example", "127.0.0.1:080"]`, true},
		{`["127.0.0.1:0"]`, false},
		{`["127.0.0.1:65536"]`, false},
		{`["example.com:+443"]`, false},
		{`["example.com:"]`, false},
		{`["::1"]`, false},
		{`["::1:4250"]`, false},
		{`["[::1"]`, false},
		{`["[::1]:"]`, false},
		{`["[::1]x"]`, false},
		{`["[127.0.0.1]:80"]`, false},
		{`["[api.github.com]"]`, false},
		{`["[fe80::1%en0]:80"]`, false},
		{`["user@example.com"]`, false},
		{`["*.github.com"]`, false},
		{`["host with space"]`, false},
		{`["example.com."]`, false},
		{`["example.com/path:443"]`, false},
		{`["api.github.com:http"]`, false},
		{`["https://api.github.com"]`, false},
		{`[""]`, false},
		{`[":4250"]`, false},
		{`["api.github.com/v3"]`, false},
	} {
		dir := t.TempDir()
		data := `{"name": "t", "entry": "init.lua", "permissions": {"network.http": ` + tt.hosts + `}}`
		os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(data), 0644)
		if _, err := LoadManifest(dir); (err == nil) != tt.ok {
			t.Errorf("%s: err = %v, want ok %v", tt.hosts, err, tt.ok)
		}
	}
}

func TestLoadManifestGrammarOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(`{
		"name": "lang-zig",
		"grammars": [{ "path": "syntaxes/zig.json", "language": "Zig", "fileTypes": ["zig"] }]
	}`), 0644)

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Grammars) != 1 || m.Grammars[0].Path != "syntaxes/zig.json" || m.Grammars[0].Language != "Zig" {
		t.Errorf("grammars = %+v", m.Grammars)
	}

	p := &Plugin{Name: m.Name, Dir: dir, Manifest: m}
	if err := p.Init(); err != nil {
		t.Fatalf("Init grammar-only plugin: %v", err)
	}
	defer p.Destroy()
	if !p.Enabled {
		t.Error("grammar-only plugin not enabled after Init")
	}
}

func TestLoadManifestRejectsBadGrammarPath(t *testing.T) {
	for _, path := range []string{"", "../outside.json", "syntaxes/../../outside.json"} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(fmt.Sprintf(`{
			"name": "bad", "grammars": [{ "path": %q }]
		}`, path)), 0644)
		if _, err := LoadManifest(dir); err == nil {
			t.Errorf("path %q: expected error", path)
		}
	}
}

func TestLoadManifestThemes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(`{
		"name": "theme-pack", "themes": [
			{"path": "themes/dark.json", "name": "midnight"},
			{"path": "themes/light.json"}
		]
	}`), 0644)
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Themes) != 2 || m.Themes[0].ID() != "midnight" || m.Themes[1].ID() != "light" {
		t.Fatalf("themes = %+v", m.Themes)
	}
	p := &Plugin{Name: m.Name, Dir: dir, Manifest: m}
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	if !p.Enabled {
		t.Fatal("theme-only plugin not enabled")
	}
}

func TestLoadManifestRejectsInvalidThemes(t *testing.T) {
	for _, themes := range []string{
		`[{"path":""}]`, `[{"path":"../outside.json"}]`, `[{"path":"/outside.json"}]`,
		`[{"path":"theme.json","name":"../outside"}]`, `[{"path":"theme.json","name":"."}]`,
		`[{"path":"a/dark.json"},{"path":"b/dark.json"}]`,
		`[{"path":"a.json","name":"same"},{"path":"b.json","name":"same"}]`,
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(`{"name":"bad","themes":`+themes+`}`), 0644)
		if _, err := LoadManifest(dir); err == nil {
			t.Errorf("accepted invalid themes: %s", themes)
		}
	}
}

func TestThemeFileRejectsEscapingSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "theme.json")
	os.WriteFile(outside, []byte(`{}`), 0644)
	if err := os.Symlink(outside, filepath.Join(dir, "theme.json")); err != nil {
		t.Skip(err)
	}
	p := &Plugin{Dir: dir}
	if _, err := p.ThemeFile(Theme{Path: "theme.json"}); err == nil {
		t.Fatal("accepted a theme outside the plugin directory")
	}
}
