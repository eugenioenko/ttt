package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

// Helper to create a file, open it in the editor, and return the harness.
func setupAutocompleteTest(t *testing.T, content string) *testHarness {
	t.Helper()
	h := newTestHarness(t, 80, 24)
	f := filepath.Join(h.dir, "ac.txt")
	os.WriteFile(f, []byte(content), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()
	return h
}

func sampleCompletionItems() []ui.CompletionItem {
	return []ui.CompletionItem{
		{Label: "console", InsertText: "console", Kind: ui.CompletionVariable},
		{Label: "const", InsertText: "const", Kind: ui.CompletionKeyword},
		{Label: "constructor", InsertText: "constructor", Kind: ui.CompletionFunction},
	}
}

func TestAutocomplete_BasicAccept(t *testing.T) {
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	// Cursor is at line 0, col 0 on an empty line.
	// Show autocomplete with items (no prefix typed, so all items match).
	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	if h.app.EditorGroup.Autocomplete == nil {
		t.Fatal("expected autocomplete widget to be active")
	}

	// The first item ("console") should be selected by default (index 0).
	// Accept with Enter.
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	// Autocomplete should be dismissed after accept.
	if h.app.EditorGroup.Autocomplete != nil {
		t.Error("expected autocomplete to be dismissed after accept")
	}

	// "console" should be inserted into the buffer.
	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "console" {
		t.Errorf("expected 'console' on line 0, got %q", got)
	}

	// Cursor should be at the end of the inserted text.
	if h.app.EditorGroup.Editor.Cursor.Col != 7 {
		t.Errorf("expected cursor col 7, got %d", h.app.EditorGroup.Editor.Cursor.Col)
	}
}

func TestAutocomplete_AcceptWithTab(t *testing.T) {
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	// Accept with Tab (alternative to Enter).
	h.pressKey(tcell.KeyTab, tcell.ModNone)

	if h.app.EditorGroup.Autocomplete != nil {
		t.Error("expected autocomplete to be dismissed after Tab accept")
	}

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "console" {
		t.Errorf("expected 'console' on line 0, got %q", got)
	}
}

func TestAutocomplete_ReplacesTypedPrefix(t *testing.T) {
	// Simulate: user types "cons", then autocomplete shows items matching "cons".
	// Accepting "console" should produce "console", not "consconsole".
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	// Type "cons" into the editor.
	for _, r := range "cons" {
		h.pressRune(r)
	}

	// Verify "cons" is in the buffer.
	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "cons" {
		t.Fatalf("expected 'cons' after typing, got %q", got)
	}

	// Show autocomplete (prefix "cons" is detected automatically via identStart).
	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	if h.app.EditorGroup.Autocomplete == nil {
		t.Fatal("expected autocomplete widget to be active")
	}

	// All three items match "cons" prefix: console, const, constructor.
	// First filtered item should be selected. Accept it.
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	// The typed prefix "cons" should be replaced, not duplicated.
	got = h.app.EditorGroup.Editor.Buf.Lines[0]
	// Items are sorted alphabetically: console, const, constructor.
	if got != "console" {
		t.Errorf("expected 'console' (prefix replaced), got %q", got)
	}
}

func TestAutocomplete_PrefixReplacementWithSecondItem(t *testing.T) {
	// Type "cons", navigate down to select "const", accept.
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	for _, r := range "cons" {
		h.pressRune(r)
	}

	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	// Navigate down once to select the second item.
	h.pressKey(tcell.KeyDown, tcell.ModNone)

	// Accept.
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	// Sorted items matching "cons": console, const, constructor.
	// Down once selects "const".
	if got != "const" {
		t.Errorf("expected 'const' (second item), got %q", got)
	}
}

func TestAutocomplete_DotTriggerNoDuplicate(t *testing.T) {
	// Simulate: "console." is already typed, then completions for methods appear.
	// Accepting "log" should produce "console.log", not "console..log".
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	// Type "console."
	for _, r := range "console." {
		h.pressRune(r)
	}

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "console." {
		t.Fatalf("expected 'console.' after typing, got %q", got)
	}

	// Show method completions. The cursor is right after the dot.
	// identStart will find prefix "" (dot is not an ident rune), so all items match.
	// With empty prefix, FilterCompletions returns items in original order (no sort).
	methodItems := []ui.CompletionItem{
		{Label: "error", InsertText: "error", Kind: ui.CompletionMethod},
		{Label: "log", InsertText: "log", Kind: ui.CompletionMethod},
		{Label: "warn", InsertText: "warn", Kind: ui.CompletionMethod},
	}

	h.app.ShowAutocomplete(methodItems, nil)
	h.redraw()

	if h.app.EditorGroup.Autocomplete == nil {
		t.Fatal("expected autocomplete widget to be active")
	}

	// Accept the first item ("error" - first in list, no sorting for empty prefix).
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	got = h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "console.error" {
		t.Errorf("expected 'console.error' (no double dot), got %q", got)
	}
}

func TestAutocomplete_DismissWithEscape(t *testing.T) {
	h := setupAutocompleteTest(t, "hello\n")
	defer h.stop()

	// Show autocomplete.
	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	if h.app.EditorGroup.Autocomplete == nil {
		t.Fatal("expected autocomplete widget to be active")
	}

	// Dismiss with Escape.
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if h.app.EditorGroup.Autocomplete != nil {
		t.Error("expected autocomplete to be dismissed after Escape")
	}

	// Buffer should be unchanged - no text inserted.
	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "hello" {
		t.Errorf("expected 'hello' (unchanged after dismiss), got %q", got)
	}
}

func TestAutocomplete_DismissDoesNotMoveCursor(t *testing.T) {
	// Type "con" and then show autocomplete. Dismiss should leave cursor at col 3.
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	for _, r := range "con" {
		h.pressRune(r)
	}

	e := h.app.EditorGroup.Editor
	if e.Cursor.Col != 3 {
		t.Fatalf("expected cursor col 3 after typing 'con', got %d", e.Cursor.Col)
	}

	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	if h.app.EditorGroup.Autocomplete == nil {
		t.Fatal("expected autocomplete widget to be active")
	}

	// Navigate in autocomplete, then dismiss.
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	// Cursor position should be unchanged.
	if e.Cursor.Col != 3 {
		t.Errorf("expected cursor col 3 after dismiss, got %d", e.Cursor.Col)
	}
	if e.Cursor.Line != 0 {
		t.Errorf("expected cursor line 0 after dismiss, got %d", e.Cursor.Line)
	}

	// Buffer should still have "con" (no insertion).
	got := e.Buf.Lines[0]
	if got != "con" {
		t.Errorf("expected 'con' (unchanged after dismiss), got %q", got)
	}
}

func TestAutocomplete_NavigationDown(t *testing.T) {
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	ac := h.app.EditorGroup.Autocomplete
	if ac == nil {
		t.Fatal("expected autocomplete widget to be active")
	}

	// Initial selection is 0.
	if ac.Selected != 0 {
		t.Errorf("expected initial selection 0, got %d", ac.Selected)
	}

	// Press Down to move to item 1.
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	if ac.Selected != 1 {
		t.Errorf("expected selection 1 after Down, got %d", ac.Selected)
	}

	// Press Down again to move to item 2.
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	if ac.Selected != 2 {
		t.Errorf("expected selection 2 after second Down, got %d", ac.Selected)
	}

	// Press Down at the end should stay at last item (no wrap).
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	if ac.Selected != 2 {
		t.Errorf("expected selection to stay at 2 (clamped), got %d", ac.Selected)
	}
}

func TestAutocomplete_NavigationUp(t *testing.T) {
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	ac := h.app.EditorGroup.Autocomplete

	// Move to item 2.
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	if ac.Selected != 2 {
		t.Fatalf("expected selection 2, got %d", ac.Selected)
	}

	// Press Up to go back to item 1.
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	if ac.Selected != 1 {
		t.Errorf("expected selection 1 after Up, got %d", ac.Selected)
	}

	// Press Up to go back to item 0.
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	if ac.Selected != 0 {
		t.Errorf("expected selection 0 after second Up, got %d", ac.Selected)
	}

	// Press Up at the top should stay at 0 (no wrap).
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	if ac.Selected != 0 {
		t.Errorf("expected selection to stay at 0 (clamped), got %d", ac.Selected)
	}
}

func TestAutocomplete_NavigateAndAccept(t *testing.T) {
	// Navigate to the last item and accept it.
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	// Move to the last item (sorted: console, const, constructor -> index 2).
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyDown, tcell.ModNone)

	// Accept the third item.
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "constructor" {
		t.Errorf("expected 'constructor' (third item), got %q", got)
	}

	// Cursor should be at end of "constructor" (len 11).
	if h.app.EditorGroup.Editor.Cursor.Col != 11 {
		t.Errorf("expected cursor col 11, got %d", h.app.EditorGroup.Editor.Cursor.Col)
	}
}

