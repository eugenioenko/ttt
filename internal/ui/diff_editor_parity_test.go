package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

func liveFind(d *DiffEditorWidget, query string) []FindMatch {
	left, _ := FindInLines(d.LeftLines(), query, SearchOptions{})
	right, _ := FindInLines(d.RightLines(), query, SearchOptions{})
	return d.SetSearchMatches(left, right)
}

func TestReadOnlyDiffShowsGapsAroundTheChanges(t *testing.T) {
	old, cur := gapFile()
	d := NewDiffEditorWidget("f.txt", diff.Parse(diff.Generate(old, cur, "f.txt")), old, cur, false)
	d.SetUnified(true)
	text := strings.Join(d.unified.Buf.Lines, "\n")
	if !strings.Contains(text, "⋯ 11 lines ⋯") || !strings.Contains(text, "⋯ 12 lines ⋯") {
		t.Fatalf("read-only diff rows = %q, want gaps above and below the change like an editable diff", d.unified.Buf.Lines)
	}
}

func TestEditableFindSearchesTheBaseSide(t *testing.T) {
	old, cur := gapFile()
	for _, mode := range []DiffMode{DiffModeUnified, DiffModeSplit} {
		f := newLiveDiffFixture(t, old, cur, mode, 100, 20)
		f.render()
		matches := liveFind(f.d, "line 15")
		if len(matches) != 1 {
			t.Fatalf("mode %v: matches = %v, want the removed base line", mode, matches)
		}
		f.d.SetActiveMatch(0)
		f.d.ScrollToLine(matches[0].Line)
		rows := f.render()
		y := rowWith(rows, "line 15")
		if y < 0 {
			t.Fatalf("mode %v: base match not on screen:\n%s", mode, strings.Join(rows, "\n"))
		}
		if mode == DiffModeUnified {
			if o := f.e.DiffOverlay; o.DeletedActive[0] < 0 {
				t.Fatalf("removed-line match is not active: %+v", o.DeletedActive)
			}
		} else if !f.d.headFocused() || f.d.left.SearchActive != 0 {
			t.Fatalf("split base match did not move to the base pane")
		}
	}
}

func TestEditableFindRevealsMatchesInsideGaps(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 100, 20)
	f.render()
	matches := liveFind(f.d, "line 03")
	if len(matches) != 1 {
		t.Fatalf("matches = %v, want one hidden file line", matches)
	}
	f.d.SetActiveMatch(0)
	f.d.ScrollToLine(matches[0].Line)
	rows := f.render()
	if rowWith(rows, "line 03") < 0 || f.e.Cursor.Line != 2 {
		t.Fatalf("hidden match not revealed (cursor %d):\n%s", f.e.Cursor.Line, strings.Join(rows, "\n"))
	}
}

func TestEditableDeletedLineMatchIsHighlighted(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 100, 20)
	f.render()
	liveFind(f.d, "line 15")
	f.d.SetActiveMatch(0)
	f.d.bind(f.e)
	grid := makeGrid(100, 20)
	f.d.Render(NewRenderSurface(grid, Rect{W: 100, H: 20}))
	found := false
	for _, row := range grid {
		for _, c := range row {
			if c.Style == term.StyleSearchActive {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("active match on a removed line is not highlighted")
	}
}

func TestEditableSplitKeysFollowTheClickedPane(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeSplit, 100, 20)
	f.d.SetContextMode(DiffContextFullFile)
	f.render()
	left := f.d.left.GetRect()
	f.d.HandleEvent(tcell.NewEventMouse(left.X+10, left.Y+2, tcell.Button1, tcell.ModNone))
	f.d.HandleEvent(tcell.NewEventMouse(left.X+10, left.Y+2, tcell.ButtonNone, tcell.ModNone))
	f.render()
	if !f.d.headFocused() {
		t.Fatal("clicking the base pane did not make it the key target")
	}
	headLine, fileLine, before := f.d.left.Cursor.Line, f.e.Cursor.Line, strings.Join(f.e.Buf.Lines, "\n")
	f.key(tcell.KeyDown)
	f.d.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	if f.d.left.Cursor.Line != headLine+1 || f.e.Cursor.Line != fileLine {
		t.Fatalf("down moved base %d→%d, file %d→%d", headLine, f.d.left.Cursor.Line, fileLine, f.e.Cursor.Line)
	}
	if strings.Join(f.e.Buf.Lines, "\n") != before {
		t.Fatal("typing with the base pane focused edited the file")
	}

	right := f.e.GetRect()
	f.d.HandleEvent(tcell.NewEventMouse(right.X+12, right.Y+2, tcell.Button1, tcell.ModNone))
	f.d.HandleEvent(tcell.NewEventMouse(right.X+12, right.Y+2, tcell.ButtonNone, tcell.ModNone))
	if f.d.headFocused() {
		t.Fatal("clicking the file side did not take the keys back")
	}
}

func TestEditableEditOnGapRowExpandsTheGap(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 100, 20)
	rows := f.render()
	gapRow := rowWith(rows, "⋯ 11 lines ⋯")
	if gapRow < 0 {
		t.Fatalf("no leading gap:\n%s", strings.Join(rows, "\n"))
	}
	line := f.e.rowAt(gapRow).bufLine
	f.e.Cursor.Line, f.e.Cursor.Col = line, 0
	f.render()
	f.e.pasteText("pasted ")
	rows = f.render()
	if rowWith(rows, "pasted line") < 0 {
		t.Fatalf("edit on a gap row stayed hidden:\n%s", strings.Join(rows, "\n"))
	}
}

func TestEditableEditAboveGapKeepsItCollapsed(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 100, 20)
	f.render()
	f.e.Cursor.Line, f.e.Cursor.Col = 14, len("changed 15")
	f.render()
	f.key(tcell.KeyEnter)
	if rows := f.render(); rowWith(rows, "⋯ 12 lines ⋯") < 0 {
		t.Fatalf("an edit next to a gap expanded it:\n%s", strings.Join(rows, "\n"))
	}
}
