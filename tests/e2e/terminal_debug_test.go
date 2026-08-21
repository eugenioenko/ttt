package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
)

// TestTerminalDebugStateExposesRawPTYTail drives a real PTY through the
// integrated terminal and confirms the `debug` command's JSON dump surfaces
// the raw bytes the child process wrote, independent of ttt's own parsed
// terminal state -- ground truth for diagnosing terminal-emulation bugs.
func TestTerminalDebugStateExposesRawPTYTail(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	// cat echoes whatever is written to the PTY, giving deterministic output.
	h.app.Settings.Terminal.Shell = "/bin/cat"
	h.exec("terminal.toggle")
	if len(h.app.Terminals) != 1 {
		t.Fatalf("expected 1 terminal after toggle, got %d", len(h.app.Terminals))
	}
	tab := h.app.Terminals[0]
	defer tab.Term.Close()

	const marker = "raw_tail_debug_marker"
	tab.Term.WriteString(marker + "\n")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(string(tab.Term.RawTail()), marker) {
		time.Sleep(30 * time.Millisecond)
	}
	if !strings.Contains(string(tab.Term.RawTail()), marker) {
		t.Fatal("marker never appeared in raw tail")
	}

	path := filepath.Join(h.dir, "debug.json")
	if err := h.app.DumpDebugState(path); err != nil {
		t.Fatalf("DumpDebugState: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var state app.DebugState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal debug state: %v", err)
	}

	if len(state.Terminals) != 1 {
		t.Fatalf("expected 1 terminal in debug state, got %d", len(state.Terminals))
	}
	dt := state.Terminals[0]
	if dt.ID != tab.ID {
		t.Errorf("terminal ID = %q, want %q", dt.ID, tab.ID)
	}
	if dt.Cols == 0 || dt.Rows == 0 {
		t.Errorf("terminal size = %dx%d, want nonzero", dt.Cols, dt.Rows)
	}
	if !strings.Contains(dt.RawTail, marker) {
		t.Errorf("raw_tail = %q, want it to contain %q", dt.RawTail, marker)
	}
}