func TestAutocomplete_FilterByPrefix(t *testing.T) {
	// Type "const" to filter out "console" (which starts with "cons" but
	// "console" also starts with "const"? No: "console" starts with "cons"
	// but not "const". "const" starts with "const", "constructor" starts with "const").
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	for _, r := range "const" {
		h.pressRune(r)
	}

	h.app.ShowAutocomplete(sampleCompletionItems(), nil)
	h.redraw()

	ac := h.app.EditorGroup.Autocomplete
	if ac == nil {
		t.Fatal("expected autocomplete widget to be active")
	}

	// "const" prefix filters: "const" and "constructor" match.
	// "console" does NOT match "const" prefix.
	if len(ac.Items) != 2 {
		t.Errorf("expected 2 filtered items, got %d", len(ac.Items))
		for i, it := range ac.Items {
			t.Logf("  item[%d]: %q", i, it.Label)
		}
	}

	// Accept first filtered item (sorted: "const" before "constructor").
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "const" {
		t.Errorf("expected 'const', got %q", got)
	}
}

func TestAutocomplete_EmptyFilterDismisses(t *testing.T) {
	// If no items match the prefix, ShowAutocomplete should not create the widget.
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	for _, r := range "xyz" {
		h.pressRune(r)
	}

	items := []ui.CompletionItem{
		{Label: "console", InsertText: "console"},
		{Label: "const", InsertText: "const"},
	}

	h.app.ShowAutocomplete(items, nil)
	h.redraw()

	// No items match "xyz", so autocomplete should not appear.
	if h.app.EditorGroup.Autocomplete != nil {
		t.Error("expected autocomplete to not appear when no items match prefix")
	}
}

