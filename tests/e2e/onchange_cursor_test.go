package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestOnChangeRuneInsert(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange.txt")
	requireNoError(t, os.WriteFile(f, []byte("ab"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.app.EditorGroup.Editor.Cursor.Col = 2

	cursorColDuringChange := -1
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		cursorColDuringChange = h.app.EditorGroup.Editor.Cursor.Col
		if orig != nil {
			orig()
		}
	}

	h.pressRune('c')

	if cursorColDuringChange == -1 {
		t.Error("OnChange did not fire on rune insert")
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "abc" {
		t.Errorf("expected buffer 'abc', got %q", got)
	}
	if h.app.EditorGroup.Editor.Cursor.Col != 3 {
		t.Errorf("expected cursor col 3, got %d", h.app.EditorGroup.Editor.Cursor.Col)
	}
	if cursorColDuringChange != 3 {
		t.Errorf("cursor during OnChange should be 3, got %d", cursorColDuringChange)
	}
}

func TestOnChangeBackspace(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange_bs.txt")
	requireNoError(t, os.WriteFile(f, []byte("abc"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.app.EditorGroup.Editor.Cursor.Col = 3

	cursorColDuringChange := -1
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		cursorColDuringChange = h.app.EditorGroup.Editor.Cursor.Col
		if orig != nil {
			orig()
		}
	}

	h.pressKey(tcell.KeyBackspace2, tcell.ModNone)

	if cursorColDuringChange == -1 {
		t.Error("OnChange did not fire on backspace")
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "ab" {
		t.Errorf("expected buffer 'ab', got %q", got)
	}
	if h.app.EditorGroup.Editor.Cursor.Col != 2 {
		t.Errorf("expected cursor col 2, got %d", h.app.EditorGroup.Editor.Cursor.Col)
	}
	if cursorColDuringChange != 2 {
		t.Errorf("cursor during OnChange should be 2, got %d", cursorColDuringChange)
	}
}

func TestOnChangeDelete(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange_del.txt")
	requireNoError(t, os.WriteFile(f, []byte("abc"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.app.EditorGroup.Editor.Cursor.Col = 1

	cursorColDuringChange := -1
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		cursorColDuringChange = h.app.EditorGroup.Editor.Cursor.Col
		if orig != nil {
			orig()
		}
	}

	h.pressKey(tcell.KeyDelete, tcell.ModNone)

	if cursorColDuringChange == -1 {
		t.Error("OnChange did not fire on delete")
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "ac" {
		t.Errorf("expected buffer 'ac', got %q", got)
	}
	if h.app.EditorGroup.Editor.Cursor.Col != 1 {
		t.Errorf("expected cursor col 1, got %d", h.app.EditorGroup.Editor.Cursor.Col)
	}
	if cursorColDuringChange != 1 {
		t.Errorf("cursor during OnChange should be 1, got %d", cursorColDuringChange)
	}
}

func TestOnChangeUndo(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange_undo.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Col = 5
	h.pressRune('!')

	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "hello!" {
		t.Fatalf("expected 'hello!', got %q", got)
	}

	changeFired := false
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		changeFired = true
		if orig != nil {
			orig()
		}
	}

	h.exec("editor.undo")

	if !changeFired {
		t.Error("OnChange did not fire on undo")
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "hello" {
		t.Errorf("expected 'hello' after undo, got %q", got)
	}
}

func TestOnChangeToggleComment(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange_comment.js")
	requireNoError(t, os.WriteFile(f, []byte("hello"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.app.EditorGroup.Editor.Cursor.Col = 3

	cursorColDuringChange := -1
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		cursorColDuringChange = h.app.EditorGroup.Editor.Cursor.Col
		if orig != nil {
			orig()
		}
	}

	h.exec("editor.toggleComment")

	if cursorColDuringChange == -1 {
		t.Error("OnChange did not fire on toggle comment")
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "// hello" {
		t.Errorf("expected '// hello', got %q", got)
	}
	if cursorColDuringChange != 6 {
		t.Errorf("cursor during OnChange should be 6 (3 + '// ' prefix), got %d", cursorColDuringChange)
	}
}

func TestOnChangeMultipleRunes(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange_multi.txt")
	requireNoError(t, os.WriteFile(f, []byte("ab"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Col = 2

	var cursorCols []int
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		cursorCols = append(cursorCols, h.app.EditorGroup.Editor.Cursor.Col)
		if orig != nil {
			orig()
		}
	}

	h.pressRune('c')
	h.pressRune('d')
	h.pressRune('e')

	if len(cursorCols) != 3 {
		t.Errorf("expected OnChange to fire 3 times, got %d", len(cursorCols))
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "abcde" {
		t.Errorf("expected 'abcde', got %q", got)
	}
	if h.app.EditorGroup.Editor.Cursor.Col != 5 {
		t.Errorf("expected cursor col 5, got %d", h.app.EditorGroup.Editor.Cursor.Col)
	}
	expected := []int{3, 4, 5}
	for i, want := range expected {
		if i < len(cursorCols) && cursorCols[i] != want {
			t.Errorf("cursor during OnChange[%d] should be %d, got %d", i, want, cursorCols[i])
		}
	}
}

func TestOnChangeTab(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange_tab.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Col = 0

	cursorColDuringChange := -1
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		cursorColDuringChange = h.app.EditorGroup.Editor.Cursor.Col
		if orig != nil {
			orig()
		}
	}

	h.pressKey(tcell.KeyTab, tcell.ModNone)

	if cursorColDuringChange == -1 {
		t.Error("OnChange did not fire on tab")
	}
	tabSize := h.app.EditorGroup.Editor.TabSize
	if tabSize == 0 {
		tabSize = h.app.Settings.Editor.TabSize
	}
	if h.app.EditorGroup.Editor.Cursor.Col != tabSize {
		t.Errorf("expected cursor col %d, got %d", tabSize, h.app.EditorGroup.Editor.Cursor.Col)
	}
	if cursorColDuringChange != tabSize {
		t.Errorf("cursor during OnChange should be %d, got %d", tabSize, cursorColDuringChange)
	}
}

func TestOnChangeBackspaceJoinLine(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange_bsjoin.txt")
	requireNoError(t, os.WriteFile(f, []byte("ab\ncd"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 1
	h.app.EditorGroup.Editor.Cursor.Col = 0

	cursorLineDuringChange := -1
	cursorColDuringChange := -1
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		cursorLineDuringChange = h.app.EditorGroup.Editor.Cursor.Line
		cursorColDuringChange = h.app.EditorGroup.Editor.Cursor.Col
		if orig != nil {
			orig()
		}
	}

	h.pressKey(tcell.KeyBackspace2, tcell.ModNone)

	if cursorColDuringChange == -1 {
		t.Error("OnChange did not fire on backspace join")
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "abcd" {
		t.Errorf("expected 'abcd', got %q", got)
	}
	if h.app.EditorGroup.Editor.Cursor.Line != 0 {
		t.Errorf("expected cursor line 0, got %d", h.app.EditorGroup.Editor.Cursor.Line)
	}
	if h.app.EditorGroup.Editor.Cursor.Col != 2 {
		t.Errorf("expected cursor col 2, got %d", h.app.EditorGroup.Editor.Cursor.Col)
	}
	if cursorLineDuringChange != 0 {
		t.Errorf("cursor line during OnChange should be 0, got %d", cursorLineDuringChange)
	}
	if cursorColDuringChange != 2 {
		t.Errorf("cursor col during OnChange should be 2, got %d", cursorColDuringChange)
	}
}

func TestOnChangeSelectionReplace(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange_selreplace.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello"), 0644))
	h.openFile(f)
	h.redraw()

	// Select "ell" (col 1 to col 4)
	h.app.EditorGroup.Editor.Cursor.Col = 1
	h.app.EditorGroup.Editor.Selection.Start(0, 1)
	h.app.EditorGroup.Editor.Cursor.Col = 4
	h.app.EditorGroup.Editor.Selection.Active = true

	cursorColDuringChange := -1
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		cursorColDuringChange = h.app.EditorGroup.Editor.Cursor.Col
		if orig != nil {
			orig()
		}
	}

	h.pressRune('X')

	if cursorColDuringChange == -1 {
		t.Error("OnChange did not fire on selection replace")
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "hXo" {
		t.Errorf("expected 'hXo', got %q", got)
	}
	if h.app.EditorGroup.Editor.Cursor.Col != 2 {
		t.Errorf("expected cursor col 2, got %d", h.app.EditorGroup.Editor.Cursor.Col)
	}
	if cursorColDuringChange != 2 {
		t.Errorf("cursor during OnChange should be 2, got %d", cursorColDuringChange)
	}
}

func TestOnChangeEnter(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "onchange_enter.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello world"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Col = 5

	cursorLineDuringChange := -1
	cursorColDuringChange := -1
	orig := h.app.EditorGroup.Editor.OnChange
	h.app.EditorGroup.Editor.OnChange = func() {
		cursorLineDuringChange = h.app.EditorGroup.Editor.Cursor.Line
		cursorColDuringChange = h.app.EditorGroup.Editor.Cursor.Col
		if orig != nil {
			orig()
		}
	}

	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	if cursorColDuringChange == -1 {
		t.Error("OnChange did not fire on enter")
	}
	if len(h.app.EditorGroup.Editor.Buf.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(h.app.EditorGroup.Editor.Buf.Lines))
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "hello" {
		t.Errorf("expected first line 'hello', got %q", got)
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[1]; got != " world" {
		t.Errorf("expected second line ' world', got %q", got)
	}
	if cursorLineDuringChange != 1 {
		t.Errorf("cursor line during OnChange should be 1, got %d", cursorLineDuringChange)
	}
	if cursorColDuringChange != 0 {
		t.Errorf("cursor col during OnChange should be 0, got %d", cursorColDuringChange)
	}
}
