package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"

	"github.com/gdamore/tcell/v3"
)

func TestHomeEndMoveToTheLineEdges(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	openFileEdgeFixture(t, h, "indent.txt", "    hello world\n")
	ed := h.app.EditorGroup.Editor
	ed.Cursor.Line, ed.Cursor.Col = 0, 8

	h.pressKey(tcell.KeyHome, tcell.ModNone)
	if ed.Cursor.Col != 4 {
		t.Fatalf("home = col %d, want 4 at the first non-space", ed.Cursor.Col)
	}
	h.pressKey(tcell.KeyHome, tcell.ModNone)
	if ed.Cursor.Col != 0 {
		t.Fatalf("second home = col %d, want 0", ed.Cursor.Col)
	}

	h.pressKey(tcell.KeyEnd, tcell.ModShift)
	start, end := ed.Selection.Range(ed.Cursor.Line, ed.Cursor.Col)
	if !ed.Selection.Active || start.Col != 0 || end.Col != len("    hello world") {
		t.Fatalf("shift+end selected %v..%v active=%v, want 0:0..0:15", start, end, ed.Selection.Active)
	}
}

func TestEndMovesEveryCursorToItsLineEnd(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	openFileEdgeFixture(t, h, "multi.txt", "ab\nabcd\n")
	ed := h.app.EditorGroup.Editor
	h.exec("multicursor.addCursorBelow")
	if ed.Multi == nil || len(ed.Multi.Cursors) != 2 {
		t.Fatal("fixture: want two cursors")
	}

	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	for _, cs := range ed.Multi.Cursors {
		if want := len([]rune(ed.Buf.Lines[cs.Line])); cs.Col != want {
			t.Fatalf("end left cursor on line %d at col %d, want %d", cs.Line, cs.Col, want)
		}
	}
}

func TestLineEdgeCommandsCanBeRebound(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	openFileEdgeFixture(t, h, "rebind.txt", "    hello world\n")
	ed := h.app.EditorGroup.Editor

	h.app.Keybindings = config.MergeKeybindings(config.DefaultKeybindings(), []config.KeyBinding{
		{Key: "alt+j", Command: "editor.goToLineEnd"},
		{Key: "alt+k", Command: "editor.selectToLineStart"},
	})
	h.app.RebindKeys()

	h.app.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "j", tcell.ModAlt))
	if ed.Cursor.Col != len("    hello world") {
		t.Fatalf("rebound goToLineEnd = col %d, want the line end", ed.Cursor.Col)
	}
	h.app.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "k", tcell.ModAlt))
	start, end := ed.Selection.Range(ed.Cursor.Line, ed.Cursor.Col)
	if !ed.Selection.Active || start.Col != 4 || end.Col != len("    hello world") {
		t.Fatalf("rebound selectToLineStart selected %v..%v active=%v, want 0:4..0:15", start, end, ed.Selection.Active)
	}
}