func TestAutocomplete_InsertTextUsedOverLabel(t *testing.T) {
	// When InsertText differs from Label, InsertText should be used.
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	items := []ui.CompletionItem{
		{Label: "myFunc()", InsertText: "myFunc", Kind: ui.CompletionFunction},
	}

	h.app.ShowAutocomplete(items, nil)
	h.redraw()

	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "myFunc" {
		t.Errorf("expected 'myFunc' (InsertText), got %q", got)
	}
}

func TestAutocomplete_LabelUsedWhenInsertTextEmpty(t *testing.T) {
	// When InsertText is empty, Label should be used.
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	items := []ui.CompletionItem{
		{Label: "println", Kind: ui.CompletionFunction},
	}

	h.app.ShowAutocomplete(items, nil)
	h.redraw()

	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "println" {
		t.Errorf("expected 'println' (Label as fallback), got %q", got)
	}
}

func TestAutocomplete_AcceptMidLine(t *testing.T) {
	// Test accepting a completion when there is text after the cursor.
	h := setupAutocompleteTest(t, "();")
	defer h.stop()

	// Cursor starts at col 0 (before the parens).
	items := []ui.CompletionItem{
		{Label: "fmt", InsertText: "fmt", Kind: ui.CompletionModule},
	}

	h.app.ShowAutocomplete(items, nil)
	h.redraw()

	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "fmt();" {
		t.Errorf("expected 'fmt();', got %q", got)
	}
}

