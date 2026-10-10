package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/undo"

	"github.com/gdamore/tcell/v3"
)

type liveDiffFixture struct {
	d    *DiffEditorWidget
	e    *EditorPaneWidget
	w, h int
}

func newLiveDiffFixture(t *testing.T, old, cur []string, mode DiffMode, w, h int) *liveDiffFixture {
	t.Helper()
	e := newRowsEditor(cur, w, h)
	e.LineNumbers = true
	e.Undo = &undo.UndoStack{}
	d := NewEditableDiffWidget("f.txt", old, diff.FullDiffLines(old, cur))
	d.SetMode(mode)
	d.bind(e)
	d.SetRect(Rect{X: 0, Y: 0, W: w, H: h})
	return &liveDiffFixture{d: d, e: e, w: w, h: h}
}

func (f *liveDiffFixture) render() []string {
	f.d.bind(f.e)
	grid := makeGrid(f.w, f.h)
	f.d.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: f.w, H: f.h}))
	rows := make([]string, f.h)
	for y := range grid {
		var sb strings.Builder
		for _, c := range grid[y] {
			sb.WriteRune(c.Ch)
		}
		rows[y] = sb.String()
	}
	return rows
}

func (f *liveDiffFixture) key(k tcell.Key) {
	f.d.HandleEvent(tcell.NewEventKey(k, "", tcell.ModNone))
	f.render()
}

func rowWith(rows []string, text string) int {
	for y, r := range rows {
		if strings.Contains(r, text) {
			return y
		}
	}
	return -1
}

func gapFile() (old, cur []string) {
	for i := 1; i <= 30; i++ {
		line := fmt.Sprintf("line %02d", i)
		old = append(old, line)
		if i == 15 {
			line = "changed 15"
		}
		cur = append(cur, line)
	}
	return old, cur
}

func TestEditableChangesOnlyShowsGapLabels(t *testing.T) {
	old, cur := gapFile()
	for _, mode := range []DiffMode{DiffModeUnified, DiffModeSplit} {
		f := newLiveDiffFixture(t, old, cur, mode, 100, 20)
		rows := f.render()
		screen := strings.Join(rows, "\n")
		for _, want := range []string{"⋯ 11 lines ⋯", "⋯ 12 lines ⋯", "line 12", "changed 15", "line 18"} {
			if !strings.Contains(screen, want) {
				t.Fatalf("mode %v: missing %q:\n%s", mode, want, screen)
			}
		}
		for _, hidden := range []string{"line 05", "line 11", "line 19", "line 25"} {
			if strings.Contains(screen, hidden) {
				t.Fatalf("mode %v: %q should be collapsed:\n%s", mode, hidden, screen)
			}
		}
		if mode == DiffModeSplit {
			y := rowWith(rows, "⋯ 11 lines ⋯")
			if strings.Count(rows[y], "⋯ 11 lines ⋯") != 2 {
				t.Fatalf("gap row not aligned across panes: %q", rows[y])
			}
		}
	}
}

func TestEditableGapLabelSingular(t *testing.T) {
	var old, cur []string
	for i := 1; i <= 9; i++ {
		line := fmt.Sprintf("line %02d", i)
		old = append(old, line)
		if i == 1 || i == 9 {
			line = "changed"
		}
		cur = append(cur, line)
	}
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 80, 20)
	screen := strings.Join(f.render(), "\n")
	if !strings.Contains(screen, " ⋯ 1 line ⋯") {
		t.Fatalf("missing singular gap label:\n%s", screen)
	}
}

func TestEditableGapClickExpands(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 100, 20)
	rows := f.render()
	y := rowWith(rows, "⋯ 12 lines ⋯")
	f.d.HandleEvent(tcell.NewEventMouse(20, y, tcell.Button1, tcell.ModNone))
	f.d.HandleEvent(tcell.NewEventMouse(20, y, tcell.ButtonNone, tcell.ModNone))
	screen := strings.Join(f.render(), "\n")
	if !strings.Contains(screen, "line 25") || strings.Contains(screen, "⋯ 12 lines ⋯") {
		t.Fatalf("click did not expand the gap:\n%s", screen)
	}
	if !strings.Contains(screen, "⋯ 11 lines ⋯") {
		t.Fatalf("click expanded the other gap too:\n%s", screen)
	}
}

func TestEditableGapEnterExpandsAndCursorSkipsHiddenLines(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 100, 20)
	f.render()
	if f.e.Cursor.Line != 11 {
		t.Fatalf("initial cursor on line %d, want the first shown line 11", f.e.Cursor.Line)
	}
	f.e.Cursor.Line = 17
	f.render()
	f.key(tcell.KeyDown)
	if f.e.Cursor.Line != 18 {
		t.Fatalf("down onto the gap row = line %d, want 18", f.e.Cursor.Line)
	}
	f.key(tcell.KeyUp)
	f.key(tcell.KeyUp)
	if f.e.Cursor.Line != 16 {
		t.Fatalf("up from the gap row = line %d, want 16", f.e.Cursor.Line)
	}
	f.key(tcell.KeyDown)
	f.key(tcell.KeyDown)
	f.key(tcell.KeyEnter)
	if got := strings.Join(f.e.Buf.Lines, "\n"); got != strings.Join(cur, "\n") {
		t.Fatalf("Enter on a gap row edited the buffer")
	}
	screen := strings.Join(f.render(), "\n")
	if !strings.Contains(screen, "line 25") || strings.Contains(screen, "⋯ 12 lines ⋯") {
		t.Fatalf("Enter did not expand the gap:\n%s", screen)
	}
}

func TestEditableCursorJumpIntoGapExpandsIt(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 100, 40)
	f.render()
	f.e.Cursor.Line = 4
	screen := strings.Join(f.render(), "\n")
	if !strings.Contains(screen, "line 05") {
		t.Fatalf("jumping the cursor into a gap did not reveal it:\n%s", screen)
	}
}

func TestEditableDiffHasNoFolds(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 100, 20)
	f.e.Folds = fold.NewState()
	f.e.Folds.SetRanges(fold.ComputeIndentRanges(f.e.Buf.Lines))
	f.render()
	if f.e.Folds != nil {
		t.Fatal("editable diff pane kept the file's folds")
	}
}
