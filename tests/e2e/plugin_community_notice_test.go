package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/plugin"
)

const (
	detailNotice   = "Community plugin. Review its permissions and source before installing."
	approvalNotice = "Community plugin. Review its permissions"
)

func TestPluginDetailCommunityNotice(t *testing.T) {
	notCommunity := false
	tests := []struct {
		name       string
		entry      plugin.RemoteRegistryEntry
		wantNotice bool
	}{
		{"third-party repo", plugin.RemoteRegistryEntry{Name: "auto-save", Repo: "https://github.com/someone/ttt-auto-save"}, true},
		{"maintained repo", plugin.RemoteRegistryEntry{Name: "bookmarks", Repo: "https://github.com/eugenioenko/ttt-plugins", Path: "bookmarks"}, false},
		{"flagged not community", plugin.RemoteRegistryEntry{Name: "db", Repo: "https://github.com/someone/ttt-db", Community: &notCommunity}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			configDir := filepath.Join(h.dir, "config")
			manager := plugin.NewManager(filepath.Join(configDir, "plugins"), filepath.Join(configDir, "plugins.ttt.json"))
			h.app.PluginManager = manager
			t.Cleanup(manager.Shutdown)
			manager.LoadAll()
			pp := app.NewPluginsPanel(manager)
			h.app.WirePluginsPanel(pp)
			pp.SetAvailable([]plugin.RemoteRegistryEntry{tt.entry})

			showPluginDetail(t, h, tt.entry)
			if tt.wantNotice {
				h.assertContains(detailNotice)
			} else {
				h.assertNotContains(detailNotice)
			}
		})
	}
}

func TestPluginApprovalCommunityNotice(t *testing.T) {
	notCommunity := false
	flagged := plugin.RemoteRegistryEntry{Name: "dbview", Repo: "https://github.com/someone/ttt-db", Community: &notCommunity}
	tests := []struct {
		name       string
		repo       string
		fromURL    bool
		wantNotice bool
	}{
		{"local plugin", "", false, true},
		{"maintained repo", "https://github.com/eugenioenko/ttt-plugins", false, false},
		{"maintained owner from URL", "https://github.com/eugenioenko/ttt-vim", true, false},
		{"flagged registry install", flagged.Repo, false, false},
		{"flagged repo from URL ignores the flag", flagged.Repo, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, p := pendingApprovalPlugin(t)
			defer h.stop()
			pp := app.NewPluginsPanel(h.app.PluginManager)
			h.app.WirePluginsPanel(pp)
			pp.SetAvailable([]plugin.RemoteRegistryEntry{flagged})

			p.Repo = tt.repo
			p.InstalledFromURL = tt.fromURL
			h.app.ShowPluginApprovalDialog(p)
			h.redraw()

			h.assertContains("dbview requests")
			h.assertContains("sqlite3")
			if tt.wantNotice {
				h.assertContains(approvalNotice)
			} else {
				h.assertNotContains(approvalNotice)
			}
		})
	}
}

func pendingApprovalPlugin(t *testing.T) (*testHarness, *plugin.Plugin) {
	t.Helper()
	h := newTestHarness(t, 120, 30)
	pluginsDir := filepath.Join(h.dir, "config", "plugins")
	dir := filepath.Join(pluginsDir, "dbview")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"plugin.ttt.json": `{"name": "dbview", "entry": "init.lua", "permissions": {"system.exec": ["sqlite3"]}}`,
		"init.lua":        `require("ttt")`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m := plugin.NewManager(pluginsDir, filepath.Join(h.dir, "config", "plugins.ttt.json"))
	h.app.PluginManager = m
	t.Cleanup(m.Shutdown)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(pending))
	}
	return h, pending[0]
}
