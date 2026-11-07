package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/plugin"
)

func TestPluginDetailShowsFileHandlerTypes(t *testing.T) {
	tests := []struct {
		name     string
		handlers string
		want     string
	}{
		{
			name:     "extensions and title",
			handlers: `{{ id = "pdf", title = "PDF Viewer", extensions = { ".pdf", ".epub" }, open = function() end }, { id = "ps", title = "PostScript", magic = "%!PS", open = function() end }}`,
			want:     "Handles: .pdf, .epub (PDF Viewer), PostScript",
		},
		{
			name:     "magic-only handler shows title",
			handlers: `{{ id = "pdf", title = "PDF Viewer", magic = "%PDF-", open = function() end }}`,
			want:     "Handles: PDF Viewer",
		},
		{
			name:     "no handlers",
			handlers: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newPluginDetailHarness(t, tt.handlers)
			defer h.stop()

			if tt.want == "" {
				h.assertNotContains("Handles:")
				return
			}
			h.assertContains(tt.want)
		})
	}
}

func TestPluginDetailOmitsHandlersForUninstalledPlugin(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	configDir := filepath.Join(h.dir, "config")
	manager := plugin.NewManager(filepath.Join(configDir, "plugins"), filepath.Join(configDir, "plugins.ttt.json"))
	h.app.PluginManager = manager
	t.Cleanup(manager.Shutdown)
	manager.LoadAll()

	source := `local ttt = require("ttt")
ttt.register({ file_handlers = {{ id = "pdf", title = "PDF Viewer", extensions = { ".pdf" }, open = function() end }} })`
	pluginPath := filepath.Join(h.dir, "debug.lua")
	if err := os.WriteFile(pluginPath, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, pluginPath)
	p := manager.FindPlugin("debug")
	if p == nil || len(p.FileHandlers) != 1 {
		t.Fatalf("debug plugin file handlers = %v, want one", p)
	}

	showPluginDetail(t, h, plugin.RemoteRegistryEntry{Name: "debug", DisplayName: "Debug Plugin"})
	h.assertNotContains("Handles:")
}

func newPluginDetailHarness(t *testing.T, handlers string) *testHarness {
	t.Helper()
	h := newTestHarness(t, 100, 30)

	configDir := filepath.Join(h.dir, "config")
	pluginDir := filepath.Join(configDir, "plugins", "detail-test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `{"name":"detail-test","version":"1.0.0","entry":"init.lua","permissions":{"editor.file_handlers":true}}`
	source := `local ttt = require("ttt")
ttt.register({ file_handlers = ` + handlers + ` })`
	for name, content := range map[string]string{
		"plugin.ttt.json": manifest,
		"init.lua":        source,
	} {
		if err := os.WriteFile(filepath.Join(pluginDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	manager := plugin.NewManager(filepath.Join(configDir, "plugins"), filepath.Join(configDir, "plugins.ttt.json"))
	h.app.PluginManager = manager
	t.Cleanup(manager.Shutdown)
	pending := manager.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(pending))
	}
	p := pending[0]
	if err := manager.ApproveAndLoad(p); err != nil {
		t.Fatal(err)
	}
	h.app.WirePlugin(p)
	showPluginDetail(t, h, plugin.RemoteRegistryEntry{Name: "detail-test", DisplayName: "Test Plugin"})
	return h
}

func showPluginDetail(t *testing.T, h *testHarness, entry plugin.RemoteRegistryEntry) {
	t.Helper()
	h.app.OpenPluginDetail(entry)
	h.redraw()
}
