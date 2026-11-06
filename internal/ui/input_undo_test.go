package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func typeInputRunes(inp *InputWidget, str string) {
	for _, r := range str {
		inp.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
}

func pressKey(inp *InputWidget, key tcell.Key, mod tcell.ModMask) EventResult {
	return inp.HandleEvent(tcell.NewEventKey(key, "", mod))
}

func TestInputUndoRedoTyping(t *testing.T) {
	inp := NewInputWidget()
	typeInputRunes(inp, "hello")
	if inp.Text != "hello" {
		t.Fatalf("expected 'hello', got %q", inp.Text)
	}

	// Undo typing
	res := pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed, got %v", res)
	}
	if inp.Text != "" {
		t.Fatalf("expected empty text after undo, got %q", inp.Text)
	}

	// Redo with Ctrl+Y
	res = pressKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed, got %v", res)
	}
	if inp.Text != "hello" {
		t.Fatalf("expected 'hello' after redo, got %q", inp.Text)
	}

	// Undo again
	pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text != "" {
		t.Fatalf("expected empty text after second undo, got %q", inp.Text)
	}

	// Redo with Ctrl+Shift+Z
	res = pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl|tcell.ModShift)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed for Ctrl+Shift+Z, got %v", res)
	}
	if inp.Text != "hello" {
		t.Fatalf("expected 'hello' after redo with Ctrl+Shift+Z, got %q", inp.Text)
	}
}

func TestInputUndoRedoWordGrouping(t *testing.T) {
	inp := NewInputWidget()
	typeInputRunes(inp, "foo bar")
	if inp.Text != "foo bar" {
		t.Fatalf("expected 'foo bar', got %q", inp.Text)
	}

	// First undo should undo " bar"
	pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text != "foo" {
		t.Fatalf("expected 'foo', got %q", inp.Text)
	}

	// Second undo should undo "foo"
	pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text != "" {
		t.Fatalf("expected '', got %q", inp.Text)
	}

	// Redo restores "foo"
	pressKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if inp.Text != "foo" {
		t.Fatalf("expected 'foo', got %q", inp.Text)
	}

	// Redo restores "foo bar"
	pressKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if inp.Text != "foo bar" {
		t.Fatalf("expected 'foo bar', got %q", inp.Text)
	}
}

func TestInputUndoRedoBackspace(t *testing.T) {
	inp := NewInputWidget()
	typeInputRunes(inp, "hello")
	pressKey(inp, tcell.KeyBackspace, tcell.ModNone)
	pressKey(inp, tcell.KeyBackspace, tcell.ModNone)
	if inp.Text != "hel" {
		t.Fatalf("expected 'hel', got %q", inp.Text)
	}

	// Undo restores backspaced characters
	pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text != "hello" {
		t.Fatalf("expected 'hello' after undoing backspace, got %q", inp.Text)
	}

	// Redo reapplies backspace
	pressKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if inp.Text != "hel" {
		t.Fatalf("expected 'hel' after redoing backspace, got %q", inp.Text)
	}
}

func TestInputUndoCursorMovementBreaksGroup(t *testing.T) {
	inp := NewInputWidget()
	typeInputRunes(inp, "abc")
	pressKey(inp, tcell.KeyLeft, tcell.ModNone) // Cursor moved before 'c'
	typeInputRunes(inp, "d")
	if inp.Text != "abdc" {
		t.Fatalf("expected 'abdc', got %q", inp.Text)
	}

	// Undo should undo 'd' insertion first
	pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text != "abc" {
		t.Fatalf("expected 'abc' after first undo, got %q", inp.Text)
	}

	// Next undo should undo "abc"
	pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text != "" {
		t.Fatalf("expected '' after second undo, got %q", inp.Text)
	}
}

func TestInputUndoSelectionReplacement(t *testing.T) {
	inp := NewInputWidget()
	typeInputRunes(inp, "hello world")
	inp.selStart = 6
	inp.selEnd = 11
	inp.CursorPos = 11

	typeInputRunes(inp, "there")
	if inp.Text != "hello there" {
		t.Fatalf("expected 'hello there', got %q", inp.Text)
	}

	// Undo restores original text with selection
	pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text != "hello world" {
		t.Fatalf("expected 'hello world' after undo, got %q", inp.Text)
	}

	// Redo restores replacement
	pressKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if inp.Text != "hello there" {
		t.Fatalf("expected 'hello there' after redo, got %q", inp.Text)
	}
}

func TestInputUndoEmptyStackAlwaysConsumed(t *testing.T) {
	inp := NewInputWidget()
	res := pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed even on empty undo stack, got %v", res)
	}
	if inp.Text != "" {
		t.Fatalf("expected empty text, got %q", inp.Text)
	}

	res = pressKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed on empty redo stack, got %v", res)
	}
}

func TestInputUndoClearAndSetText(t *testing.T) {
	inp := NewInputWidget()
	typeInputRunes(inp, "sample")
	inp.Clear()
	if inp.Text != "" {
		t.Fatalf("expected empty after Clear, got %q", inp.Text)
	}

	// Undo should restore "sample"
	pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text != "sample" {
		t.Fatalf("expected 'sample' after undoing Clear, got %q", inp.Text)
	}

	// SetText resets undo history
	inp.SetText("fixed")
	if inp.Text != "fixed" {
		t.Fatalf("expected 'fixed', got %q", inp.Text)
	}
	pressKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text != "fixed" {
		t.Fatalf("expected 'fixed' unchanged after undo, got %q", inp.Text)
	}
}

func TestInputCopyTextStatePreservesUndo(t *testing.T) {
	source := NewInputWidget()
	typeInputRunes(source, "query")

	dest := NewInputWidget()
	dest.CopyTextState(source)
	if dest.Text != "query" {
		t.Fatalf("expected 'query', got %q", dest.Text)
	}

	// dest can undo what source typed
	pressKey(dest, tcell.KeyCtrlZ, tcell.ModCtrl)
	if dest.Text != "" {
		t.Fatalf("expected '' after undo on dest, got %q", dest.Text)
	}

	// dest can redo
	pressKey(dest, tcell.KeyCtrlY, tcell.ModCtrl)
	if dest.Text != "query" {
		t.Fatalf("expected 'query' after redo on dest, got %q", dest.Text)
	}
}
