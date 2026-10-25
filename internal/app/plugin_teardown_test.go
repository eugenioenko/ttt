package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
)

func setupTestPluginApp(t *testing.T, pluginName, initLua string, permissions string) (*App, *plugin.Manager) {
	t.Helper()
	a := prepareListenTestApp(t)

	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	pluginDir := filepath.Join(pluginsDir, pluginName)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `{"name": "` + pluginName + `", "version": "1.0.0", "entry": "init.lua"`
	if permissions != "" {
		manifest += `, "permissions": ` + permissions
	}
	manifest += `}`

	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte(initLua), 0644); err != nil {
		t.Fatal(err)
	}

	m := plugin.NewManager(pluginsDir, filepath.Join(dir, "plugins.ttt.json"))
	t.Cleanup(m.Shutdown)
	a.PluginManager = m

	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(pending))
	}
	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatal(err)
	}
	a.WirePlugin(pending[0])
	return a, m
}

func TestPluginUninstallRemovesStatusItems(t *testing.T) {
	const initLua = `
local ttt = require("ttt")
ttt.register({ commands = { { id = "probe.mark", title = "Probe: Mark", handler = function()
	ttt.set_status_item("left", "status_one", "STATUS_1")
	ttt.set_status_item("right", "status_two", "STATUS_2")
end } } })
`
	a, m := setupTestPluginApp(t, "probe", initLua, `{"commands": true}`)

	if !a.Reg.Execute("probe.mark") {
		t.Fatal("failed to execute probe.mark")
	}

	var foundLeft, foundRight bool
	for _, seg := range a.Status.LeftSegments() {
		if seg.Text == "STATUS_1" && seg.ID == "probe:status_one" {
			foundLeft = true
		}
	}
	for _, seg := range a.Status.RightSegments() {
		if seg.Text == "STATUS_2" && seg.ID == "probe:status_two" {
			foundRight = true
		}
	}
	if !foundLeft || !foundRight {
		t.Fatalf("status items not found before uninstall: left=%v, right=%v", foundLeft, foundRight)
	}

	a.doPluginUninstall("probe")

	if m.FindPlugin("probe") != nil {
		t.Fatal("plugin still installed after uninstall")
	}

	for _, seg := range a.Status.LeftSegments() {
		if seg.Text == "STATUS_1" || seg.Owner == "plugin:probe" || seg.ID == "probe:status_one" {
			t.Errorf("left status item was not removed: %+v", seg)
		}
	}
	for _, seg := range a.Status.RightSegments() {
		if seg.Text == "STATUS_2" || seg.Owner == "plugin:probe" || seg.ID == "probe:status_two" {
			t.Errorf("right status item was not removed: %+v", seg)
		}
	}
}

func TestPluginUninstallClosesOnUninstallTabs(t *testing.T) {
	const initLua = `
local ttt = require("ttt")
ttt.on_uninstall(function()
	ttt.open_tab({ title = "PROBETAB", render = function(panel) panel:label("left behind") end })
end)
`
	a, m := setupTestPluginApp(t, "probe", initLua, `{"panel.editor": true}`)
	initialTabs := a.EditorGroup.TabCount()

	a.doPluginUninstall("probe")

	if m.FindPlugin("probe") != nil {
		t.Fatal("plugin still installed after uninstall")
	}

	if got := a.EditorGroup.TabCount(); got != initialTabs {
		t.Fatalf("tab count after uninstall = %d, want %d", got, initialTabs)
	}

	for i := range a.EditorGroup.TabCount() {
		title, _ := a.EditorGroup.TabInfo(i)
		if title == "PROBETAB" {
			t.Errorf("tab opened in on_uninstall was left behind: %q", title)
		}
	}
}

func TestPluginUninstallCleansUpBothActiveAndOnUninstallResources(t *testing.T) {
	const initLua = `
local ttt = require("ttt")
ttt.register({ commands = { { id = "probe.setup", title = "Probe: Setup", handler = function()
	ttt.set_status_item("left", "active_item", "ACTIVE_STATUS")
	ttt.open_tab({ title = "ACTIVETAB", render = function(panel) panel:label("active") end })
end } } })
ttt.on_uninstall(function()
	ttt.set_status_item("left", "teardown_item", "TEARDOWN_STATUS")
	ttt.open_tab({ title = "TEARDOWNTAB", render = function(panel) panel:label("teardown") end })
end)
`
	a, m := setupTestPluginApp(t, "probe", initLua, `{"commands": true, "panel.editor": true}`)
	initialTabs := a.EditorGroup.TabCount()

	if !a.Reg.Execute("probe.setup") {
		t.Fatal("failed to execute probe.setup")
	}

	if a.EditorGroup.TabCount() != initialTabs+1 {
		t.Fatalf("expected active tab to open, got tab count %d", a.EditorGroup.TabCount())
	}

	a.doPluginUninstall("probe")

	if m.FindPlugin("probe") != nil {
		t.Fatal("plugin still installed after uninstall")
	}

	if got := a.EditorGroup.TabCount(); got != initialTabs {
		t.Fatalf("tab count after uninstall = %d, want %d", got, initialTabs)
	}

	for _, seg := range a.Status.LeftSegments() {
		if seg.Owner == "plugin:probe" || seg.Text == "ACTIVE_STATUS" || seg.Text == "TEARDOWN_STATUS" {
			t.Errorf("lingering status item found after uninstall: %+v", seg)
		}
	}
}
