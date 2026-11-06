package e2e

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestFindUndoDoesNotMutateEditor(t *testing.T) {
	h := newTestHarness(t, 100, 25)
	defer h.stop()

	h.exec("file.new")
	pasteText(h, pasteKeys("initial editor content"))
	h.redraw()

	origText := editorText(h)
	if !strings.Contains(origText, "initial editor content") {
		t.Fatalf("expected initial text, got %q", origText)
	}

	// Open Find bar
	h.exec("search.find")
	h.redraw()

	find, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatalf("expected FindBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}

	// Type in Find bar
	for _, r := range "search term" {
		h.pressRune(r)
	}
	if find.Input.Text != "search term" {
		t.Fatalf("expected 'search term' in find input, got %q", find.Input.Text)
	}

	// Ctrl+Z in Find bar should undo find input, NOT the editor
	h.pressKey(tcell.KeyCtrlZ, tcell.ModCtrl)
	if find.Input.Text != "search" {
		t.Fatalf("expected 'search' after undoing ' term', got %q", find.Input.Text)
	}
	if editorText(h) != origText {
		t.Fatalf("editor buffer was modified by Ctrl+Z in find bar! got %q, expected %q", editorText(h), origText)
	}

	// Undo again
	h.pressKey(tcell.KeyCtrlZ, tcell.ModCtrl)
	if find.Input.Text != "" {
		t.Fatalf("expected '' after second undo, got %q", find.Input.Text)
	}
	if editorText(h) != origText {
		t.Fatalf("editor buffer was modified by Ctrl+Z in find bar! got %q, expected %q", editorText(h), origText)
	}

	// Redo with Ctrl+Y
	h.pressKey(tcell.KeyCtrlY, tcell.ModCtrl)
	if find.Input.Text != "search" {
		t.Fatalf("expected 'search' after redo, got %q", find.Input.Text)
	}
	h.pressKey(tcell.KeyCtrlY, tcell.ModCtrl)
	if find.Input.Text != "search term" {
		t.Fatalf("expected 'search term' after redo, got %q", find.Input.Text)
	}
	if editorText(h) != origText {
		t.Fatalf("editor buffer was modified by Ctrl+Y in find bar! got %q, expected %q", editorText(h), origText)
	}

	// Undo back to empty
	h.pressKey(tcell.KeyCtrlZ, tcell.ModCtrl)
	h.pressKey(tcell.KeyCtrlZ, tcell.ModCtrl)
	if find.Input.Text != "" {
		t.Fatalf("expected empty find input, got %q", find.Input.Text)
	}

	// Crucial test for Issue #1377:
	// Pressing Ctrl+Z when find input has no more history to undo MUST NOT fall through to editor.undo!
	h.pressKey(tcell.KeyCtrlZ, tcell.ModCtrl)
	if editorText(h) != origText {
		t.Fatalf("editor buffer was mutated by extra Ctrl+Z while find was focused! got %q, expected %q", editorText(h), origText)
	}

	// Dismiss Find bar and verify editor undo still works when editor is focused
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	h.redraw()
	if h.app.Root.HasOverlay() {
		t.Fatal("expected find bar overlay to be dismissed")
	}

	// In editor, Ctrl+Z should undo the pasted text
	h.pressKey(tcell.KeyCtrlZ, tcell.ModCtrl)
	if editorText(h) != "" {
		t.Fatalf("expected editor undo to work when editor is focused, got %q", editorText(h))
	}
}

func TestReplaceUndoDoesNotMutateEditor(t *testing.T) {
	h := newTestHarness(t, 100, 25)
	defer h.stop()

	h.exec("file.new")
	pasteText(h, pasteKeys("editor line"))
	h.redraw()

	origText := editorText(h)

	// Open Replace bar
	h.exec("search.replace")
	h.redraw()

	replace, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatalf("expected ReplaceBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}

	// Row 0 is SearchInput: type and undo
	for _, r := range "findme" {
		h.pressRune(r)
	}
	if replace.SearchInput.Text != "findme" {
		t.Fatalf("expected 'findme', got %q", replace.SearchInput.Text)
	}

	h.pressKey(tcell.KeyCtrlZ, tcell.ModCtrl)
	if replace.SearchInput.Text != "" {
		t.Fatalf("expected '', got %q", replace.SearchInput.Text)
	}
	if editorText(h) != origText {
		t.Fatalf("editor was mutated by Ctrl+Z in replace bar! got %q", editorText(h))
	}

	h.pressKey(tcell.KeyCtrlY, tcell.ModCtrl)
	if replace.SearchInput.Text != "findme" {
		t.Fatalf("expected 'findme' after redo, got %q", replace.SearchInput.Text)
	}

	// Tab to ReplaceInput (row 1)
	h.pressKey(tcell.KeyTab, tcell.ModNone)
	for _, r := range "replaceme" {
		h.pressRune(r)
	}
	if replace.ReplaceInput.Text != "replaceme" {
		t.Fatalf("expected 'replaceme', got %q", replace.ReplaceInput.Text)
	}

	h.pressKey(tcell.KeyCtrlZ, tcell.ModCtrl)
	if replace.ReplaceInput.Text != "" {
		t.Fatalf("expected '' after undo in ReplaceInput, got %q", replace.ReplaceInput.Text)
	}
	if editorText(h) != origText {
		t.Fatalf("editor was mutated by Ctrl+Z in ReplaceInput! got %q", editorText(h))
	}

	h.pressKey(tcell.KeyCtrlY, tcell.ModCtrl)
	if replace.ReplaceInput.Text != "replaceme" {
		t.Fatalf("expected 'replaceme' after redo, got %q", replace.ReplaceInput.Text)
	}
}

func TestGlobalSearchUndoDoesNotMutateEditor(t *testing.T) {
	h := newTestHarness(t, 100, 25)
	defer h.stop()

	h.exec("file.new")
	pasteText(h, pasteKeys("editor content"))
	h.redraw()

	origText := editorText(h)

	// Open sidebar search
	h.exec("sidebar.search")
	h.redraw()

	if h.app.Search == nil || h.app.Search.Input == nil {
		t.Fatal("expected search widget")
	}

	for _, r := range "needle" {
		h.pressRune(r)
	}
	if h.app.Search.Input.Text != "needle" {
		t.Fatalf("expected 'needle', got %q", h.app.Search.Input.Text)
	}

	h.pressKey(tcell.KeyCtrlZ, tcell.ModCtrl)
	if h.app.Search.Input.Text != "" {
		t.Fatalf("expected '' after undo in search, got %q", h.app.Search.Input.Text)
	}
	if editorText(h) != origText {
		t.Fatalf("editor was mutated by Ctrl+Z in sidebar search! got %q", editorText(h))
	}

	h.pressKey(tcell.KeyCtrlY, tcell.ModCtrl)
	if h.app.Search.Input.Text != "needle" {
		t.Fatalf("expected 'needle' after redo in search, got %q", h.app.Search.Input.Text)
	}
}