func TestAutocomplete_AcceptReplacesPartialPrefixMidLine(t *testing.T) {
	// Type "pr" at the beginning of a line that has "();" after,
	// then accept "print" - should produce "print();".
	h := setupAutocompleteTest(t, "();\n")
	defer h.stop()

	// Type "pr"
	h.pressRune('p')
	h.pressRune('r')

	items := []ui.CompletionItem{
		{Label: "print", InsertText: "print", Kind: ui.CompletionFunction},
		{Label: "process", InsertText: "process", Kind: ui.CompletionVariable},
	}

	h.app.ShowAutocomplete(items, nil)
	h.redraw()

	// Accept "print" (first sorted item).
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "print();" {
		t.Errorf("expected 'print();', got %q", got)
	}
}

func TestAutocomplete_CursorPositionAfterAccept(t *testing.T) {
	// Verify cursor is placed correctly after the inserted text.
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	for _, r := range "my" {
		h.pressRune(r)
	}

	items := []ui.CompletionItem{
		{Label: "myVariable", InsertText: "myVariable", Kind: ui.CompletionVariable},
	}

	h.app.ShowAutocomplete(items, nil)
	h.redraw()

	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	e := h.app.EditorGroup.Editor
	if e.Cursor.Col != 10 {
		t.Errorf("expected cursor col 10 (len of 'myVariable'), got %d", e.Cursor.Col)
	}
	if e.Cursor.Line != 0 {
		t.Errorf("expected cursor line 0, got %d", e.Cursor.Line)
	}
}

func TestAutocomplete_WidgetRendersOnScreen(t *testing.T) {
	// Verify that autocomplete items appear on screen when rendered.
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	items := []ui.CompletionItem{
		{Label: "apple", InsertText: "apple", Kind: ui.CompletionVariable},
		{Label: "banana", InsertText: "banana", Kind: ui.CompletionVariable},
	}

	h.app.ShowAutocomplete(items, nil)
	h.redraw()

	// The autocomplete widget should render item labels on screen.
	h.assertContains("apple")
	h.assertContains("banana")
}

func TestAutocomplete_DismissRemovesFromScreen(t *testing.T) {
	h := setupAutocompleteTest(t, "\n")
	defer h.stop()

	items := []ui.CompletionItem{
		{Label: "uniqueCompletion", InsertText: "uniqueCompletion", Kind: ui.CompletionVariable},
	}

	h.app.ShowAutocomplete(items, nil)
	h.redraw()
	h.assertContains("uniqueCompletion")

	// Dismiss.
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	h.redraw()

	// The autocomplete menu label should no longer be on screen.
	// (The text was not inserted, so it should not appear.)
	h.assertNotContains("uniqueCompletion")
}

func TestAutocomplete_SnippetPlaceholder(t *testing.T) {
	h := setupAutocompleteTest(t, "<div class\n")
	defer h.stop()

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line = 0
	editor.Cursor.Col = 10

	h.app.ShowAutocomplete([]ui.CompletionItem{{
		Label:            "class",
		InsertText:       `class="$1"`,
		InsertTextFormat: lsp.InsertTextFormatSnippet,
	}}, nil)
	h.redraw()
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	if got := editor.Buf.Lines[0]; got != `<div class=""` {
		t.Fatalf("buffer = %q, want %q", got, `<div class=""`)
	}
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 12 {
		t.Fatalf("cursor = %d:%d, want 0:12", editor.Cursor.Line, editor.Cursor.Col)
	}
}
