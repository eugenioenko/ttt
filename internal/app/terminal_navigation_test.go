package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestTerminalNavigationWrapsAndFocusesActiveTab(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	for range 3 {
		a.TerminalPanel.AddTerminal(ui.NewTerminalWidget(nil, nil))
	}
	a.ContentSplit.ShowBottom = true
	a.TerminalPanel.SetActive(0)
	if a.TerminalPanel.ActiveIndex() != 0 {
		t.Fatalf("initial active terminal = %d, want 0", a.TerminalPanel.ActiveIndex())
	}

	a.FocusEditor()
	a.NextTerminal()
	if got := a.TerminalPanel.ActiveIndex(); got != 1 {
		t.Fatalf("next from first terminal = %d, want 1", got)
	}
	if a.Root.Focused != a.TerminalPanel {
		t.Fatal("next did not focus the terminal panel")
	}

	a.FocusEditor()
	a.PrevTerminal()
	if got := a.TerminalPanel.ActiveIndex(); got != 0 {
		t.Fatalf("previous from second terminal = %d, want 0", got)
	}
	if a.Root.Focused != a.TerminalPanel {
		t.Fatal("previous did not focus the terminal panel")
	}
	if a.TerminalPanel.Count() != 3 {
		t.Fatalf("terminal count = %d, want 3", a.TerminalPanel.Count())
	}
	a.TerminalPanel.SetActive(0)
	a.PrevTerminal()
	if got := a.TerminalPanel.ActiveIndex(); got != 2 {
		t.Fatalf("previous from first terminal = %d, want 2 (wrap)", got)
	}
	a.NextTerminal()
	if got := a.TerminalPanel.ActiveIndex(); got != 0 {
		t.Fatalf("next from last terminal = %d, want 0 (wrap)", got)
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
