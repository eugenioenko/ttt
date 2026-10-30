package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
)

// The probe plugin from #972: a command sets a status item, and on_uninstall
// opens a tab.
const leftoverProbeInit = `
	local ttt = require("ttt")
	ttt.register({ commands = { { id = "probe.mark", title = "Probe: Mark", handler = function()
		ttt.set_status_item("left", "probe", "PROBESTATUS")
	end } } })
	ttt.on_uninstall(function()
		ttt.open_tab({ title = "PROBETAB", render = function(panel) panel:label("left behind") end })
	end)
`

func loadLeftoverProbe(h *testHarness) *plugin.Manager {
	h.t.Helper()
	return loadProbe(h, leftoverProbeInit)
}

func loadProbe(h *testHarness, init string) *plugin.Manager {
	h.t.Helper()
	cfg := filepath.Join(h.dir, "config")
	dir := filepath.Join(cfg, "plugins", "probe")
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.t.Fatal(err)
	}
	files := map[string]string{
		"plugin.ttt.json": `{"name": "probe", "version": "1.0.0", "entry": "init.lua", "permissions": {"commands": true, "panel.editor": true}}`,
		"init.lua":        init,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			h.t.Fatal(err)
		}
	}
	m := plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	h.app.PluginManager = m
	h.t.Cleanup(m.Shutdown)
	pending := m.LoadAll()
	if len(pending) != 1 {
		h.t.Fatalf("pending approvals = %d, want 1", len(pending))
	}
	if err := m.ApproveAndLoad(pending[0]); err != nil {
		h.t.Fatal(err)
	}
	h.app.WirePlugin(pending[0])
	h.redraw()
	return m
}

func (h *testHarness) confirmUninstall(name string) {
	h.t.Helper()
	h.app.PluginUninstallByName(name)
	h.redraw()
	h.clickDialogButton("Uninstall")
}

func (h *testHarness) clickDialogButton(label string) {
	h.t.Helper()
	for y := 0; y < 24; y++ {
		row := h.screenRow(y)
		if strings.Contains(row, "Cancel") {
			if x := displayColumnOf(row, label); x >= 0 {
				h.click(x+1, y)
				h.redraw()
				return
			}
		}
	}
	h.t.Fatalf("no %s button on screen", label)
}

func TestPluginUninstallRemovesItsStatusItems(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	m := loadLeftoverProbe(h)

	h.exec("probe.mark")
	h.assertContains("PROBESTATUS")

	h.confirmUninstall("probe")

	if m.FindPlugin("probe") != nil {
		t.Fatal("plugin still installed")
	}
	h.assertNotContains("PROBESTATUS")
}

func TestPluginUninstallClosesTabsItsUninstallCallbackOpens(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	m := loadLeftoverProbe(h)
	tabs := h.app.EditorGroup.TabCount()

	h.confirmUninstall("probe")

	if m.FindPlugin("probe") != nil {
		t.Fatal("plugin still installed")
	}
	if got := h.app.EditorGroup.TabCount(); got != tabs {
		t.Errorf("tab count = %d after uninstall, want %d", got, tabs)
	}
	h.assertNotContains("PROBETAB")
}

func TestPluginReloadDropsTheOldInstancesStatusItems(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	m := loadLeftoverProbe(h)
	m.FindPlugin("probe").SetBookmark(0, 'Q', term.Style{})

	h.exec("probe.mark")
	h.assertContains("PROBESTATUS")

	h.exec("plugin.reload")
	h.assertNotContains("PROBESTATUS")
	if bookmark, ok := h.app.EditorGroup.Editor.GetBookmark(0); !ok || bookmark.Icon != 'Q' {
		t.Fatal("shared bookmark did not survive plugin reload")
	}

	h.exec("probe.mark")
	h.assertContains("PROBESTATUS")
}

