package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/gdamore/tcell/v3"
)

const requestExecPath = "/opt/vendor/postgresql-16.4/bin/psql"

func newRequestExecHarness(t *testing.T) (*testHarness, *plugin.Manager, string) {
	t.Helper()
	h := newTestHarness(t, 100, 30)
	cfg := filepath.Join(h.dir, "config")
	dir := filepath.Join(cfg, "plugins", "db")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"plugin.ttt.json": `{"name": "db", "displayName": "DB", "version": "1.0.0", "entry": "init.lua", "permissions": {"commands": true, "system.exec": ["psql"]}}`,
		"init.lua": `
			local ttt = require("ttt")
			local sys = require("ttt.system")
			local function ask()
				sys.request_exec("` + requestExecPath + `", function(granted)
					ttt.notify("granted=" .. tostring(granted))
				end)
			end
			ttt.register({ commands = { { id = "db.ask", title = "DB: Ask", handler = ask } } })
		`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	regPath := filepath.Join(cfg, "plugins.ttt.json")
	m := plugin.NewManager(filepath.Join(cfg, "plugins"), regPath)
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
	h.redraw()
	return h, m, regPath
}

func TestPluginRequestExecAllowPersistsThePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixtures use Unix absolute paths")
	}
	h, m, regPath := newRequestExecHarness(t)
	defer h.stop()

	h.exec("db.ask")
	h.redraw()
	h.assertContains("DB requests")
	h.assertContains("Allow it to run")
	h.assertContains(requestExecPath + "?")

	clicked := false
	for y := 0; y < 30 && !clicked; y++ {
		row := h.screenRow(y)
		if strings.Contains(row, "Deny") {
			if x := displayColumnOf(row, "Allow"); x >= 0 {
				h.click(x+1, y)
				clicked = true
			}
		}
	}
	if !clicked {
		t.Fatal("no Allow button on screen")
	}
	h.redraw()

	h.assertContains("granted=true")
	if err := m.FindPlugin("db").Granted.CheckExec(requestExecPath); err != nil {
		t.Errorf("running plugin not granted: %v", err)
	}
	data, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"execPaths"`) || !strings.Contains(string(data), requestExecPath) {
		t.Errorf("grant not persisted:\n%s", data)
	}
}

func TestPluginRequestExecDismissGrantsNothing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixtures use Unix absolute paths")
	}
	h, m, regPath := newRequestExecHarness(t)
	defer h.stop()

	h.exec("db.ask")
	h.redraw()
	h.assertContains("DB requests")
	h.pressKey(tcell.KeyEscape, 0)
	h.redraw()

	h.assertNotContains("DB requests")
	h.assertContains("granted=false")
	if err := m.FindPlugin("db").Granted.CheckExec(requestExecPath); err == nil {
		t.Error("dismissing the dialog granted the path")
	}
	data, _ := os.ReadFile(regPath)
	if strings.Contains(string(data), requestExecPath) {
		t.Errorf("dismissed request persisted:\n%s", data)
	}
}
