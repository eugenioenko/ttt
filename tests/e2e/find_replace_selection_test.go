package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
)

func TestFindPrefilledWithSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("hello world hello"))

	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 6)
	editor.Cursor.Line = 0
	editor.Cursor.Col = 11
	editor.Selection.Active = true

	h.exec("search.find")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatalf("expected FindBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.Input.Text != "world" {
		t.Fatalf("find input = %q, want %q", bar.Input.Text, "world")
	}
	if len(bar.Matches) != 1 || bar.Matches[0].Col != 6 {
		t.Fatalf("unexpected matches: %+v", bar.Matches)
	}
}

func TestReplacePrefilledWithSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("apple banana apple"))

	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 0)
	editor.Cursor.Line = 0
	editor.Cursor.Col = 5
	editor.Selection.Active = true

	h.exec("search.replace")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatalf("expected ReplaceBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.SearchInput.Text != "apple" {
		t.Fatalf("replace search input = %q, want %q", bar.SearchInput.Text, "apple")
	}
	if len(bar.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(bar.Matches))
	}
}

func TestFindRefocusUpdatesSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("alpha beta gamma"))

	// Open Find initially without selection
	h.exec("search.find")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatalf("expected FindBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.Input.Text != "" {
		t.Fatalf("initial find input = %q, want empty", bar.Input.Text)
	}

	// Click editor, select "beta", then re-run search.find
	h.click(6, 4)
	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 6)
	editor.Cursor.Line = 0
	editor.Cursor.Col = 10
	editor.Selection.Active = true

	h.exec("search.find")
	if bar.Input.Text != "beta" {
		t.Fatalf("find input after refocus = %q, want %q", bar.Input.Text, "beta")
	}
	if len(bar.Matches) != 1 || bar.Matches[0].Col != 6 {
		t.Fatalf("unexpected matches after refocus: %+v", bar.Matches)
	}
}

func TestReplaceRefocusUpdatesSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("cat dog cat"))

	// Open Replace initially without selection
	h.exec("search.replace")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatalf("expected ReplaceBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.SearchInput.Text != "" {
		t.Fatalf("initial replace search input = %q, want empty", bar.SearchInput.Text)
	}

	// Click editor, select "cat", then re-run search.replace
	h.click(6, 4)
	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 0)
	editor.Cursor.Line = 0
	editor.Cursor.Col = 3
	editor.Selection.Active = true

	h.exec("search.replace")
	if bar.SearchInput.Text != "cat" {
		t.Fatalf("replace search input after refocus = %q, want %q", bar.SearchInput.Text, "cat")
	}
	if len(bar.Matches) != 2 {
		t.Fatalf("expected 2 matches after refocus, got %d", len(bar.Matches))
	}
}

func TestFindMultilineSelectionIgnored(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("first line\nsecond line"))

	// Multi-line selection spanning line 0 to line 1
	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 0)
	editor.Cursor.Line = 1
	editor.Cursor.Col = 5
	editor.Selection.Active = true

	h.exec("search.find")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatalf("expected FindBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.Input.Text != "" {
		t.Fatalf("multi-line selection should not populate single-line input, got %q", bar.Input.Text)
	}
}
