package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
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

func TestDisabledPluginUpdateRefreshesActiveTheme(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	upstream := t.TempDir()
	gitIn(t, upstream, "init", "-q")
	writeVersion := func(version, background string) {
		t.Helper()
		files := map[string]string{
			"plugin.ttt.json": `{"name":"theme-pack","version":"` + version + `","themes":[{"path":"dark.json"}]}`,
			"dark.json":       `{"default":{"bg":"` + background + `"}}`,
		}
		for name, data := range files {
			if err := os.WriteFile(filepath.Join(upstream, name), []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
		}
		gitIn(t, upstream, "add", ".")
		gitIn(t, upstream, "commit", "-q", "-m", version)
	}
	writeVersion("1.0.0", "#123456")
	pluginsDir := filepath.Join(config.OverrideConfigDir, "plugins")
	os.MkdirAll(pluginsDir, 0755)
	gitIn(t, pluginsDir, "clone", "-q", upstream, "theme-pack")
	m := plugin.NewManager(pluginsDir, filepath.Join(config.OverrideConfigDir, "plugins.ttt.json"))
	h.app.PluginManager = m
	t.Cleanup(m.Shutdown)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(pending))
	}
	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatal(err)
	}
	h.app.WirePlugin(pending[0])
	h.app.Settings.Theme = "plugins/theme-pack/dark"
	h.app.ApplySettings(*h.app.Settings)
	h.app.PluginToggleByName("theme-pack", false)
	writeVersion("2.0.0", "#654321")
	h.awaitPluginUpdate("theme-pack", filepath.Join(h.dir, "unused.pdf"))
	loaded, err := config.LoadTheme(h.app.Settings.Theme)
	if err != nil || loaded.Default.Bg != "#654321" {
		t.Fatalf("installed theme background = %q, err %v, want updated background", loaded.Default.Bg, err)
	}
	if h.app.Screen.GetStyleMap()[term.StyleDefault].GetBackground() != tcell.GetColor("#654321") {
		t.Fatal("active theme colors were not refreshed")
	}
	if m.FindPlugin("theme-pack") != nil || m.Registry().Find("theme-pack").Enabled || h.app.Root.HasOverlay() {
		t.Fatal("updating themes loaded or requested approval for a disabled plugin")
	}
	if m.Registry().Find("theme-pack").Version != "2.0.0" {
		t.Fatal("disabled plugin version was not updated")
	}
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

func clickDialogButton(t *testing.T, h *testHarness, label string) {
	t.Helper()
	for y := 0; y < 30; y++ {
		row := h.screenRow(y)
		if x := displayColumnOf(row, label); x >= 0 {
			h.click(x+1, y)
			h.redraw()
			return
		}
	}
	t.Fatalf("button %q not found on screen:\n%s", label, h.screenText())
}

func TestPluginToggleRequestsApprovalForAdditionalPermissions(t *testing.T) {
	h, _, _ := newUpdatablePluginHarness(t, `, "fs.read": true`)
	defer h.stop()

	pp := app.NewPluginsPanel(h.app.PluginManager)
	h.app.WirePluginsPanel(pp)

	// Update adds fs.read and opens the approval dialog.
	h.awaitPluginUpdate("pdfview", filepath.Join(h.dir, "paper.pdf"))
	h.assertContains("pdfview requests")

	// Enabling while already awaiting approval from an update keeps prompting without duplicating the queue.
	pp.OnToggle("pdfview", true)
	h.redraw()
	if h.app.PluginManager.FindPlugin("pdfview") != nil {
		t.Fatal("plugin initialized before approval when toggled while awaiting update approval")
	}
	if len(h.app.PendingPluginApprovals) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(h.app.PendingPluginApprovals))
	}
	h.assertContains("pdfview requests")

	// Deny leaves it disabled and preserves only previously approved permissions.
	clickDialogButton(t, h, "Deny")
	if len(h.app.PendingPluginApprovals) != 0 {
		t.Fatalf("pending approvals after deny = %d, want 0", len(h.app.PendingPluginApprovals))
	}
	entry := h.app.PluginManager.Registry().Find("pdfview")
	if entry == nil || entry.Enabled || entry.Permissions.FsRead.Enabled() || !entry.Permissions.PanelSidebar {
		t.Fatalf("registry entry after deny = %+v, want disabled with old permissions only", entry)
	}

	// Enabling after denial prompts for approval instead of running with old permissions.
	pp.OnToggle("pdfview", true)
	h.redraw()
	if h.app.PluginManager.FindPlugin("pdfview") != nil {
		t.Fatal("plugin initialized before approval when enabled after denial")
	}
	h.assertContains("pdfview requests")

	// Restarting while pending approval keeps it discoverable at startup, and enabling the startup-pending entry prompts without duplicating the queue.
	h.app.DismissDialog()
	cfg := filepath.Join(h.dir, "config")
	restarted := plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	t.Cleanup(restarted.Shutdown)
	startupPending := restarted.LoadAll()
	if len(startupPending) != 1 || startupPending[0].Name != "pdfview" {
		t.Fatalf("startup pending = %v, want [pdfview]", startupPending)
	}
	h.app.PluginManager = restarted
	h.app.PendingPluginApprovals = startupPending
	pp = app.NewPluginsPanel(restarted)
	h.app.WirePluginsPanel(pp)
	pp.OnToggle("pdfview", true)
	h.redraw()
	if restarted.FindPlugin("pdfview") != nil {
		t.Fatal("plugin initialized before approval when enabled while awaiting startup approval")
	}
	if len(h.app.PendingPluginApprovals) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(h.app.PendingPluginApprovals))
	}
	h.assertContains("pdfview requests")

	// Allow initializes the plugin with the approved grant and wires its panel.
	clickDialogButton(t, h, "Allow")
	loaded := h.app.PluginManager.FindPlugin("pdfview")
	if loaded == nil || !loaded.Loaded() || !loaded.Granted.FsRead.Enabled() {
		t.Fatalf("plugin after Allow = %+v, want loaded with fs.read granted", loaded)
	}
	h.app.Sidebar.SetActivePanel("plugin.pdfview")
	h.redraw()
	h.assertContains("pdf sidebar 2.0.0")

	// Ordinary disable and re-enable with unchanged permissions loads without prompting.
	pp.OnToggle("pdfview", false)
	h.redraw()
	if h.app.PluginManager.FindPlugin("pdfview") != nil {
		t.Fatal("plugin still loaded after disable")
	}
	pp.OnToggle("pdfview", true)
	h.redraw()
	h.assertNotContains("pdfview requests")
	if reenabled := h.app.PluginManager.FindPlugin("pdfview"); reenabled == nil || !reenabled.Loaded() || !reenabled.Granted.FsRead.Enabled() {
		t.Fatalf("plugin after ordinary re-enable = %+v", reenabled)
	}
}
