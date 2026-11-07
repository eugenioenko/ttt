package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

const scrollTestLines = 200

func openScrollFile(t *testing.T, h *testHarness) {
	t.Helper()
	lines := make([]string, scrollTestLines)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	path := filepath.Join(h.dir, "scroll.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.exec("editor.focus")
	h.redraw()
}

func placeScroll(t *testing.T, h *testHarness, top, line int) {
	t.Helper()
	editor := h.app.EditorGroup.Editor
	editor.Viewport.TopLine = top
	editor.Cursor.Line, editor.Cursor.Col = line, 0
	h.redraw()
}

func TestScrollDownFollowsCursor(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)
	placeScroll(t, h, 0, 0)

	editor := h.app.EditorGroup.Editor
	half := max(1, editor.Viewport.Height/2)
	cursorY := editor.CursorY
	h.exec("view.scrollDown")
	if editor.Viewport.TopLine != half || editor.Cursor.Line != half {
		t.Fatalf("Scroll Down moved to top %d line %d, want %d/%d", editor.Viewport.TopLine, editor.Cursor.Line, half, half)
	}
	if editor.CursorY != cursorY {
		t.Fatalf("Scroll Down moved the cursor from screen row %d to %d", cursorY, editor.CursorY)
	}
}

func TestScrollUpFollowsCursor(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)
	placeScroll(t, h, 100, 105)

	editor := h.app.EditorGroup.Editor
	half := max(1, editor.Viewport.Height/2)
	cursorY := editor.CursorY
	h.exec("view.scrollUp")
	if want := 100 - half; editor.Viewport.TopLine != want {
		t.Fatalf("Scroll Up moved the viewport to %d, want %d", editor.Viewport.TopLine, want)
	}
	if want := 105 - half; editor.Cursor.Line != want {
		t.Fatalf("Scroll Up moved the cursor to line %d, want %d", editor.Cursor.Line, want)
	}
	if editor.CursorY != cursorY {
		t.Fatalf("Scroll Up moved the cursor from screen row %d to %d", cursorY, editor.CursorY)
	}
}

func TestScrollLinesUsesConfiguredStep(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)
	h.app.Settings.Editor.ScrollLines = 7
	placeScroll(t, h, 100, 105)

	editor := h.app.EditorGroup.Editor
	h.exec("view.scrollDown")
	if editor.Viewport.TopLine != 107 || editor.Cursor.Line != 112 {
		t.Fatalf("Scroll Down moved to top %d line %d, want 107/112", editor.Viewport.TopLine, editor.Cursor.Line)
	}
	h.exec("view.scrollUp")
	if editor.Viewport.TopLine != 100 || editor.Cursor.Line != 105 {
		t.Fatalf("Scroll Up moved to top %d line %d, want 100/105", editor.Viewport.TopLine, editor.Cursor.Line)
	}
}

func TestScrollPreservesColumn(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)
	placeScroll(t, h, 0, 0)

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Col = 3
	h.redraw()
	half := max(1, editor.Viewport.Height/2)
	h.exec("view.scrollDown")
	if editor.Cursor.Line != half || editor.Cursor.Col != 3 {
		t.Fatalf("Scroll Down moved the cursor to (%d,%d), want (%d,3)", editor.Cursor.Line, editor.Cursor.Col, half)
	}
	h.exec("view.scrollUp")
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 3 {
		t.Fatalf("Scroll Up moved the cursor to (%d,%d), want (0,3)", editor.Cursor.Line, editor.Cursor.Col)
	}
}

func TestScrollUsesVisibleRowsWithFold(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	var b strings.Builder
	b.WriteString("package main\n\nfunc main() {\n")
	for range 20 {
		b.WriteString("\ta()\n")
	}
	b.WriteString("}\n\n")
	for i := range 40 {
		fmt.Fprintf(&b, "func f%d() {}\n", i)
	}
	path := filepath.Join(h.dir, "scroll_fold.go")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.exec("fold.collapseAll")
	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 0
	h.redraw()

	// Visible rows: 0 package, 1 blank, 2 func main, 3 closing brace,
	// 4 blank, 5 f0, ... Half a viewport is 9 rows, which lands on line 29.
	h.exec("view.scrollDown")
	if editor.Cursor.Line != 29 || editor.Viewport.TopLine != 29 {
		t.Fatalf("scroll moved to top %d line %d, want 29/29", editor.Viewport.TopLine, editor.Cursor.Line)
	}
}

func TestScrollStopsAtFileEdges(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)

	editor := h.app.EditorGroup.Editor
	placeScroll(t, h, 0, 0)
	h.exec("view.scrollUp")
	if editor.Viewport.TopLine != 0 || editor.Cursor.Line != 0 {
		t.Fatalf("Scroll Up moved to top %d line %d, want 0/0", editor.Viewport.TopLine, editor.Cursor.Line)
	}

	h.app.Settings.Editor.ScrollLines = scrollTestLines * 2
	h.exec("view.scrollDown")
	lastLine := len(editor.Buf.Lines) - 1
	if editor.Cursor.Line != lastLine {
		t.Fatalf("Scroll Down stopped the cursor at line %d, want %d", editor.Cursor.Line, lastLine)
	}
	if editor.Viewport.TopLine+editor.Viewport.Height < len(editor.Buf.Lines) {
		t.Fatalf("Scroll Down left the end of the file off screen: top %d height %d lines %d", editor.Viewport.TopLine, editor.Viewport.Height, len(editor.Buf.Lines))
	}
	first := editor.Viewport.TopLine
	h.exec("view.scrollDown")
	if editor.Viewport.TopLine != first {
		t.Fatalf("Scroll Down moved past the end to %d, want %d", editor.Viewport.TopLine, first)
	}
}

func TestScrollDefaultKeybindings(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)
	placeScroll(t, h, 0, 0)

	editor := h.app.EditorGroup.Editor
	half := max(1, editor.Viewport.Height/2)
	h.pressKey(tcell.KeyDown, tcell.ModCtrl)
	if editor.Viewport.TopLine != half || editor.Cursor.Line != half {
		t.Fatalf("Ctrl+Down moved to top %d line %d, want %d/%d", editor.Viewport.TopLine, editor.Cursor.Line, half, half)
	}
	h.pressKey(tcell.KeyUp, tcell.ModCtrl)
	if editor.Viewport.TopLine != 0 || editor.Cursor.Line != 0 {
		t.Fatalf("Ctrl+Up moved to top %d line %d, want 0/0", editor.Viewport.TopLine, editor.Cursor.Line)
	}
}
