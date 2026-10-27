package ui

import (
	"fmt"
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

func TestTerminalWidget_SelectedTextNegativeBounds(t *testing.T) {
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

	tw.hasSelection = true
	tw.selAnchor = termSelPos{Line: 0, Col: -5}
	tw.selCurrent = termSelPos{Line: 0, Col: -1}

	got := tw.selectedText()
	if got != "" {
		t.Errorf("selectedText() = %q, want empty string", got)
	}

	tw.selAnchor = termSelPos{Line: 0, Col: -5}
	tw.selCurrent = termSelPos{Line: 0, Col: 5}
	got = tw.selectedText()
	if got != "hello" {
		t.Errorf("selectedText() = %q, want %q", got, "hello")
	}
}

func TestTerminalWidget_CtrlVPastes(t *testing.T) {
	clipboard.DisableSystem()
	term, err := terminal.New("/bin/sh", 80, 24, 0, nil, "", "-c",
		"stty raw -echo; printf paste-keys-ready; exec cat")
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	updated := newUpdateChan(term)
	term.Run()
	waitFor(t, updated, func() bool {
		return strings.Contains(string(term.RawTail()), "paste-keys-ready")
	})

	tw := NewTerminalWidget(term, nil)
	if !tw.CtrlVPastes {
		t.Fatal("Ctrl+V must paste by default")
	}
	for _, bracketed := range []bool{false, true} {
		mode := "\x1b[?2004l"
		if bracketed {
			mode = "\x1b[?2004h"
		}
		before := len(term.RawTail())
		term.WriteString(mode)
		waitFor(t, updated, func() bool {
			return len(term.RawTail()) >= before+len(mode) &&
				term.DecPrivateModes().BracketedPasteMode == bracketed
		})
		for _, enabled := range []bool{true, false} {
			for _, key := range []struct {
				name string
				key  tcell.Key
				text string
			}{
				{"control key", tcell.KeyCtrlV, ""},
				{"lowercase rune", tcell.KeyRune, "v"},
				{"uppercase rune", tcell.KeyRune, "V"},
			} {
				for _, shifted := range []bool{false, true} {
					t.Run(fmt.Sprintf("bracketed=%t/enabled=%t/%s/shift=%t", bracketed, enabled, key.name, shifted), func(t *testing.T) {
						tw.CtrlVPastes = enabled
						tw.hasSelection = true
						tw.scrollOffset = 1
						clipboard.Set("clipboard-payload")
						mod := tcell.ModCtrl
						if shifted {
							mod |= tcell.ModShift
						}
						want := "\x16"
						if enabled || shifted {
							want = "clipboard-payload"
							if bracketed {
								want = "\x1b[200~" + want + "\x1b[201~"
							}
						}
						before := len(term.RawTail())
						if got := tw.HandleEvent(tcell.NewEventKey(key.key, key.text, mod)); got != EventConsumed {
							t.Fatalf("HandleEvent = %v, want EventConsumed", got)
						}
						waitFor(t, updated, func() bool {
							return len(term.RawTail()) >= before+len(want)
						})
						if got := string(term.RawTail()[before:]); got != want {
							t.Errorf("PTY received %q, want %q", got, want)
						}
						if tw.HasSelection() || tw.scrollOffset != 0 {
							t.Error("key must clear the selection and return to the live view")
						}
					})
				}
			}
		}
	}
}
