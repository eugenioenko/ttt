package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/lsp"

	"github.com/gdamore/tcell/v3"
)

const outlineMd = `# Title

## Section One
body
### Sub

## Section Two
`

func writeOutlineFile(t *testing.T, h *testHarness) string {
	t.Helper()
	path := filepath.Join(h.dir, "doc.md")
	if err := os.WriteFile(path, []byte(outlineMd), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOutlineMarkdownFallback(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	path := writeOutlineFile(t, h)
	h.openFile(path)
	h.exec("sidebar.outline")

	if h.app.Sidebar.ActivePanel != "outline" {
		t.Fatalf("expected active panel 'outline', got %q", h.app.Sidebar.ActivePanel)
	}
	h.assertContains("Title")
	h.assertContains("Section One")
	h.assertContains("Sub")
	h.assertContains("Section Two")

	flat := h.app.Symbols.Tree.FlatList()
	if len(flat) != 4 {
		t.Fatalf("expected 4 outline nodes, got %d", len(flat))
	}
}

func TestOutlineKeyboardNavigationAndJump(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	path := writeOutlineFile(t, h)
	h.openFile(path)
	h.exec("sidebar.outline")

	if h.app.Root.Focused != h.app.Symbols.Adapter {
		t.Fatal("expected outline panel focused after sidebar.outline")
	}

	// Arrow navigation reveals the symbol without stealing focus.
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	if got := h.app.EditorGroup.Editor.Cursor.Line; got != 2 {
		t.Errorf("expected cursor at line 2 (Section One) after down, got %d", got)
	}
	if h.app.Root.Focused != h.app.Symbols.Adapter {
		t.Error("outline should keep focus while navigating")
	}

	// Move to the last leaf and activate: jump + focus editor.
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if got := h.app.EditorGroup.Editor.Cursor.Line; got != 6 {
		t.Errorf("expected cursor at line 6 (Section Two) after enter, got %d", got)
	}
	if h.app.Root.Focused != h.app.EditorGroup {
		t.Error("expected editor focused after activating a symbol")
	}
}

func TestOutlineClickJump(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	path := writeOutlineFile(t, h)
	h.openFile(path)
	h.exec("sidebar.outline")

	// Tree content starts below the tab row and divider.
	r := h.app.Sidebar.GetRect()
	h.click(r.X+4, r.Y+2+3) // 4th row: Section Two

	if got := h.app.EditorGroup.Editor.Cursor.Line; got != 6 {
		t.Errorf("expected cursor at line 6 after click, got %d", got)
	}
	// Clicking inside the sidebar keeps panel focus (SplitPanel.OnLeftClick),
	// same as the Find panel; only keyboard activation focuses the editor.
	if h.app.Root.Focused != h.app.Symbols.Adapter {
		t.Error("expected outline to keep focus after mouse click")
	}
}

func TestOutlineEmptyState(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	h.openFile(filepath.Join(h.dir, "alpha.txt"))
	h.exec("sidebar.outline")
	h.assertContains("No symbols")
}

func TestOutlineFollowsCursor(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	path := writeOutlineFile(t, h)
	h.openFile(path)
	h.exec("sidebar.outline")

	h.app.EditorGroup.Editor.Cursor.Line = 4
	h.app.ApplySymbols([]lsp.DocumentSymbol{
		{Name: "first", SelectionRange: lsp.Range{Start: lsp.Position{Line: 0}}},
		{Name: "second", SelectionRange: lsp.Range{Start: lsp.Position{Line: 3}}},
		{Name: "third", SelectionRange: lsp.Range{Start: lsp.Position{Line: 6}}},
	})

	sel := h.app.Symbols.Tree.Selected()
	if sel == nil || sel.Label != "second" {
		t.Errorf("expected 'second' selected for cursor line 4, got %+v", sel)
	}
}
