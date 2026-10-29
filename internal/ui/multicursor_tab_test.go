package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func newMultiCursorEditor(lines []string, positions [][2]int) *EditorPaneWidget {
	e := newEditorWithLines(lines...)
	e.Cursor.Line, e.Cursor.Col = positions[0][0], positions[0][1]
	e.ensureMulti()
	for _, p := range positions[1:] {
		e.Multi.Add(p[0], p[1])
	}
	e.syncFromMulti()
	return e
}

// Secondary cursors must not borrow StyleSelection: in the default "background"
// selection style it carries no reverse attribute, so cursors render as a soft tint.
func TestMultiCursorSecondaryCursorsRenderAsBlock(t *testing.T) {
	e := newRowsEditor([]string{"foo", "bar"}, 20, 4)
	e.ensureMulti()
	e.Multi.Add(1, 1)
	e.syncFromMulti()
	grid := renderGrid(e)
	if c := grid[1][e.GutterWidth()+1]; c.Style != term.StyleReverse || c.BgStyle != 0 {
		t.Fatalf("secondary cursor cell style=%v bg=%v, want StyleReverse with no bg", c.Style, c.BgStyle)
	}
}

// Tab in multi-cursor mode inserts one indent unit at every cursor, not just the
// primary one.
func TestMultiCursorTabInsertsAtEachCursor(t *testing.T) {
	e := newMultiCursorEditor([]string{"foo", "bar"}, [][2]int{{0, 0}, {1, 0}})
	if !e.isMultiActive() {
		t.Fatal("expected multi-cursor mode to be active")
	}

	e.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", 0))

	if e.Buf.Lines[0] != "    foo" {
		t.Fatalf("expected first line indented, got %q", e.Buf.Lines[0])
	}
	if e.Buf.Lines[1] != "    bar" {
		t.Fatalf("expected second cursor's line indented too, got %q", e.Buf.Lines[1])
	}
}

// Shift+Tab in multi-cursor mode is a no-op (matching VS Code, and avoiding the
// old behavior where only the primary cursor's line was outdented).
func TestMultiCursorBacktabIsNoOp(t *testing.T) {
	e := newMultiCursorEditor([]string{"    foo", "    bar"}, [][2]int{{0, 0}, {1, 0}})
	if !e.isMultiActive() {
		t.Fatal("expected multi-cursor mode to be active")
	}

	e.HandleEvent(tcell.NewEventKey(tcell.KeyBacktab, "", 0))

	if e.Buf.Lines[0] != "    foo" || e.Buf.Lines[1] != "    bar" {
		t.Fatalf("expected no change on multi-cursor backtab, got %q / %q", e.Buf.Lines[0], e.Buf.Lines[1])
	}
}
