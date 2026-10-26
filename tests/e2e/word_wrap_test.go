package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestWordWrapToggle(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	if h.app.Settings.Editor.WordWrap {
		t.Fatal("word wrap should be disabled by default")
	}

	h.exec("options.toggleWordWrap")

	if !h.app.Settings.Editor.WordWrap {
		t.Error("word wrap should be enabled after toggle")
	}
	if !h.app.EditorGroup.WordWrap {
		t.Error("editor group word wrap should be enabled after toggle")
	}
	if !h.app.EditorGroup.Editor.WordWrap {
		t.Error("editor pane word wrap should be enabled after toggle")
	}

	h.exec("options.toggleWordWrap")

	if h.app.Settings.Editor.WordWrap {
		t.Error("word wrap should be disabled after second toggle")
	}
	if h.app.EditorGroup.WordWrap {
		t.Error("editor group word wrap should be disabled after second toggle")
	}
	if h.app.EditorGroup.Editor.WordWrap {
		t.Error("editor pane word wrap should be disabled after second toggle")
	}
}

func TestWordWrapRendersLongLine(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	longLine := strings.Repeat("abcde", 20)
	path := filepath.Join(h.dir, "wrap.txt")
	os.WriteFile(path, []byte(longLine), 0644)

	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	screenBefore := h.screenText()

	h.exec("options.toggleWordWrap")
	h.redraw()

	screenAfter := h.screenText()

	countBefore := strings.Count(screenBefore, "abcdeabcde")
	countAfter := strings.Count(screenAfter, "abcdeabcde")

	if countAfter <= countBefore {
		t.Errorf("wrapped screen should show more content: unwrapped has %d occurrences, wrapped has %d",
			countBefore, countAfter)
	}
}

func TestWordWrapLineNumbersOnlyOnFirstRow(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	longLine := strings.Repeat("x", 200)
	path := filepath.Join(h.dir, "wrapnum.txt")
	os.WriteFile(path, []byte(longLine+"\nsecondline"), 0644)

	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	h.exec("options.toggleWordWrap")
	h.redraw()

	editor := h.app.EditorGroup.Editor
	r := editor.GetRect()
	gutterW := editor.GutterWidth()

	lineNumCount := 0
	for y := 0; y < r.H; y++ {
		screenY := r.Y + y
		row := h.screenRow(screenY)
		if r.X+gutterW > len([]rune(row)) {
			continue
		}
		runes := []rune(row)
		gutter := string(runes[r.X : r.X+gutterW])
		trimmed := strings.TrimSpace(gutter)
		if trimmed != "" && len(trimmed) > 0 && trimmed[0] >= '1' && trimmed[0] <= '9' {
			lineNumCount++
		}
	}

	bufLines := len(editor.Buf.Lines)
	if lineNumCount != bufLines {
		t.Errorf("expected %d line numbers (one per buffer line), got %d", bufLines, lineNumCount)
	}

	totalEditorRows := r.H
	if lineNumCount >= totalEditorRows {
		t.Error("line numbers should appear on fewer rows than total editor rows (proving some rows are continuation)")
	}
}

func TestWordWrapCursorPosition(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	longLine := strings.Repeat("abcde", 20)
	path := filepath.Join(h.dir, "wrapcur.txt")
	os.WriteFile(path, []byte(longLine), 0644)

	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	h.exec("options.toggleWordWrap")
	h.redraw()

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Col = 0
	h.redraw()

	startY := editor.CursorY

	editorW := editor.Viewport.Width
	moveCount := editorW + 5
	for i := 0; i < moveCount; i++ {
		h.pressKey(tcell.KeyRight, 0)
	}
	h.redraw()

	if editor.Cursor.Col != moveCount {
		t.Errorf("expected cursor col %d, got %d", moveCount, editor.Cursor.Col)
	}

	if editor.CursorY <= startY {
		t.Errorf("cursor should have moved to a wrapped row, startY=%d, curY=%d", startY, editor.CursorY)
	}
}

