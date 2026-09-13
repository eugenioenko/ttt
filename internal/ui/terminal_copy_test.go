package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/gdamore/tcell/v3"
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

func TestTerminalWidget_CopySelection(t *testing.T) {
	clipboard.DisableSystem()
	term, err := terminal.New("/bin/cat", 20, 5, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	term.Run()

	term.WriteString("hello world\n")
	waitForTerminalOutput(t, term, "hello world")

	tw := NewTerminalWidget(term, nil)
	tw.SetRect(Rect{X: 0, Y: 0, W: 20, H: 5})

	if tw.HasSelection() {
		t.Error("expected no selection initially")
	}
	if tw.CopySelection() {
		t.Error("expected CopySelection() to return false when no selection")
	}

	tw.hasSelection = true
	tw.selAnchor = termSelPos{Line: 0, Col: 0}
	tw.selCurrent = termSelPos{Line: 0, Col: 5}

	if !tw.HasSelection() {
		t.Error("expected HasSelection() to be true")
	}

	if !tw.CopySelection() {
		t.Error("expected CopySelection() to return true")
	}
	if tw.HasSelection() {
		t.Error("expected selection to be cleared after copy")
	}
	if got := clipboard.Get(); got != "hello" {
		t.Errorf("clipboard.Get() = %q, want %q", got, "hello")
	}
}

func TestTerminalWidget_HandleEventCopyAndPaste(t *testing.T) {
	clipboard.DisableSystem()
	term, err := terminal.New("/bin/cat", 20, 5, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	term.Run()

	term.WriteString("testing copy paste\n")
	waitForTerminalOutput(t, term, "testing copy paste")

	tw := NewTerminalWidget(term, nil)
	tw.SetRect(Rect{X: 0, Y: 0, W: 20, H: 5})

	tw.hasSelection = true
	tw.selAnchor = termSelPos{Line: 0, Col: 0}
	tw.selCurrent = termSelPos{Line: 0, Col: 7}

	ctrlC := tcell.NewEventKey(tcell.KeyCtrlC, "", tcell.ModCtrl)
	if res := tw.HandleEvent(ctrlC); res != EventConsumed {
		t.Errorf("HandleEvent(Ctrl+C) = %v, want EventConsumed", res)
	}
	if got := clipboard.Get(); got != "testing" {
		t.Errorf("clipboard.Get() = %q, want %q", got, "testing")
	}

	tw.hasSelection = true
	tw.selAnchor = termSelPos{Line: 0, Col: 8}
	tw.selCurrent = termSelPos{Line: 0, Col: 12}
	ctrlShiftC := tcell.NewEventKey(tcell.KeyRune, "C", tcell.ModCtrl|tcell.ModShift)
	if res := tw.HandleEvent(ctrlShiftC); res != EventConsumed {
		t.Errorf("HandleEvent(Ctrl+Shift+C) = %v, want EventConsumed", res)
	}
	if got := clipboard.Get(); got != "copy" {
		t.Errorf("clipboard.Get() = %q, want %q", got, "copy")
	}

	clipboard.Set("terminal-paste-marker")
	ctrlV := tcell.NewEventKey(tcell.KeyCtrlV, "", tcell.ModCtrl)
	if res := tw.HandleEvent(ctrlV); res != EventConsumed {
		t.Errorf("HandleEvent(Ctrl+V) = %v, want EventConsumed", res)
	}
	waitForTerminalOutput(t, term, "terminal-paste-marker")

	clipboard.Set("terminal-shift-paste-marker")
	ctrlShiftV := tcell.NewEventKey(tcell.KeyRune, "V", tcell.ModCtrl|tcell.ModShift)
	if res := tw.HandleEvent(ctrlShiftV); res != EventConsumed {
		t.Errorf("HandleEvent(Ctrl+Shift+V) = %v, want EventConsumed", res)
	}
	waitForTerminalOutput(t, term, "terminal-shift-paste-marker")

	tw.hasSelection = true
	clipboard.Set("")
	if res := tw.HandleEvent(ctrlV); res != EventConsumed {
		t.Errorf("HandleEvent(Ctrl+V empty clipboard) = %v, want EventConsumed", res)
	}
	if tw.HasSelection() {
		t.Error("expected selection to be cleared when pasting with empty clipboard")
	}
}
