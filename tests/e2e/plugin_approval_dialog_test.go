package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
)

const longExecPath = "/opt/homebrew/Cellar/postgresql@16/16.4/libexec/bin/psql"

func showExecApproval(t *testing.T, w, h int) *testHarness {
	t.Helper()
	th := newTestHarness(t, w, h)
	pluginsDir := filepath.Join(th.dir, "config", "plugins")
	dir := filepath.Join(pluginsDir, "dbview")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name": "dbview", "entry": "init.lua", "permissions": {"system.exec": ["sqlite3", "` + longExecPath + `"]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(`require("ttt")`), 0644); err != nil {
		t.Fatal(err)
	}
	m := plugin.NewManager(pluginsDir, filepath.Join(th.dir, "config", "plugins.ttt.json"))
	th.app.PluginManager = m
	t.Cleanup(m.Shutdown)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(pending))
	}
	th.app.ShowPluginApprovalDialog(pending[0])
	th.redraw()
	th.assertContains("dbview requests")
	return th
}

func TestPluginApprovalDialogWidensForLongExecPath(t *testing.T) {
	h := showExecApproval(t, 120, 30)
	defer h.stop()
	h.assertContains(longExecPath)
	h.assertContains("sqlite3")
}

func TestPluginApprovalDialogWrapsExecPathOnNarrowScreen(t *testing.T) {
	h := showExecApproval(t, 60, 30)
	defer h.stop()
	h.assertNotContains(longExecPath)

	var joined strings.Builder
	col := -1
	for y := range 30 {
		row := []rune(h.screenRow(y))
		if col < 0 {
			col = strings.Index(string(row), "/opt/")
			if col < 0 {
				continue
			}
			col = len([]rune(string(row)[:col]))
		}
		if col >= len(row) {
			break
		}
		value, _, _ := strings.Cut(string(row[col:]), "│")
		value = strings.TrimSpace(value)
		if value == "" {
			break
		}
		joined.WriteString(value)
	}
	if joined.String() != longExecPath {
		t.Fatalf("wrapped path reads %q, want %q\n%s", joined.String(), longExecPath, h.screenText())
	}
}
