package e2e

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func pasteText(h *testHarness, events []*tcell.EventKey) {
	h.t.Helper()
	text := term.CollectPasteText(events)
	if text != "" {
		h.app.PasteText(text)
		h.flushOnChange()
		h.redraw()
	}
}

func pasteKeys(runes string) []*tcell.EventKey {
	var events []*tcell.EventKey
	for _, r := range runes {
		if r == '\n' {
			events = append(events, tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
		} else {
			events = append(events, tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
		}
	}
	return events
}

func TestBracketedPasteSimple(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.exec("file.new")

	pasteText(h, pasteKeys("hi"))

	buf := h.app.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatal("expected active buffer")
	}
	if buf.Lines[0] != "hi" {
		t.Errorf("expected 'hi', got %q", buf.Lines[0])
	}
}

func TestBracketedPasteIntoFindBar(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.exec("file.new")
	h.exec("search.find")
	bar := findBarWidget(t, h)

	pasteText(h, pasteKeys("https://github.com/eugenioenko/ttt"))
	if got := bar.Input.Text; got != "https://github.com/eugenioenko/ttt" {
		t.Fatalf("find input = %q", got)
	}
	if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != "" {
		t.Fatalf("paste leaked into editor: %q", got)
	}
	clipboard.DisableSystem()
	clipboard.Set("-clipboard")
	h.exec("editor.paste")
	if got := bar.Input.Text; got != "https://github.com/eugenioenko/ttt-clipboard" {
		t.Fatalf("find input after clipboard command = %q", got)
	}
	if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != "" {
		t.Fatalf("clipboard command leaked into editor: %q", got)
	}

	h.click(5, 3)
	h.app.PasteText("XYZ")
	if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != "XYZ" {
		t.Fatalf("editor paste after click = %q", got)
	}
}

func TestBracketedPasteMultiLine(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.exec("file.new")

	pasteText(h, pasteKeys("line1\nline2\nline3"))

	buf := h.app.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatal("expected active buffer")
	}
	if len(buf.Lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d: %v", len(buf.Lines), buf.Lines)
	}
	if buf.Lines[0] != "line1" {
		t.Errorf("line 0: expected 'line1', got %q", buf.Lines[0])
	}
	if buf.Lines[1] != "line2" {
		t.Errorf("line 1: expected 'line2', got %q", buf.Lines[1])
	}
	if buf.Lines[2] != "line3" {
		t.Errorf("line 2: expected 'line3', got %q", buf.Lines[2])
	}
}

func TestBracketedPasteIntoDialog(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.exec("file.new")

	// Open Save As dialog (new file has no path, so file.save opens Save As)
	h.exec("file.save")
	h.redraw()

	// Paste a path into the dialog via bracketed paste
	h.app.PasteText("/tmp/dialog-paste-test.txt")
	h.redraw()

	text := h.screenText()
	if !strings.Contains(text, "dialog-paste-test.txt") {
		t.Errorf("pasted text not visible in dialog; screen:\n%s", text)
	}

	// Editor buffer should be untouched
	buf := h.app.EditorGroup.ActiveBuffer()
	if buf != nil && len(buf.Lines) > 0 && buf.Lines[0] != "" {
		t.Errorf("paste leaked into editor: %q", buf.Lines[0])
	}
}

func TestBracketedPasteKeyboardWorksAfter(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.exec("file.new")

	pasteText(h, pasteKeys("A"))
	h.pressRune('B')

	buf := h.app.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatal("expected active buffer")
	}
	if buf.Lines[0] != "AB" {
		t.Errorf("expected 'AB', got %q", buf.Lines[0])
	}
}

func TestBracketedPasteIntoReplaceBar(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.exec("file.new")
	h.exec("search.replace")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatalf("expected ReplaceBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}

	pasteText(h, pasteKeys("old"))
	if got := bar.SearchInput.Text; got != "old" {
		t.Fatalf("search input = %q", got)
	}
	h.pressKey(tcell.KeyTab, tcell.ModNone)
	pasteText(h, pasteKeys("new"))
	if got := bar.ReplaceInput.Text; got != "new" {
		t.Fatalf("replace input = %q", got)
	}
	clipboard.Set("er")
	h.exec("editor.paste")
	if got := bar.ReplaceInput.Text; got != "newer" {
		t.Fatalf("replace input after clipboard command = %q", got)
	}
	if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != "" {
		t.Fatalf("paste leaked into editor: %q", got)
	}
}
