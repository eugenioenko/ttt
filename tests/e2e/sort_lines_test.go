package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSortLinesAsc(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "sort.txt")
	requireNoError(t, os.WriteFile(f, []byte("cherry\napple\nbanana\n"), 0644))
	h.openFile(f)
	h.redraw()

	// Select all then sort ascending
	h.exec("editor.selectAll")
	h.exec("editor.sortLinesAsc")

	lines := h.app.EditorGroup.Editor.Buf.Lines
	// selectAll excludes the trailing empty line (end.Col==0 adjustment),
	// so only lines 0-2 are sorted; line 3 ("") stays.
	if lines[0] != "apple" || lines[1] != "banana" || lines[2] != "cherry" || lines[3] != "" {
		t.Errorf("expected [apple, banana, cherry, ''], got %v", lines)
	}
}

func TestSortLinesAscNoSelection(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "sortall.txt")
	requireNoError(t, os.WriteFile(f, []byte("cherry\napple\nbanana\n"), 0644))
	h.openFile(f)
	h.redraw()

	// No selection — sorts all lines (trailing empty line stays at bottom)
	h.exec("editor.sortLinesAsc")

	lines := h.app.EditorGroup.Editor.Buf.Lines
	if lines[0] != "apple" || lines[1] != "banana" || lines[2] != "cherry" || lines[3] != "" {
		t.Errorf("expected [apple, banana, cherry, ''], got %v", lines)
	}
}

func TestSortLinesDesc(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "sortdesc.txt")
	requireNoError(t, os.WriteFile(f, []byte("apple\ncherry\nbanana\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.selectAll")
	h.exec("editor.sortLinesDesc")

	lines := h.app.EditorGroup.Editor.Buf.Lines
	// selectAll excludes trailing empty line: sorts lines 0-2 descending
	if lines[0] != "cherry" || lines[1] != "banana" || lines[2] != "apple" || lines[3] != "" {
		t.Errorf("expected [cherry, banana, apple, ''], got %v", lines)
	}
}

func TestReverseLines(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "reverse.txt")
	requireNoError(t, os.WriteFile(f, []byte("first\nsecond\nthird\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.selectAll")
	h.exec("editor.reverseLines")

	lines := h.app.EditorGroup.Editor.Buf.Lines
	// selectAll excludes trailing empty line: reverses lines 0-2
	if lines[0] != "third" || lines[1] != "second" || lines[2] != "first" || lines[3] != "" {
		t.Errorf("expected [third, second, first, ''], got %v", lines)
	}
}

func TestReverseLinesNoSelection(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "revall.txt")
	requireNoError(t, os.WriteFile(f, []byte("first\nsecond\nthird\n"), 0644))
	h.openFile(f)
	h.redraw()

	// No selection — reverses all lines (trailing empty line stays at bottom)
	h.exec("editor.reverseLines")

	lines := h.app.EditorGroup.Editor.Buf.Lines
	if lines[0] != "third" || lines[1] != "second" || lines[2] != "first" || lines[3] != "" {
		t.Errorf("expected [third, second, first, ''], got %v", lines)
	}
}

func TestUniqueLines(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "unique.txt")
	requireNoError(t, os.WriteFile(f, []byte("apple\nbanana\napple\ncherry\nbanana\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.selectAll")
	h.exec("editor.uniqueLines")

	lines := h.app.EditorGroup.Editor.Buf.Lines
	// selectAll excludes trailing empty line: deduplicates lines 0-4
	// ["apple","banana","apple","cherry","banana"] -> ["apple","banana","cherry"]
	// trailing empty line stays at end
	if len(lines) != 4 || lines[0] != "apple" || lines[1] != "banana" || lines[2] != "cherry" || lines[3] != "" {
		t.Errorf("expected [apple, banana, cherry, ''], got %v", lines)
	}
}

func TestSortLinesUndo(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "sortundo.txt")
	requireNoError(t, os.WriteFile(f, []byte("cherry\napple\nbanana\n"), 0644))
	h.openFile(f)
	h.redraw()

	// No selection — sort all lines (trailing empty line stays at bottom)
	h.exec("editor.sortLinesAsc")

	lines := h.app.EditorGroup.Editor.Buf.Lines
	if lines[0] != "apple" || lines[1] != "banana" || lines[2] != "cherry" || lines[3] != "" {
		t.Errorf("expected [apple, banana, cherry, ''] after sort, got %v", lines)
	}

	// Undo should restore original order
	h.exec("editor.undo")

	lines = h.app.EditorGroup.Editor.Buf.Lines
	if lines[0] != "cherry" || lines[1] != "apple" || lines[2] != "banana" || lines[3] != "" {
		t.Errorf("expected [cherry, apple, banana, ''] after undo, got %v", lines)
	}
}
