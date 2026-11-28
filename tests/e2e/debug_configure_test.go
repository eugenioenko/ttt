package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/config/debugadapter"
	"github.com/eugenioenko/ttt/internal/dap"
)

func TestDebugConfigureWithoutAdaptersCreatesNothing(t *testing.T) {
	h := newTestHarness(t, 160, 32)
	defer h.stop()
	h.exec("debug.configure")
	h.assertContains("No debuggers are configured")
	if _, err := os.Stat(filepath.Join(h.dir, ".ttt", "launch.json")); !os.IsNotExist(err) {
		t.Fatalf("launch.json created without adapters: %v", err)
	}
}

func TestDebugConfigurePicksDetectedAdapter(t *testing.T) {
	h := newTestHarness(t, 120, 32)
	defer h.stop()
	h.app.Settings.Debug.Adapters["go"] = debugadapter.Config{
		Languages:      []string{"go"},
		Markers:        []string{"go.mod"},
		Configurations: []map[string]any{{"name": "Launch package", "request": "launch", "program": "${workspaceFolder}"}},
	}
	h.app.Settings.Debug.Adapters["node"] = debugadapter.Config{Languages: []string{"javascript"}}
	if err := os.WriteFile(filepath.Join(h.dir, "go.mod"), []byte("module example\n"), 0644); err != nil {
		t.Fatal(err)
	}
	launch := filepath.Join(h.dir, ".ttt", "launch.json")

	h.exec("debug.configure")
	h.assertContains("Create launch.json")
	h.assertContains("go (detected)")
	h.assertContains("node")
	h.pressKey(tcell.KeyEscape, 0)
	if _, err := os.Stat(launch); !os.IsNotExist(err) {
		t.Fatalf("dismissing the picker created launch.json: %v", err)
	}

	h.exec("debug.configure")
	h.pressKey(tcell.KeyEnter, 0)
	targets, err := dap.LoadTargets(h.dir)
	if err != nil || len(targets) != 1 {
		t.Fatalf("targets = %v, %v", targets, err)
	}
	if got := targets[0]; got.Name != "Launch package" || got.Type != "go" {
		t.Errorf("target = %+v", got)
	}
	if h.app.EditorGroup.ActiveFilePath() != launch {
		t.Errorf("active file = %q, want %q", h.app.EditorGroup.ActiveFilePath(), launch)
	}
}
