package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/gdamore/tcell/v3"
)

func navigationDetail(wrapped bool) *CommitDetailWidget {
	var patch strings.Builder
	patch.WriteString("--- a/f.txt\n+++ b/f.txt\n@@ -1,60 +1,60 @@\n")
	for i := 1; i <= 60; i++ {
		if i == 5 || i == 25 || i == 45 {
			fmt.Fprintf(&patch, "-old%d\n+new%d\n", i, i)
		} else {
			fmt.Fprintf(&patch, " line%d %s\n", i, strings.Repeat("context ", 12))
		}
	}
	d := NewCommitDetailWidget("", "", "", false)
	d.SetDetail("", []CommitDetailFile{{Path: "f.txt", Diff: diff.Parse(patch.String())}}, "")
	d.SetWrapped(wrapped)
	return d
}

func TestCommitDetailHunkTargetsStayVisible(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		for _, wrapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%d/wrap=%v", mode, wrapped), func(t *testing.T) {
				d := navigationDetail(wrapped)
				d.SetMode(mode)
				renderDetail(d, 100, 10)
				for i, line := range []int{5, 25, 45, 5} {
					index := i%3 + 1
					if err := d.GoToHunk(index); err != nil {
						t.Fatal(err)
					}
					grid := renderDetail(d, 100, 10)
					if _, y, ok := detailFind(grid, fmt.Sprintf("new%d", line)); !ok || y == 0 {
						t.Fatalf("hunk %d hidden after jump:\n%s", index, detailTestText(grid))
					}
				}
			})
		}
	}
}

func TestCommitDetailViewportUsesAbsoluteEmbeddedRows(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		for _, wrapped := range []bool{false, true} {
			d := navigationDetail(wrapped)
			d.SetMode(mode)
			renderDetail(d, 100, 10)
			d.TopLine = 15
			renderDetail(d, 100, 10)
			_, row := d.ViewportFileAndRow()
			visual := d.visualRows[d.TopLine+d.stickyRows()]
			v := d.rowView(visual.row)
			line, _ := v.lead().layout().rowToTop(visual.offset)
			want, _ := v.diffRowForBufLine(v.lead(), line)
			if row != want {
				t.Errorf("mode=%d wrap=%v: viewport diff row=%d, want %d", mode, wrapped, row, want)
			}
		}
	}
}

func TestCommitDetailScrollbarClearsHunkAnchor(t *testing.T) {
	d := navigationDetail(false)
	renderDetail(d, 100, 10)
	if err := d.GoToHunk(3); err != nil {
		t.Fatal(err)
	}
	renderDetail(d, 100, 10)
	before := d.TopLine
	d.HandleEvent(tcell.NewEventMouse(d.scrollbar.X, d.scrollbar.Y, tcell.Button1, 0))
	if d.TopLine == before {
		t.Fatal("scrollbar did not move")
	}
	file, row := d.ViewportFileAndRow()
	if got := d.HunkAt(file, row); got != 0 {
		t.Fatalf("scrollbar moved to top but current_hunk=%d, want 0", got)
	}
}

func TestCommitDetailDisclosureScrollClearsHunkAnchor(t *testing.T) {
	d := navigationDetail(false)
	renderDetail(d, 100, 10)
	if err := d.GoToHunk(3); err != nil {
		t.Fatal(err)
	}
	renderDetail(d, 100, 10)
	before := d.TopLine
	d.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", 0))
	if d.TopLine >= before {
		t.Fatal("Tab did not scroll to the commit header")
	}
	renderDetail(d, 100, 10)
	file, row := d.ViewportFileAndRow()
	if got := d.HunkAt(file, row); got != 0 {
		t.Fatalf("Tab scrolled to the header but current_hunk=%d, want 0", got)
	}
}

func TestCommitDetailDisclosureWithoutScrollKeepsHunkAnchor(t *testing.T) {
	d := commitDetailWithFiles(t, "f.txt")
	renderDetail(d, 167, 40)
	if err := d.GoToHunk(2); err != nil {
		t.Fatal(err)
	}
	renderDetail(d, 167, 40)
	before := d.TopLine
	d.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", 0))
	if d.TopLine != before {
		t.Fatal("Tab unexpectedly scrolled")
	}
	file, row := d.ViewportFileAndRow()
	if got := d.HunkAt(file, row); got != 2 {
		t.Fatalf("Tab without scrolling changed current_hunk=%d, want 2", got)
	}
}

