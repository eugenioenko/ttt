//go:build !windows

package app

import (
	"path/filepath"
	"testing"
)

// The debug state has to say where the keyboard input goes, and which
// terminal tab it goes to, so an agent can address the terminal by id.
func TestDebugStateReportsTheActiveTerminal(t *testing.T) {
	a := spawnTestApp(t)
	dir := t.TempDir()

	first, err := a.SpawnTerminalSpec(markerSpec(dir, filepath.Join(t.TempDir(), "started")))
	if err != nil {
		t.Fatalf("SpawnTerminalSpec: %v", err)
	}
	defer a.CloseTerminal(first)

	a.Root.SetFocus(a.TerminalPanel)

	if got := a.describeFocus(); got != "terminal" {
		t.Errorf("focus = %q, want %q", got, "terminal")
	}

	active := a.activeTerminal()
	if active == nil {
		t.Fatal("no active terminal reported")
	}
	if active.ID != first {
		t.Errorf("active terminal id = %q, want %q", active.ID, first)
	}
	if active.Tab != 0 {
		t.Errorf("active terminal tab = %d, want 0", active.Tab)
	}

	// A second terminal becomes the active one, and the report follows it.
	second, err := a.SpawnTerminalSpec(markerSpec(dir, filepath.Join(t.TempDir(), "started2")))
	if err != nil {
		t.Fatalf("SpawnTerminalSpec: %v", err)
	}
	defer a.CloseTerminal(second)

	active = a.activeTerminal()
	if active == nil {
		t.Fatal("no active terminal reported")
	}
	if active.ID != second {
		t.Errorf("active terminal id = %q, want %q", active.ID, second)
	}
	if active.Tab != 1 {
		t.Errorf("active terminal tab = %d, want 1", active.Tab)
	}

	state := a.BuildDebugState()
	if state.Focus != "terminal" {
		t.Errorf("debug state focus = %q, want %q", state.Focus, "terminal")
	}
	if state.ActiveTerminal == nil {
		t.Fatal("debug state reports no active terminal")
	}
	if state.ActiveTerminal.ID != second || state.ActiveTerminal.Tab != 1 {
		t.Errorf("debug state active terminal = %+v, want id %q tab 1", state.ActiveTerminal, second)
	}

	// Switching back to the first tab must be reported too.
	a.TerminalPanel.SetActive(0)
	state = a.BuildDebugState()
	if state.ActiveTerminal == nil {
		t.Fatal("debug state reports no active terminal")
	}
	if state.ActiveTerminal.ID != first || state.ActiveTerminal.Tab != 0 {
		t.Errorf("debug state active terminal = %+v, want id %q tab 0", state.ActiveTerminal, first)
	}
}

// Focus that has left the terminal must not be reported as the terminal.
func TestDebugStateStopsReportingTerminalFocusOnceFocusMoves(t *testing.T) {
	a := spawnTestApp(t)
	dir := t.TempDir()

	id, err := a.SpawnTerminalSpec(markerSpec(dir, filepath.Join(t.TempDir(), "started")))
	if err != nil {
		t.Fatalf("SpawnTerminalSpec: %v", err)
	}
	defer a.CloseTerminal(id)

	a.Root.SetFocus(a.TerminalPanel)
	if got := a.describeFocus(); got != "terminal" {
		t.Fatalf("focus = %q, want %q", got, "terminal")
	}

	a.FocusEditor()
	if got := a.describeFocus(); got == "terminal" {
		t.Error("focus still reported as the terminal after it moved to the editor")
	}
	if state := a.BuildDebugState(); state.Focus == "terminal" {
		t.Errorf("debug state focus = %q, want it to have left the terminal", state.Focus)
	}
}

// Without a terminal there is nothing to report.
func TestDebugStateHasNoActiveTerminalWithoutOne(t *testing.T) {
	a := spawnTestApp(t)

	if active := a.activeTerminal(); active != nil {
		t.Errorf("active terminal = %+v, want none", active)
	}
}
