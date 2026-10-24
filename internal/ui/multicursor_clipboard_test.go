package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/selection"
)

// openClipboardEditor opens a single-tab editor group over lines and makes it
// the active editor, ready for clipboard commands.
func openClipboardEditor(t *testing.T, lines ...string) *EditorGroupWidget {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.txt")
	if err := os.WriteFile(path, []byte(joinLines(lines)), 0644); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(path)
	return g
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

// setCursors replaces the editor's cursors with the given positions, marking
// the first as primary.
func setCursors(e *EditorPaneWidget, positions [][2]int) {
	e.Multi = nil
	e.Cursor.Line, e.Cursor.Col = positions[0][0], positions[0][1]
	e.ensureMulti()
	for _, p := range positions[1:] {
		e.Multi.Add(p[0], p[1])
	}
	e.syncFromMulti()
}

// selectAtCursor gives the cursor at index idx an active selection anchored at
// (line, col), with the cursor position itself as the moving end.
func selectAtCursor(e *EditorPaneWidget, idx, line, col int) {
	e.syncToMulti()
	cs := &e.Multi.Cursors[idx]
	cs.Sel = selection.Selection{Active: true, Anchor: selection.Position{Line: line, Col: col}}
	e.syncFromMulti()
}

// undoSteps undoes until the stack is empty and reports how many steps it took.
func undoSteps(e *EditorPaneWidget) int {
	steps := 0
	for e.Undo.Undo(e.Buf) != nil {
		steps++
	}
	return steps
}

func TestMultiCursorCopyJoinsSelectionsInDocumentOrder(t *testing.T) {
	clipboard.DisableSystem()
	g := openClipboardEditor(t, "alpha foo", "beta bar", "gamma baz")
	e := g.Editor
	setCursors(e, [][2]int{{0, 9}, {1, 9}}) // primary first on purpose
	selectAtCursor(e, 0, 0, 6)              // "foo"
	selectAtCursor(e, 1, 1, 5)              // "bar"

	g.Copy()

	want := "foo\nbar"
	if got := clipboard.Get(); got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}
}

func TestMultiCursorCopyEmptySelectionTakesWholeLines(t *testing.T) {
	clipboard.DisableSystem()
	g := openClipboardEditor(t, "one", "two", "three")
	e := g.Editor
	setCursors(e, [][2]int{{0, 1}, {2, 2}})

	g.Copy()

	want := "one\nthree"
	if got := clipboard.Get(); got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}
}

func TestMultiCursorCutDeletesAtEveryCursorAsOneUndoStep(t *testing.T) {
	clipboard.DisableSystem()
	g := openClipboardEditor(t, "keep foo", "keep bar", "tail")
	e := g.Editor
	setCursors(e, [][2]int{{0, 8}, {1, 8}})
	selectAtCursor(e, 0, 0, 5) // "foo"
	selectAtCursor(e, 1, 1, 5) // "bar"

	g.Cut()

	if got := clipboard.Get(); got != "foo\nbar" {
		t.Fatalf("clipboard = %q, want %q", got, "foo\nbar")
	}
	if e.Buf.Lines[0] != "keep " || e.Buf.Lines[1] != "keep " || e.Buf.Lines[2] != "tail" {
		t.Fatalf("buffer after cut = %q", e.Buf.Lines)
	}
	// The whole cut is exactly one undo step, and it restores both deletions.
	if steps := undoSteps(e); steps != 1 {
		t.Fatalf("expected exactly 1 undo step, got %d", steps)
	}
	if e.Buf.Lines[0] != "keep foo" || e.Buf.Lines[1] != "keep bar" {
		t.Fatalf("buffer after undo = %q, want both selections restored", e.Buf.Lines)
	}
}

func TestMultiCursorCutEmptySelectionDeletesWholeLines(t *testing.T) {
	clipboard.DisableSystem()
	g := openClipboardEditor(t, "drop me", "keep me", "drop me too")
	e := g.Editor
	setCursors(e, [][2]int{{0, 0}, {2, 0}})

	g.Cut()

	if got := clipboard.Get(); got != "drop me\ndrop me too" {
		t.Fatalf("clipboard = %q, want %q", got, "drop me\ndrop me too")
	}
	if len(e.Buf.Lines) != 1 || e.Buf.Lines[0] != "keep me" {
		t.Fatalf("buffer after cut = %q, want only the kept line", e.Buf.Lines)
	}
}

func TestMultiCursorPasteSpreadsOneLinePerCursor(t *testing.T) {
	clipboard.DisableSystem()
	g := openClipboardEditor(t, "aaa", "bbb")
	e := g.Editor
	setCursors(e, [][2]int{{0, 0}, {1, 0}})
	clipboard.Set("X\nY")

	g.Paste()

	if e.Buf.Lines[0] != "Xaaa" || e.Buf.Lines[1] != "Ybbb" {
		t.Fatalf("buffer after spread paste = %q", e.Buf.Lines)
	}
	if e.Cursor.Line != 0 || e.Cursor.Col != 1 {
		t.Fatalf("primary cursor = (%d,%d), want (0,1)", e.Cursor.Line, e.Cursor.Col)
	}
	if steps := undoSteps(e); steps != 1 {
		t.Fatalf("expected exactly 1 undo step, got %d", steps)
	}
	if e.Buf.Lines[0] != "aaa" || e.Buf.Lines[1] != "bbb" {
		t.Fatalf("buffer after undo = %q", e.Buf.Lines)
	}
}

func TestMultiCursorPasteFullTextWhenLineCountMismatch(t *testing.T) {
	clipboard.DisableSystem()
	g := openClipboardEditor(t, "aaa", "bbb")
	e := g.Editor
	setCursors(e, [][2]int{{0, 0}, {1, 0}})
	clipboard.Set("Z")

	g.Paste()

	if e.Buf.Lines[0] != "Zaaa" || e.Buf.Lines[1] != "Zbbb" {
		t.Fatalf("buffer after full paste = %q", e.Buf.Lines)
	}
}

func TestMultiCursorPasteReplacesSelections(t *testing.T) {
	clipboard.DisableSystem()
	g := openClipboardEditor(t, "foo bar", "baz qux")
	e := g.Editor
	setCursors(e, [][2]int{{0, 3}, {1, 3}})
	selectAtCursor(e, 0, 0, 0) // "foo"
	selectAtCursor(e, 1, 1, 0) // "baz"
	clipboard.Set("1\n2")

	g.Paste()

	if e.Buf.Lines[0] != "1 bar" || e.Buf.Lines[1] != "2 qux" {
		t.Fatalf("buffer after paste over selections = %q", e.Buf.Lines)
	}
}

// A single cursor must keep the old behavior: copy the whole line with a
// trailing newline, and paste without spreading.
func TestSingleCursorCopyPasteUnchanged(t *testing.T) {
	clipboard.DisableSystem()
	g := openClipboardEditor(t, "solo line")
	e := g.Editor
	e.Multi = nil

	g.Copy()
	if got := clipboard.Get(); got != "solo line\n" {
		t.Fatalf("single-cursor copy = %q, want %q", got, "solo line\n")
	}

	clipboard.Set("ins ")
	g.Paste()
	if e.Buf.Lines[0] != "ins solo line" {
		t.Fatalf("single-cursor paste = %q", e.Buf.Lines[0])
	}
}
