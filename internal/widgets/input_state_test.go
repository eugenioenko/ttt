package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestInputUndoRedoTyping(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetFocused(true)
	typeWidgetInputRunes(inp, "hello")
	if inp.Text() != "hello" {
		t.Fatalf("expected 'hello', got %q", inp.Text())
	}

	res := pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed, got %v", res)
	}
	if inp.Text() != "" {
		t.Fatalf("expected empty text after undo, got %q", inp.Text())
	}

	res = pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed, got %v", res)
	}
	if inp.Text() != "hello" {
		t.Fatalf("expected 'hello' after redo, got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "" {
		t.Fatalf("expected empty text after second undo, got %q", inp.Text())
	}

	res = pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl|tcell.ModShift)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed for Ctrl+Shift+Z, got %v", res)
	}
	if inp.Text() != "hello" {
		t.Fatalf("expected 'hello' after redo with Ctrl+Shift+Z, got %q", inp.Text())
	}
}

func TestInputUndoRedoWordGrouping(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetFocused(true)
	typeWidgetInputRunes(inp, "foo bar")
	if inp.Text() != "foo bar" {
		t.Fatalf("expected 'foo bar', got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "foo" {
		t.Fatalf("expected 'foo', got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "" {
		t.Fatalf("expected '', got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if inp.Text() != "foo" {
		t.Fatalf("expected 'foo', got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if inp.Text() != "foo bar" {
		t.Fatalf("expected 'foo bar', got %q", inp.Text())
	}
}

func TestInputUndoRedoBackspace(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetFocused(true)
	typeWidgetInputRunes(inp, "hello")
	pressWidgetKey(inp, tcell.KeyBackspace, tcell.ModNone)
	pressWidgetKey(inp, tcell.KeyBackspace, tcell.ModNone)
	if inp.Text() != "hel" {
		t.Fatalf("expected 'hel', got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "hello" {
		t.Fatalf("expected 'hello' after undoing backspace, got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if inp.Text() != "hel" {
		t.Fatalf("expected 'hel' after redoing backspace, got %q", inp.Text())
	}
}

func TestInputUndoCursorMovementBreaksGroup(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetFocused(true)
	typeWidgetInputRunes(inp, "abc")
	pressWidgetKey(inp, tcell.KeyLeft, tcell.ModNone)
	typeWidgetInputRunes(inp, "d")
	if inp.Text() != "abdc" {
		t.Fatalf("expected 'abdc', got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "abc" {
		t.Fatalf("expected 'abc' after first undo, got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "" {
		t.Fatalf("expected '' after second undo, got %q", inp.Text())
	}
}

func TestInputUndoSelectionReplacement(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetFocused(true)
	typeWidgetInputRunes(inp, "hello world")
	inp.selStart = 6
	inp.selEnd = 11
	inp.cursorPos = 11

	typeWidgetInputRunes(inp, "there")
	if inp.Text() != "hello there" {
		t.Fatalf("expected 'hello there', got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "hello world" {
		t.Fatalf("expected 'hello world' after undo, got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if inp.Text() != "hello there" {
		t.Fatalf("expected 'hello there' after redo, got %q", inp.Text())
	}
}

func TestInputUndoEmptyStackAlwaysConsumed(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetFocused(true)
	res := pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed even on empty undo stack, got %v", res)
	}
	if inp.Text() != "" {
		t.Fatalf("expected empty text, got %q", inp.Text())
	}

	res = pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed on empty redo stack, got %v", res)
	}
}

func TestInputUndoClearAndSetText(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetFocused(true)
	typeWidgetInputRunes(inp, "sample")
	inp.Clear()
	if inp.Text() != "" {
		t.Fatalf("expected empty after Clear, got %q", inp.Text())
	}

	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "sample" {
		t.Fatalf("expected 'sample' after undoing Clear, got %q", inp.Text())
	}

	inp.SetText("fixed")
	if inp.Text() != "fixed" {
		t.Fatalf("expected 'fixed', got %q", inp.Text())
	}
	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "fixed" {
		t.Fatalf("expected 'fixed' unchanged after undo, got %q", inp.Text())
	}
}

func TestInputCopyTextStatePreservesUndo(t *testing.T) {
	source := NewInputWidget(InputConfig{})
	source.SetFocused(true)
	typeWidgetInputRunes(source, "query")

	dest := NewInputWidget(InputConfig{})
	dest.SetFocused(true)
	dest.CopyTextState(source)
	if dest.Text() != "query" {
		t.Fatalf("expected 'query', got %q", dest.Text())
	}

	pressWidgetKey(dest, tcell.KeyCtrlZ, tcell.ModCtrl)
	if dest.Text() != "" {
		t.Fatalf("expected '' after undo on dest, got %q", dest.Text())
	}

	pressWidgetKey(dest, tcell.KeyCtrlY, tcell.ModCtrl)
	if dest.Text() != "query" {
		t.Fatalf("expected 'query' after redo on dest, got %q", dest.Text())
	}
}

func TestInputCopyTextStateKeepsIndependentHistoryAndConfiguration(t *testing.T) {
	source := NewInputWidget(InputConfig{Wrap: true})
	source.SetFocused(true)
	typeWidgetInputRunes(source, "one two three")
	source.Undo()
	source.Undo()
	source.selStart, source.selEnd, source.cursorPos = 0, 2, 2
	source.scrollOffset, source.rowOffset, source.goalCol = 1, 2, 3
	changes := 0
	dest := NewInputWidget(InputConfig{Placeholder: "destination", OnChange: func(string) { changes++ }})
	dest.SetFocused(true)
	dest.drag.Begin(1, 1, 0)
	dest.CopyTextState(source)
	if changes != 0 || dest.OwnsPointerCapture() || dest.Config.Placeholder != "destination" || !dest.IsFocused() {
		t.Fatal("copy changed destination configuration, notified, or kept drag active")
	}
	if dest.text != source.text || dest.cursorPos != 2 || dest.selStart != 0 || dest.selEnd != 2 || dest.scrollOffset != 1 || dest.rowOffset != 2 || dest.goalCol != 3 {
		t.Fatal("copy lost selection, cursor, or scroll state")
	}
	source.Redo()
	source.Redo()
	dest.CopyTextState(source)
	dest.Undo()
	dest.Undo()
	typeWidgetInputRunes(dest, "X")
	pressWidgetKey(dest, tcell.KeyLeft, tcell.ModNone)
	typeWidgetInputRunes(dest, "Y")
	source.Undo()
	if source.Text() != "one two" {
		t.Fatalf("destination edits overwrote source undo history: %q", source.Text())
	}
	source.Undo()
	if source.Text() != "one" {
		t.Fatalf("destination edits overwrote earlier source history: %q", source.Text())
	}
	source.Redo()
	source.Redo()
	if source.Text() != "one two three" {
		t.Fatalf("source redo history was lost: %q", source.Text())
	}
}
