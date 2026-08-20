package ui

import "testing"

func multiLineGroup(t *testing.T, lines int) *EditorGroupWidget {
	t.Helper()
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	content := make([]string, lines)
	for i := range content {
		content[i] = "package line"
	}
	g.tabs[0].Buf.Lines = content
	return g
}

// PlaceCursor must never scroll. Startup places cursors before the first
// render, and a scroll computed against a zero-height viewport leaves the tab
// mis-framed for as long as it stays open.
func TestPlaceCursorLeavesViewportAlone(t *testing.T) {
	g := multiLineGroup(t, 200)
	topBefore := g.Editor.Viewport.TopLine
	leftBefore := g.Editor.Viewport.LeftCol
	heightBefore := g.Editor.Viewport.Height

	if !g.PlaceCursor(120, 5) {
		t.Fatal("PlaceCursor reported no active editor")
	}
	if g.Editor.Cursor.Line != 119 || g.Editor.Cursor.Col != 4 {
		t.Errorf("cursor = (%d, %d), want (119, 4)", g.Editor.Cursor.Line, g.Editor.Cursor.Col)
	}
	if g.Editor.Viewport.TopLine != topBefore {
		t.Errorf("TopLine moved to %d, want %d — PlaceCursor must not scroll", g.Editor.Viewport.TopLine, topBefore)
	}
	if g.Editor.Viewport.LeftCol != leftBefore {
		t.Errorf("LeftCol moved to %d, want %d — PlaceCursor must not scroll", g.Editor.Viewport.LeftCol, leftBefore)
	}
	if g.Editor.Viewport.Height != heightBefore {
		t.Errorf("Height changed to %d, want %d", g.Editor.Viewport.Height, heightBefore)
	}
}

func TestPlaceCursorClampsToBuffer(t *testing.T) {
	g := multiLineGroup(t, 10)

	g.PlaceCursor(9999, 1)
	if g.Editor.Cursor.Line != 9 {
		t.Errorf("line past EOF = %d, want 9", g.Editor.Cursor.Line)
	}

	g.PlaceCursor(0, 0)
	if g.Editor.Cursor.Line != 0 {
		t.Errorf("line before start = %d, want 0", g.Editor.Cursor.Line)
	}

	// "package line" is 12 runes; a column past the end lands at the end.
	g.PlaceCursor(3, 500)
	if g.Editor.Cursor.Col != 12 {
		t.Errorf("column past EOL = %d, want 12", g.Editor.Cursor.Col)
	}

	// Columns 0 and 1 both mean the start of the line.
	g.PlaceCursor(3, 1)
	if g.Editor.Cursor.Col != 0 {
		t.Errorf("column 1 = %d, want 0", g.Editor.Cursor.Col)
	}
}

// GoToLine, in contrast, is expected to scroll — it is what brings a line into
// view once the viewport has a real height.
func TestGoToLineDoesScroll(t *testing.T) {
	g := multiLineGroup(t, 200)
	g.Editor.Viewport.Height = 30

	g.GoToLine(120)
	if g.Editor.Viewport.TopLine == 0 {
		t.Error("GoToLine left TopLine at 0; it should have scrolled to the target")
	}
	if g.Editor.Cursor.Line != 119 {
		t.Errorf("cursor line = %d, want 119", g.Editor.Cursor.Line)
	}
}