func TestPluginUninstallAsksBeforeRemovingAPluginWhoseCleanupFailed(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	m := loadProbe(h, `require("ttt").on_uninstall(function() error("launcher is read-only") end)`)

	h.confirmUninstall("probe")

	h.assertContains("Uninstall Failed")
	h.assertContains("launcher is read-only")
	if m.FindPlugin("probe") == nil {
		t.Fatal("plugin removed although its cleanup failed")
	}

	h.clickDialogButton("Remove Anyway")

	if m.FindPlugin("probe") != nil {
		t.Fatal("Remove Anyway left the plugin installed")
	}
}

func TestPluginBookmarksAreSharedAndSurviveUninstall(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	m := loadProbe(h, `local ttt = require("ttt"); ttt.register({})`)
	path := filepath.Join(h.dir, "bookmarks.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := h.app.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}

	first := m.FindPlugin("probe")
	second := &plugin.Plugin{Name: "other", Enabled: true}
	h.app.WirePlugin(second)
	first.SetBookmark(0, 'Q', term.Style{})
	second.SetBookmark(1, 'R', term.Style{})

	if got := len(first.GetAllBookmarks()); got != 2 {
		t.Fatalf("shared bookmark count = %d before uninstall, want 2", got)
	}

	h.confirmUninstall("probe")

	for line, icon := range map[int]rune{0: 'Q', 1: 'R'} {
		bookmark, ok := h.app.EditorGroup.Editor.GetBookmark(line)
		if !ok || bookmark.Icon != icon {
			t.Errorf("bookmark on line %d = (%q, %v), want %q to remain", line+1, bookmark.Icon, ok, icon)
		}
	}
}

func TestDebugPluginReplacementClearsOldPublishedSources(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	m := loadProbe(h, `local ttt = require("ttt"); ttt.register({})`)
	path := filepath.Join(h.dir, "probe.lua")
	source := "local ttt = require(\"ttt\")\nttt.register({})\n"
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if err := h.app.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}

	old := m.FindPlugin("probe")
	old.PublishDiagnostics(path, []plugin.DiagnosticItem{{
		StartLine: 0, EndLine: 0, Severity: 1, Message: "old diagnostic",
	}})
	old.SetDecorations(path, []plugin.DecorationItem{{
		StartLine: 0, EndLine: 0, Note: "old decoration",
	}})
	old.SetStatusItem("left", "old", "OLDSTATUS", 1000, nil)
	h.redraw()

	if got := len(h.app.EditorGroup.DiagnosticsByPath()[path]); got != 1 {
		t.Fatalf("old diagnostics = %d before replacement, want 1", got)
	}
	if h.app.EditorGroup.Editor.DiffOverlay == nil {
		t.Fatal("old decoration was not applied before replacement")
	}
	h.assertContains("OLDSTATUS")

	h.exec("debug.runPlugin")

	if got := m.FindPlugin("probe"); got == nil || got == old {
		t.Fatal("debug plugin did not replace the previous instance")
	}
	if got := len(h.app.EditorGroup.DiagnosticsByPath()[path]); got != 0 {
		t.Errorf("old diagnostics after replacement = %d, want 0", got)
	}
	if h.app.EditorGroup.Editor.DiffOverlay != nil {
		t.Error("old decoration remained after debug plugin replacement")
	}
	h.assertNotContains("OLDSTATUS")

	replacement := m.FindPlugin("probe")
	replacement.PublishDiagnostics(path, []plugin.DiagnosticItem{{
		StartLine: 0, EndLine: 0, Severity: 1, Message: "new diagnostic",
	}})
	replacement.SetDecorations(path, []plugin.DecorationItem{{
		StartLine: 0, EndLine: 0, Note: "new decoration",
	}})
	replacement.SetStatusItem("left", "new", "NEWSTATUS", 1000, nil)
	h.redraw()

	if got := len(h.app.EditorGroup.DiagnosticsByPath()[path]); got != 1 {
		t.Errorf("replacement diagnostics = %d, want 1", got)
	}
	if h.app.EditorGroup.Editor.DiffOverlay == nil {
		t.Error("replacement decoration was removed after publishing")
	}
	h.assertContains("NEWSTATUS")
}
