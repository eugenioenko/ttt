package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoToLineViewportPosition(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "line "+string(rune('0'+i/100))+string(rune('0'+(i/10)%10))+string(rune('0'+i%10)))
	}
	f := filepath.Join(h.dir, "big.txt")
	requireNoError(t, os.WriteFile(f, []byte(strings.Join(lines, "\n")+"\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.app.EditorGroup.GoToLine(50)
	h.redraw()

	topLine := h.app.EditorGroup.Editor.Viewport.TopLine
	cursorLine := h.app.EditorGroup.Editor.Cursor.Line
	vpHeight := h.app.EditorGroup.Editor.Viewport.Height

	if cursorLine != 49 {
		t.Fatalf("expected cursor on line 49 (0-indexed), got %d", cursorLine)
	}

	margin := vpHeight / 3
	expectedTop := cursorLine - margin
	if topLine != expectedTop {
		t.Errorf("expected TopLine ~%d (1/3 margin), got %d", expectedTop, topLine)
	}

	if cursorLine < topLine || cursorLine >= topLine+vpHeight {
		t.Errorf("cursor line %d not visible in viewport [%d, %d)", cursorLine, topLine, topLine+vpHeight)
	}
}

func TestGoToLineCol(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "cols.txt")
	requireNoError(t, os.WriteFile(f, []byte("short\nabcdefghij\nx\n"), 0644))
	h.openFile(f)
	h.redraw()

	// Column within the line: 1-based col 4 lands on 0-based col 3.
	h.app.EditorGroup.GoToLineCol(2, 4)
	if l, c := h.app.EditorGroup.Editor.Cursor.Line, h.app.EditorGroup.Editor.Cursor.Col; l != 1 || c != 3 {
		t.Errorf("GoToLineCol(2,4): cursor = (%d,%d), want (1,3)", l, c)
	}

	// Column past the end of the line clamps to the line's rune length.
	h.app.EditorGroup.GoToLineCol(3, 99)
	if l, c := h.app.EditorGroup.Editor.Cursor.Line, h.app.EditorGroup.Editor.Cursor.Col; l != 2 || c != 1 {
		t.Errorf("GoToLineCol(3,99): cursor = (%d,%d), want (2,1)", l, c)
	}

	// Col 0 (no column captured) and col 1 keep the cursor at line start.
	h.app.EditorGroup.GoToLineCol(2, 0)
	if c := h.app.EditorGroup.Editor.Cursor.Col; c != 0 {
		t.Errorf("GoToLineCol(2,0): col = %d, want 0", c)
	}
	h.app.EditorGroup.GoToLineCol(2, 1)
	if c := h.app.EditorGroup.Editor.Cursor.Col; c != 0 {
		t.Errorf("GoToLineCol(2,1): col = %d, want 0", c)
	}
}
