package app

import (
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/ttt/internal/ui"
)

func waitForTerminalOutput(t *testing.T, term *terminal.Terminal, expected string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(string(term.RawTail()), expected) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for terminal output %q", expected)
}

func TestAppCopyAndPaste_TerminalFocused(t *testing.T) {
	clipboard.DisableSystem()
	a := buildTestApp(t, config.DefaultSettings())

	term, err := terminal.New("/bin/cat", 20, 5, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	term.Run()

	tw := ui.NewTerminalWidget(term, nil)
	tw.SetRect(ui.Rect{X: 0, Y: 0, W: 20, H: 5})

	a.TerminalPanel.AddTerminal(tw)
	a.Root.SetFocus(a.TerminalPanel)

	if got := a.focusedTerminalWidget(); got != tw {
		t.Fatalf("focusedTerminalWidget() = %v, want %v", got, tw)
	}

	clipboard.Set("hello from clipboard")
	a.Paste()

	waitForTerminalOutput(t, term, "hello from clipboard")

	clipboard.Set("keep this")
	a.Copy()
	if got := clipboard.Get(); got != "keep this" {
		t.Errorf("clipboard.Get() = %q, want %q (should not copy editor content when terminal focused)", got, "keep this")
	}
}

func TestHandleRightClick_FocusesTerminal(t *testing.T) {
	clipboard.DisableSystem()
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	RegisterCommands(a)

	term, err := terminal.New("/bin/cat", 20, 5, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	term.Run()

	tw := ui.NewTerminalWidget(term, nil)
	tw.SetRect(ui.Rect{X: 0, Y: 0, W: 80, H: 10})
	a.TerminalPanel.AddTerminal(tw)

	a.SplitPanel.SetRect(ui.Rect{X: 0, Y: 0, W: 80, H: 24})
	a.ContentSplit.SetRect(ui.Rect{X: 0, Y: 0, W: 80, H: 24})
	a.ContentSplit.ShowBottom = true
	a.BottomPanel.SetActivePanel("terminal")

	a.Root.SetFocus(a.EditorGroup)

	divY := a.ContentSplit.DividerScreenY()
	handleRightClick(a, 10, divY+2)

	if a.menuReturnFocus != a.TerminalPanel {
		t.Errorf("menuReturnFocus = %v, want %v", a.menuReturnFocus, a.TerminalPanel)
	}

	w := a.Root.TopOverlayWidget()
	if w == nil {
		t.Fatal("expected menu overlay to be open")
	}
	menu, ok := w.(*ui.ContextMenuWidget)
	if !ok {
		t.Fatalf("expected ContextMenuWidget, got %T", w)
	}
	menu.OnExec("editor.paste")

	if got := a.focusedTerminalWidget(); got != tw {
		t.Errorf("focusedTerminalWidget() = %v, want %v", got, tw)
	}
}
