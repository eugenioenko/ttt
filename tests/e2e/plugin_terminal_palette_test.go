package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
)

// This fixture intentionally lives in the test: it exercises the manifest,
// Lua module and host wiring without depending on a separately checked-out plugin.
func TestPluginTerminalPaletteManifestLifecycleAndWire(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	root := t.TempDir()
	dir := filepath.Join(root, "fixture")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"palette-fixture","entry":"init.lua","permissions":{"terminal.palette":true}}`
	source := `local terminal = require("ttt.terminal")
function check_palette()
local palette = terminal.palette()
assert(palette.status == "partial")
assert(palette.colors["10"] == "fixture-color")
terminal.request_palette()
end`
	for name, contents := range map[string]string{"plugin.ttt.json": manifest, "init.lua": source} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	m := plugin.NewManager(filepath.Join(root, "plugins"), filepath.Join(root, "registry.json"), root)
	h.app.PluginManager = m
	t.Cleanup(m.Shutdown)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending plugins = %d, want 1", len(pending))
	}
	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatal(err)
	}
	p := pending[0]
	if err := p.Granted.Check("terminal.palette"); err != nil {
		t.Fatalf("fixture permission not granted: %v", err)
	}
	if p.Terminal != nil {
		t.Fatal("terminal API should not be wired before WirePlugin")
	}
	h.app.TerminalPalette = &fixtureTerminalPalette{}
	h.app.WirePlugin(p)
	if p.Terminal == nil {
		t.Fatal("WirePlugin did not install terminal API")
	}
	if err := p.State.DoString(`check_palette()`); err != nil {
		t.Fatalf("Lua palette lifecycle: %v", err)
	}
}

type fixtureTerminalPalette struct{}

func (*fixtureTerminalPalette) Snapshot() (string, map[string]string) {
	return "partial", map[string]string{"10": "fixture-color"}
}

func (*fixtureTerminalPalette) Request() {}
