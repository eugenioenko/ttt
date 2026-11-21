package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

var conflictedLines = []string{
	"line1",
	"<<<<<<< HEAD",
	"current",
	"||||||| base",
	"base",
	"=======",
	"incoming",
	">>>>>>> other",
	"line3",
}

func actionX(e *EditorPaneWidget, action int) int {
	span := conflictActionSpans()[action]
	return e.GetRect().X + e.GutterWidth() + span.start + 1
}

func TestMergeConflictActionRowAboveHeader(t *testing.T) {
	e := newRowsEditor(append([]string(nil), conflictedLines...), 70, 10)
	rows := renderRows(e)
	want := "[Accept Current]  [Accept Incoming]  [Accept Both]"
	if rows[0] != "line1" || rows[1] != want || rows[2] != "<<<<<<< HEAD" {
		t.Fatalf("rows = %q", rows[:4])
	}
	if row := e.rowAt(1); !row.conflict || row.bufLine != 1 {
		t.Fatalf("row 1 = %+v, want the conflict action row of line 1", row)
	}
	if row := e.rowAt(2); row.isPhantom() || row.bufLine != 1 {
		t.Fatalf("row 2 = %+v, want the header line", row)
	}
}

func TestMergeConflictResolveActions(t *testing.T) {
	tests := []struct {
		name   string
		action int
		want   []string
	}{
		{"current", 0, []string{"line1", "current", "line3"}},
		{"incoming", 1, []string{"line1", "incoming", "line3"}},
		{"both", 2, []string{"line1", "current", "incoming", "line3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newRowsEditor(append([]string(nil), conflictedLines...), 70, 10)
			renderRows(e)
			clickAt(e, actionX(e, tt.action), e.GetRect().Y+1)
			if got := e.Buf.Lines; strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Fatalf("lines = %q, want %q", got, tt.want)
			}
			if e.Cursor.Line != 1 || e.Cursor.Col != 0 {
				t.Fatalf("cursor = (%d,%d), want (1,0)", e.Cursor.Line, e.Cursor.Col)
			}
			if _, ok := conflictAt(e.Buf.Lines, 1); ok {
				t.Fatal("conflict survived resolution")
			}
			if rows := renderRows(e); strings.Contains(strings.Join(rows, "\n"), "Accept") {
				t.Fatalf("action row survived resolution: %q", rows)
			}
		})
	}
}

func TestMergeConflictWithoutBaseSection(t *testing.T) {
	lines := []string{"<<<<<<< HEAD", "current", "=======", "incoming", ">>>>>>> other"}
	e := newRowsEditor(append([]string(nil), lines...), 70, 8)
	renderRows(e)
	clickAt(e, actionX(e, 0), e.GetRect().Y)
	if got := strings.Join(e.Buf.Lines, "\n"); got != "current" {
		t.Fatalf("current side = %q", got)
	}

	e = newRowsEditor(append([]string(nil), lines...), 70, 8)
	renderRows(e)
	clickAt(e, actionX(e, 1), e.GetRect().Y)
	if got := strings.Join(e.Buf.Lines, "\n"); got != "incoming" {
		t.Fatalf("incoming side = %q", got)
	}
}

func TestMergeConflictEmptySideKeepsOneLine(t *testing.T) {
	e := newRowsEditor([]string{"<<<<<<< HEAD", "=======", "incoming", ">>>>>>> other"}, 70, 8)
	renderRows(e)
	clickAt(e, actionX(e, 0), e.GetRect().Y)
	if len(e.Buf.Lines) != 1 || e.Buf.Lines[0] != "" {
		t.Fatalf("lines = %q, want one empty line", e.Buf.Lines)
	}
}

func TestMergeConflictHoverFeedback(t *testing.T) {
	e := newRowsEditor(append([]string(nil), conflictedLines...), 70, 10)
	renderGrid(e)
	x := actionX(e, 1)
	e.HandleEvent(tcell.NewEventMouse(x, e.GetRect().Y+1, tcell.ButtonNone, tcell.ModNone))
	grid := renderGrid(e)
	if cell := grid[1][x-e.GetRect().X]; cell.Style != term.StyleButtonFocused {
		t.Fatalf("hovered button style = %v, want focused", cell.Style)
	}
	if shape := e.PointerShapeAt(x, e.GetRect().Y+1); shape != term.PointerHand {
		t.Fatalf("pointer shape = %q, want hand", shape)
	}
	if e.conflictHover != [2]int{1, 1} {
		t.Fatalf("conflictHover = %v", e.conflictHover)
	}
}

func TestMergeConflictRowsFollowEdits(t *testing.T) {
	e := newRowsEditor([]string{"line1", "line2"}, 70, 10)
	if rows := renderRows(e); strings.Contains(strings.Join(rows, "\n"), "Accept") {
		t.Fatalf("unexpected action row: %q", rows)
	}

	e.Buf.Splice(1, 1, "<<<<<<< HEAD", "current", "=======", "incoming", ">>>>>>> other")
	rows := renderRows(e)
	if !strings.HasPrefix(rows[1], "[Accept Current]") || rows[2] != "<<<<<<< HEAD" {
		t.Fatalf("rows after inserting a conflict = %q", rows[:3])
	}

	e.Buf.Splice(1, 5, "line2")
	rows = renderRows(e)
	if strings.Contains(strings.Join(rows, "\n"), "Accept") || rows[1] != "line2" {
		t.Fatalf("rows after removing the conflict = %q", rows)
	}
}

func TestMergeConflictResolutionUndoesBackToMarkers(t *testing.T) {
	e := newRowsEditor(append([]string(nil), conflictedLines...), 70, 10)
	e.Undo = &undo.UndoStack{}
	renderRows(e)
	clickAt(e, actionX(e, 2), e.GetRect().Y+1)
	if got := strings.Join(e.Buf.Lines, "\n"); got != "line1\ncurrent\nincoming\nline3" {
		t.Fatalf("lines = %q", got)
	}
	e.Undo.Undo(e.Buf)
	rows := renderRows(e)
	if !strings.HasPrefix(rows[1], "[Accept Current]") || rows[2] != "<<<<<<< HEAD" {
		t.Fatalf("rows after undo = %q", rows[:3])
	}
	if got := strings.Join(e.Buf.Lines, "\n"); got != strings.Join(conflictedLines, "\n") {
		t.Fatalf("lines after undo = %q", got)
	}
}

func TestMergeConflictReadOnlyPaneHidesActions(t *testing.T) {
	e := newRowsEditor(append([]string(nil), conflictedLines...), 70, 10)
	e.ReadOnly = true
	rows := renderRows(e)
	if rows[1] != "<<<<<<< HEAD" {
		t.Fatalf("rows = %q", rows[:3])
	}
}

func TestConflictAtRequiresCompleteBlock(t *testing.T) {
	if _, ok := conflictAt([]string{"<<<<<<< HEAD", "current"}, 0); ok {
		t.Fatal("unterminated conflict accepted")
	}
	if _, ok := conflictAt([]string{"current"}, 0); ok {
		t.Fatal("line without a header accepted")
	}
	c, ok := conflictAt([]string{"<<<<<<< HEAD", "current", "||||||| b", "base", "=======", "incoming", ">>>>>>> x"}, 0)
	if !ok || c.start != 0 || c.base != 2 || c.separator != 4 || c.end != 6 {
		t.Fatalf("conflict = %+v, %v", c, ok)
	}
}
