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

func TestScrollRestoresGoalColumn(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	path := filepath.Join(h.dir, "goal.txt")
	if err := os.WriteFile(path, []byte("0123456789abcdefghij\nxx\nthird line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.Settings.Editor.ScrollLines = 1

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 10
	h.redraw()
	h.exec("view.scrollDown")
	if editor.Cursor.Line != 1 || editor.Cursor.Col != 2 {
		t.Fatalf("Scroll Down moved the cursor to (%d,%d), want (1,2)", editor.Cursor.Line, editor.Cursor.Col)
	}
	h.exec("view.scrollUp")
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 10 {
		t.Fatalf("Scroll Up moved the cursor to (%d,%d), want (0,10)", editor.Cursor.Line, editor.Cursor.Col)
	}

	h.exec("view.scrollDown")
	h.pressKey(tcell.KeyLeft, tcell.ModNone)
	h.exec("view.scrollUp")
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 1 {
		t.Fatalf("cursor move did not reset the goal: got (%d,%d), want (0,1)", editor.Cursor.Line, editor.Cursor.Col)
	}
}

func TestScrollGoalColumnAcrossShortLines(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	path := filepath.Join(h.dir, "goal_lines.txt")
	content := "0123456789abcdefghij\nxx\n0123456789abcdefghij\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.Settings.Editor.ScrollLines = 1

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 10
	h.redraw()
	h.exec("view.scrollDown")
	if editor.Cursor.Line != 1 || editor.Cursor.Col != 2 {
		t.Fatalf("first down = (%d,%d), want (1,2)", editor.Cursor.Line, editor.Cursor.Col)
	}
	h.exec("view.scrollDown")
	if editor.Cursor.Line != 2 || editor.Cursor.Col != 10 {
		t.Fatalf("second down = (%d,%d), want (2,10)", editor.Cursor.Line, editor.Cursor.Col)
	}
	h.exec("view.scrollUp")
	h.exec("view.scrollUp")
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 10 {
		t.Fatalf("scrolling back = (%d,%d), want (0,10)", editor.Cursor.Line, editor.Cursor.Col)
	}

	// An edit invalidates the goal: scrolling back uses the typed column.
	h.exec("view.scrollDown")
	h.pressRune('Z')
	h.exec("view.scrollUp")
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 3 {
		t.Fatalf("edit did not reset the goal: got (%d,%d), want (0,3)", editor.Cursor.Line, editor.Cursor.Col)
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

func TestScrollLineCommandsAvailable(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	for _, title := range []string{"Scroll Line Up", "Scroll Line Down"} {
		cmd, ok := h.reg.FindByTitle(title)
		if !ok || cmd.Hidden {
			t.Fatalf("%q missing from command palette", title)
		}
	}
}

func TestScrollLineViewportOnly(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)
	placeScroll(t, h, 10, 0)

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Col = 3
	editor.Selection.Start(5, 0)
	h.redraw()

	// The cursor starts above the viewport and must stay there.
	h.exec("view.scrollLineDown")
	if editor.Viewport.TopLine != 11 {
		t.Fatalf("Scroll Line Down moved the viewport to %d, want 11", editor.Viewport.TopLine)
	}
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 3 {
		t.Fatalf("Scroll Line Down moved the cursor to (%d,%d), want (0,3)", editor.Cursor.Line, editor.Cursor.Col)
	}
	if !editor.Selection.Active || editor.Selection.Anchor.Line != 5 || editor.Selection.Anchor.Col != 0 {
		t.Fatalf("Scroll Line Down changed the selection to %+v", editor.Selection)
	}

	h.exec("view.scrollLineUp")
	if editor.Viewport.TopLine != 10 {
		t.Fatalf("Scroll Line Up moved the viewport to %d, want 10", editor.Viewport.TopLine)
	}
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 3 {
		t.Fatalf("Scroll Line Up moved the cursor to (%d,%d), want (0,3)", editor.Cursor.Line, editor.Cursor.Col)
	}
}

func TestScrollLineDefaultKeybindings(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)
	placeScroll(t, h, 0, 5)

	editor := h.app.EditorGroup.Editor
	h.pressKey(tcell.KeyDown, tcell.ModCtrl)
	if editor.Viewport.TopLine != 1 || editor.Cursor.Line != 5 {
		t.Fatalf("Ctrl+Down moved to top %d line %d, want 1/5", editor.Viewport.TopLine, editor.Cursor.Line)
	}
	h.pressKey(tcell.KeyUp, tcell.ModCtrl)
	if editor.Viewport.TopLine != 0 || editor.Cursor.Line != 5 {
		t.Fatalf("Ctrl+Up moved to top %d line %d, want 0/5", editor.Viewport.TopLine, editor.Cursor.Line)
	}
}

func TestMouseSelectionAfterViewportScroll(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	openScrollFile(t, h)
	for range 20 {
		h.pressKey(tcell.KeyDown, tcell.ModCtrl)
	}
	h.redraw()

	e := h.app.EditorGroup.Editor
	r := e.GetRect()
	x := r.X + e.GutterWidth() + 2
	y := r.Y + 2
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	h.app.Root.HandleEvent(tcell.NewEventMouse(x+4, y, tcell.Button1, tcell.ModNone))
	h.app.Root.HandleEvent(tcell.NewEventMouse(x+4, y, tcell.ButtonNone, tcell.ModNone))
	h.redraw()
	if !e.Selection.Active {
		t.Fatalf("drag did not select text after viewport scroll; top=%d cursor=%+v", e.Viewport.TopLine, e.Cursor)
	}
	if h.app.EditorGroup.SelectedText() == "" {
		t.Fatal("selected text is empty after viewport scroll")
	}
}

func TestScrollLineStopsAtEdges(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)

	editor := h.app.EditorGroup.Editor
	placeScroll(t, h, 0, 0)
	h.exec("view.scrollLineUp")
	if editor.Viewport.TopLine != 0 || editor.Cursor.Line != 0 {
		t.Fatalf("Scroll Line Up moved to top %d line %d, want 0/0", editor.Viewport.TopLine, editor.Cursor.Line)
	}

	last := len(editor.Buf.Lines) - 1
	placeScroll(t, h, last, 0)
	h.exec("view.scrollLineDown")
	if editor.Viewport.TopLine != last || editor.Cursor.Line != 0 {
		t.Fatalf("Scroll Line Down moved to top %d line %d, want %d/0", editor.Viewport.TopLine, editor.Cursor.Line, last)
	}
}

func TestScrollLineKeepsCursorAcrossFold(t *testing.T) {
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
	path := filepath.Join(h.dir, "scroll_line_fold.go")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.exec("fold.collapseAll")
	editor := h.app.EditorGroup.Editor
	editor.Viewport.TopLine = 2
	editor.Cursor.Line, editor.Cursor.Col = 2, 0
	h.redraw()

	// The closing brace is the next visible row after the collapsed body.
	h.exec("view.scrollLineDown")
	if editor.Viewport.TopLine != 23 || editor.Cursor.Line != 2 {
		t.Fatalf("scroll moved to top %d line %d, want 23/2", editor.Viewport.TopLine, editor.Cursor.Line)
	}
	h.exec("view.scrollLineUp")
	if editor.Viewport.TopLine != 2 || editor.Cursor.Line != 2 {
		t.Fatalf("scroll back moved to top %d line %d, want 2/2", editor.Viewport.TopLine, editor.Cursor.Line)
	}
}

func TestScrollLineHidesCursorOutsideViewport(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)
	placeScroll(t, h, 0, 5)

	editor := h.app.EditorGroup.Editor
	if _, _, visible := h.app.Root.CursorPosition(); !visible {
		t.Fatal("cursor should be visible before scrolling")
	}
	for range 10 {
		h.exec("view.scrollLineDown")
	}
	if _, _, visible := h.app.Root.CursorPosition(); visible {
		t.Fatalf("cursor outside viewport reported visible at (%d,%d)", editor.CursorX, editor.CursorY)
	}
}

func TestScrollLineHidesCursorOnHorizontalScrollbar(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = strings.Repeat("x", 200)
	}
	path := filepath.Join(h.dir, "wide.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	editor := h.app.EditorGroup.Editor
	h.redraw()

	// Put the cursor one row below the text viewport, on the horizontal
	// scrollbar row that is still inside the editor rect.
	editor.Viewport.TopLine = 10
	editor.Cursor.Line, editor.Cursor.Col = 10+editor.Viewport.Height, 0
	h.redraw()
	if x, y, visible := h.app.Root.CursorPosition(); visible {
		t.Fatalf("cursor on scrollbar row reported visible at (%d,%d)", x, y)
	}
}

func TestCursorHiddenInGutterWhenHorizontallyScrolled(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openScrollFile(t, h)
	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 0
	editor.Viewport.LeftCol = editor.GutterWidth() - 1
	h.redraw()
	if x, y, visible := h.app.Root.CursorPosition(); visible {
		t.Fatalf("cursor in gutter reported visible at (%d,%d)", x, y)
	}
}
