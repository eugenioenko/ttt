package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func typeWidgetInputRunes(inp *InputWidget, str string) {
	for _, r := range str {
		inp.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
}

func pressWidgetKey(inp *InputWidget, key tcell.Key, mod tcell.ModMask) EventResult {
	return inp.HandleEvent(tcell.NewEventKey(key, "", mod))
}

func TestWidgetInputUndoRedo(t *testing.T) {
	inp := NewInputWidget(InputConfig{Bordered: true})
	inp.SetFocused(true)

	typeWidgetInputRunes(inp, "hello")
	if inp.Text() != "hello" {
		t.Fatalf("expected 'hello', got %q", inp.Text())
	}

	// Undo
	res := pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed, got %v", res)
	}
	if inp.Text() != "" {
		t.Fatalf("expected '', got %q", inp.Text())
	}

	// Redo
	res = pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed, got %v", res)
	}
	if inp.Text() != "hello" {
		t.Fatalf("expected 'hello', got %q", inp.Text())
	}

	// Ctrl+Shift+Z redo
	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "" {
		t.Fatalf("expected '', got %q", inp.Text())
	}
	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl|tcell.ModShift)
	if inp.Text() != "hello" {
		t.Fatalf("expected 'hello', got %q", inp.Text())
	}
}

func TestWidgetInputUndoGroupingAndBackspace(t *testing.T) {
	inp := NewInputWidget(InputConfig{Bordered: true})
	inp.SetFocused(true)

	typeWidgetInputRunes(inp, "one two")
	if inp.Text() != "one two" {
		t.Fatalf("expected 'one two', got %q", inp.Text())
	}

	// Undo word " two"
	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "one" {
		t.Fatalf("expected 'one', got %q", inp.Text())
	}

	// Undo "one"
	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "" {
		t.Fatalf("expected '', got %q", inp.Text())
	}

	// Redo both
	pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if inp.Text() != "one two" {
		t.Fatalf("expected 'one two', got %q", inp.Text())
	}

	// Backspace
	pressWidgetKey(inp, tcell.KeyBackspace, tcell.ModNone)
	if inp.Text() != "one tw" {
		t.Fatalf("expected 'one tw', got %q", inp.Text())
	}
	pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if inp.Text() != "one two" {
		t.Fatalf("expected 'one two' after undoing backspace, got %q", inp.Text())
	}
}

func TestWidgetInputEmptyUndoConsumed(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetFocused(true)

	res := pressWidgetKey(inp, tcell.KeyCtrlZ, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed on empty undo, got %v", res)
	}

	res = pressWidgetKey(inp, tcell.KeyCtrlY, tcell.ModCtrl)
	if res != EventConsumed {
		t.Fatalf("expected EventConsumed on empty redo, got %v", res)
	}
}
