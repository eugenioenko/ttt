package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/gdamore/tcell/v3"
)

const updatablePluginManifest = `{
	"name": "pdfview",
	"version": "VERSION",
	"entry": "init.lua",
	"permissions": { "panel.sidebar": true, "editor.file_handlers": true EXTRA }
}`

const updatablePluginSource = `
local ttt = require("ttt")
ttt.register({
	sidebar = { title = "PDF", render = function(panel) panel:label("pdf sidebar VERSION") end },
	file_handlers = {
		{ id = "pdf", magic = "%PDF-", open = function(path, tab)
			tab.render = function(panel) panel:label("pdf page VERSION") end
		end },
	},
})
`

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func commitPluginVersion(t *testing.T, repo, version, extraPermissions string) {
	t.Helper()
	manifest := strings.ReplaceAll(updatablePluginManifest, "VERSION", version)
	files := map[string]string{
		"plugin.ttt.json": strings.ReplaceAll(manifest, "EXTRA", extraPermissions),
		"init.lua":        strings.ReplaceAll(updatablePluginSource, "VERSION", version),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-q", "-m", version)
}

// newUpdatablePluginHarness installs pdfview 1.0.0 from a local git remote that
// already holds 2.0.0, with its sidebar panel shown and paper.pdf open in it.
func newUpdatablePluginHarness(t *testing.T, extraPermissions string) (*testHarness, *plugin.Plugin, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	h := newTestHarness(t, 100, 30)

	upstream := t.TempDir()
	gitIn(t, upstream, "init", "-q")
	commitPluginVersion(t, upstream, "1.0.0", "")
	cfg := filepath.Join(h.dir, "config")
	pluginsDir := filepath.Join(cfg, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, pluginsDir, "clone", "-q", upstream, "pdfview")
	commitPluginVersion(t, upstream, "2.0.0", extraPermissions)

	m := plugin.NewManager(pluginsDir, filepath.Join(cfg, "plugins.ttt.json"))
	h.app.PluginManager = m
	t.Cleanup(m.Shutdown)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(pending))
	}
	p := pending[0]
	if err := m.ApproveAndLoad(p); err != nil {
		t.Fatal(err)
	}
	h.app.WirePlugin(p)
	h.app.ShowSidebar()
	h.app.Sidebar.SetActivePanel("plugin.pdfview")

	pdf := filepath.Join(h.dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01\x02\xff"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(pdf)
	h.redraw()
	h.assertContains("pdf sidebar 1.0.0")
	h.assertContains("pdf page 1.0.0")
	return h, p, pdf
}

// awaitPluginUpdate runs the UI goroutine's side of an update: until the
// fetch posts its result, it keeps rendering the plugin's sidebar panel and
// reopening files the plugin handles, then applies the result.
func (h *testHarness) awaitPluginUpdate(name, pdf string) {
	h.t.Helper()
	other := filepath.Join(h.dir, "other.pdf")
	if err := os.WriteFile(other, []byte("%PDF-1.7\n\x00\x01"), 0644); err != nil {
		h.t.Fatal(err)
	}
	h.app.PluginUpdateByName(name)
	deadline := time.After(20 * time.Second)
	for {
		select {
		case ev := <-h.screen.EventQ():
			iev, ok := ev.(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			if result, ok := iev.Data().(*app.PluginUpdateResult); ok {
				h.app.HandlePluginUpdateResult(result)
				h.redraw()
				return
			}
		case <-deadline:
			h.t.Fatal("timed out waiting for the update")
		default:
			h.app.EditorGroup.OpenFile(other)
			h.redraw()
			h.app.EditorGroup.OpenFile(pdf)
			h.redraw()
		}
	}
}

func TestPluginUpdateSwapsLuaStateOnUIGoroutine(t *testing.T) {
	h, old, pdf := newUpdatablePluginHarness(t, "")
	defer h.stop()

	h.awaitPluginUpdate("pdfview", pdf)

	updated := h.app.PluginManager.FindPlugin("pdfview")
	if updated == nil || updated == old || !updated.Loaded() || old.Loaded() {
		t.Fatalf("update did not replace the plugin: old loaded=%v, new=%v", old.Loaded(), updated)
	}
	h.assertContains("pdf sidebar 2.0.0")
	h.assertContains("pdf page 2.0.0")
	h.assertNotContains("Binary file")
}

func TestPluginUpdateNeedingApprovalReleasesPanels(t *testing.T) {
	h, old, pdf := newUpdatablePluginHarness(t, `, "fs.read": true`)
	defer h.stop()

	h.awaitPluginUpdate("pdfview", pdf)

	if old.Loaded() || h.app.PluginManager.FindPlugin("pdfview") != nil {
		t.Fatal("plugin is still loaded while its new permissions await approval")
	}
	if h.app.Sidebar.HasPanel("plugin.pdfview") {
		t.Error("sidebar kept the unloaded plugin's panel")
	}
	h.assertContains("pdfview requests")
	h.assertNotContains("pdf page")
}