func TestCommitDetailMessageDoesNotSelectOffscreenHunk(t *testing.T) {
	d := NewCommitDetailWidget("", "", "", false)
	d.SetDetail(strings.Repeat("Commit description\n\n", 20), []CommitDetailFile{{
		Path: "f.txt",
		Diff: diff.Parse("--- a/f.txt\n+++ b/f.txt\n@@ -1 +1 @@\n-first\n+FIRST\n@@ -10 +10 @@\n-last\n+LAST\n"),
	}}, "")
	grid := renderDetail(d, 100, 10)
	if _, _, visible := detailFind(grid, "FIRST"); visible {
		t.Fatal("fixture must keep the diff below the viewport")
	}
	file, row := d.ViewportFileAndRow()
	at := d.HunkAt(file, row)
	if at != 0 {
		t.Fatalf("message-only viewport reports current_hunk=%d, want 0", at)
	}
	if err := d.GoToHunk(at + 1); err != nil {
		t.Fatal(err)
	}
	grid = renderDetail(d, 100, 10)
	if _, _, visible := detailFind(grid, "FIRST"); !visible {
		t.Fatalf("Next skipped the first hunk:\n%s", detailTestText(grid))
	}
}

func TestCommitDetailReplacementClearsHunkAnchor(t *testing.T) {
	for _, current := range []bool{false, true} {
		d := navigationDetail(false)
		d.CurrentChanges = current
		renderDetail(d, 100, 10)
		if err := d.GoToHunk(3); err != nil {
			t.Fatal(err)
		}
		files := []CommitDetailFile{{Path: "other.txt", Diff: diff.Parse("--- a/other.txt\n+++ b/other.txt\n@@ -1,3 +1,3 @@\n a\n b\n-c\n+C\n")}}
		if current {
			d.SetCurrentChanges("", files, "")
		} else {
			d.SetDetail("", files, "")
		}
		renderDetail(d, 100, 10)
		file, row := d.ViewportFileAndRow()
		if got := d.HunkAt(file, row); got != 0 {
			t.Errorf("current=%v: replaced detail retains current_hunk=%d, want 0", current, got)
		}
	}
}

func TestEditableHunkNavigationPreservesEOFDeletion(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		old := []string{"a", "b", "c", "d", "e", "f", "gone"}
		cur := slices.Clone(old[:6])
		f := newLiveDiffFixture(t, old, cur, mode, 100, 4)
		f.render()
		if err := f.d.GoToHunk(1); err != nil {
			t.Fatal(err)
		}
		screen := strings.Join(f.render(), "\n")
		if !strings.Contains(screen, "gone") {
			t.Errorf("mode=%d: deletion hidden after jump:\n%s", mode, screen)
		}
		if got := f.d.HunkAt(f.d.CursorRow()); got != 1 {
			t.Errorf("mode=%d: EOF deletion reports current_hunk=%d, want 1", mode, got)
		}
		if !slices.Equal(f.e.Buf.Lines, cur) {
			t.Fatal("navigation modified buffer")
		}
		f.d.HandleEvent(tcell.NewEventKey(tcell.KeyHome, "", 0))
		f.render()
		if got := f.d.HunkAt(f.d.CursorRow()); got != 0 {
			t.Errorf("mode=%d: cursor movement retained deletion: current_hunk=%d", mode, got)
		}
	}
}

func TestEditableHunkNavigationFollowsPendingEdits(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		for _, insert := range []bool{false, true} {
			old := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
			cur := slices.Clone(old)
			cur[6] = "CHANGED"
			f := newLiveDiffFixture(t, old, cur, mode, 100, 10)
			f.render()
			var edit undo.EditCommand = &undo.DeleteLineCommand{Idx: 0}
			if insert {
				edit = &undo.InsertLineCommand{Idx: 0, Text: "inserted"}
			}
			f.e.exec(edit)
			for step := 0; step < 3; step++ {
				f.render()
				want := slices.Index(f.e.Buf.Lines, "CHANGED")
				if err := f.d.GoToHunk(1); err != nil {
					t.Fatal(err)
				}
				if got := f.e.Cursor.Line; got != want {
					t.Errorf("mode=%d insert=%v step=%d: cursor=%d, want %d", mode, insert, step, got, want)
				}
				if got := f.d.HunkAt(f.d.fullRowForBufLine(want)); got != 1 {
					t.Errorf("mode=%d insert=%v step=%d: changed line reports hunk=%d, want 1", mode, insert, step, got)
				}
				if step == 0 {
					f.e.Undo.Undo(f.e.Buf)
				} else {
					f.e.Undo.Redo(f.e.Buf)
				}
			}
		}
	}
}