func TestWordWrapDisablesHorizontalScroll(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	longLine := strings.Repeat("x", 200)
	path := filepath.Join(h.dir, "nohscroll.txt")
	os.WriteFile(path, []byte(longLine), 0644)

	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	h.exec("options.toggleWordWrap")
	h.redraw()

	editor := h.app.EditorGroup.Editor
	if editor.Viewport.LeftCol != 0 {
		t.Errorf("expected LeftCol=0 with word wrap, got %d", editor.Viewport.LeftCol)
	}
}

func openWrapped(t *testing.T, h *testHarness, name, content string) {
	t.Helper()
	path := filepath.Join(h.dir, name)
	os.WriteFile(path, []byte(content), 0644)
	h.app.EditorGroup.OpenFile(path)
	if !h.app.Settings.Editor.WordWrap {
		h.exec("options.toggleWordWrap")
	}
	h.exec("editor.focus")
	h.redraw()
}

func TestWordWrapArrowKeysMoveByScreenRow(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openWrapped(t, h, "rows.txt", strings.Repeat("abcde", 30)+"\nnext line")

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 3
	h.redraw()
	startX, startY := editor.CursorX, editor.CursorY
	width := editor.Viewport.Width

	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.redraw()
	if editor.Cursor.Line != 0 || editor.Cursor.Col != width+3 {
		t.Fatalf("Down moved to (%d,%d), want the next wrapped row at (0,%d)", editor.Cursor.Line, editor.Cursor.Col, width+3)
	}
	if editor.CursorY != startY+1 || editor.CursorX != startX {
		t.Fatalf("cursor drawn at (%d,%d), want (%d,%d)", editor.CursorX, editor.CursorY, startX, startY+1)
	}

	h.pressKey(tcell.KeyUp, tcell.ModNone)
	h.redraw()
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 3 || editor.CursorY != startY {
		t.Fatalf("Up moved to (%d,%d) on screen row %d, want (0,3) on row %d", editor.Cursor.Line, editor.Cursor.Col, editor.CursorY, startY)
	}

	rows := (150 + width - 1) / width
	for range rows {
		h.pressKey(tcell.KeyDown, tcell.ModNone)
	}
	h.redraw()
	if editor.Cursor.Line != 1 || editor.Cursor.Col != 3 || editor.CursorY != startY+rows {
		t.Fatalf("after %d Downs cursor at (%d,%d) on screen row %d, want (1,3) on row %d", rows, editor.Cursor.Line, editor.Cursor.Col, editor.CursorY, startY+rows)
	}
}

func TestWordWrapShiftDownSelectsScreenRow(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openWrapped(t, h, "select.txt", strings.Repeat("abcde", 30))

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 0
	h.pressKey(tcell.KeyDown, tcell.ModShift)

	width := editor.Viewport.Width
	got := editor.Selection.Text(editor.Buf.Lines, editor.Cursor.Line, editor.Cursor.Col)
	if want := strings.Repeat("abcde", 30)[:width]; got != want {
		t.Fatalf("Shift+Down selected %q, want the first screen row %q", got, want)
	}
}

func TestWordWrapPageDownMovesOneScreen(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	var lines []string
	for range 20 {
		lines = append(lines, strings.Repeat("abcde", 30))
	}
	openWrapped(t, h, "pages.txt", strings.Join(lines, "\n"))

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 0
	h.redraw()
	width, height := editor.Viewport.Width, editor.Viewport.Height
	rowsPerLine := (150 + width - 1) / width

	h.pressKey(tcell.KeyPgDn, tcell.ModNone)
	h.redraw()
	wantLine, wantSeg := height/rowsPerLine, height%rowsPerLine
	if editor.Cursor.Line != wantLine || editor.Cursor.Col != wantSeg*width {
		t.Fatalf("PageDown moved to (%d,%d), want screen row %d at (%d,%d)", editor.Cursor.Line, editor.Cursor.Col, height, wantLine, wantSeg*width)
	}

	h.pressKey(tcell.KeyPgUp, tcell.ModNone)
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 0 {
		t.Fatalf("PageUp moved to (%d,%d), want (0,0)", editor.Cursor.Line, editor.Cursor.Col)
	}
}
