package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestTerminalNavigationWrapsAndFocusesActiveTab(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	for range 2 {
		if _, err := a.SpawnTerminalSpec(TerminalSpec{Shell: "/bin/cat"}); err != nil {
			t.Fatalf("SpawnTerminalSpec: %v", err)
		}
	}
	t.Cleanup(a.CloseAllTerminals)
	if a.TerminalPanel.ActiveIndex() != 1 {
		t.Fatalf("initial active terminal = %d, want 1", a.TerminalPanel.ActiveIndex())
	}

	a.NextTerminal()
	if got := a.TerminalPanel.ActiveIndex(); got != 0 {
		t.Fatalf("next from last terminal = %d, want 0", got)
	}
	if a.Root.Focused != a.TerminalPanel {
		t.Fatal("next did not focus the terminal panel")
	}

	a.PrevTerminal()
	if got := a.TerminalPanel.ActiveIndex(); got != 1 {
		t.Fatalf("previous from first terminal = %d, want 1 (wrap)", got)
	}
	a.NextTerminal()
	if got := a.TerminalPanel.ActiveIndex(); got != 0 {
		t.Fatalf("next from last terminal = %d, want 0 (wrap)", got)
	}
	a.NextTerminal()
	if got := a.TerminalPanel.ActiveIndex(); got != 1 {
		t.Fatalf("next from first terminal = %d, want 1", got)
	}
	a.PrevTerminal()
	if got := a.TerminalPanel.ActiveIndex(); got != 0 {
		t.Fatalf("previous from last terminal = %d, want 0", got)
	}

	for _, id := range []string{"terminal.next", "terminal.prev"} {
		if !config.ForceKeyCommands[id] {
			t.Errorf("%s is not a force-key command", id)
		}
	}
}

func TestTerminalNavigationWithoutTerminalsIsNoop(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.NextTerminal()
	a.PrevTerminal()
	if a.TerminalPanel.Count() != 0 {
		t.Fatalf("terminal count = %d, want 0", a.TerminalPanel.Count())
	}
}
