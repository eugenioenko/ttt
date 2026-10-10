package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

func compactLines(t *testing.T, unified string) []diff.DiffLine {
	t.Helper()
	lines, _ := compactDiffLinesWithContext(diff.Parse(unified), nil, nil, map[int]bool{})
	return lines
}

func TestCompactDiffSeparatorShowsLineDistance(t *testing.T) {
	lines := compactLines(t, "--- a/test.go\n+++ b/test.go\n@@ -22,3 +22,3 @@\n line 22\n line 23\n line 24\n@@ -356,2 +356,2 @@\n line 356\n line 357\n")
	if len(lines) < 4 {
		t.Fatalf("expected two hunks and a separator, got %v", lines)
	}
	separator := lines[3]
	if separator.Left.Text != " ⋯ 331 lines ⋯" || separator.Right.Text != " ⋯ 331 lines ⋯" {
		t.Fatalf("separator = %q / %q, want collapsed distance", separator.Left.Text, separator.Right.Text)
	}
}

func TestCompactDiffSeparatorUsesSingularLine(t *testing.T) {
	lines := compactLines(t, "--- a/test.go\n+++ b/test.go\n@@ -24,1 +24,1 @@\n line 24\n@@ -26,1 +26,1 @@\n line 26\n")
	if got := lines[1].Left.Text; got != " ⋯ 1 line ⋯" {
		t.Fatalf("separator = %q, want singular distance", got)
	}
}

func TestCompactDiffSeparatorOmitsAdjacentLines(t *testing.T) {
	lines := compactLines(t, "--- a/test.go\n+++ b/test.go\n@@ -24,1 +24,1 @@\n line 24\n@@ -25,1 +25,1 @@\n line 25\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want adjacent diff rows with no separator: %v", len(lines), lines)
	}
	if lines[0].Left.Text != "line 24" || lines[1].Left.Text != "line 25" {
		t.Fatalf("adjacent rows shifted: %q then %q", lines[0].Left.Text, lines[1].Left.Text)
	}
}

func TestDiffGutterMarksAndColorsChangedLines(t *testing.T) {
	grid := makeGrid(10, 2)
	surface := NewRenderSurface(grid, Rect{W: 10, H: 2})
	renderDiffGutterWithSigns(surface, 0, 0, 5, diff.SideLine{Num: 12, Kind: diff.Deleted}, term.StyleDefault, true)
	renderDiffGutterWithSigns(surface, 0, 1, 5, diff.SideLine{Num: 13, Kind: diff.Added}, term.StyleDefault, true)

	if deleted, added := cellRow(grid, 0), cellRow(grid, 1); !strings.Contains(deleted, "12 −") || !strings.Contains(added, "13 +") {
		t.Fatalf("changed gutters do not show line markers:\n%s\n%s", deleted, added)
	}
	for column := 0; column < 5; column++ {
		if grid[0][column].Style != term.StyleGutterDeleted {
			t.Fatalf("deleted gutter column %d style = %v, want StyleGutterDeleted", column, grid[0][column].Style)
		}
		if grid[0][column].BgStyle != term.StyleDiffDeleted {
			t.Fatalf("deleted gutter column %d background = %v, want StyleDiffDeleted", column, grid[0][column].BgStyle)
		}
		if grid[1][column].Style != term.StyleGutterAdded {
			t.Fatalf("added gutter column %d style = %v, want StyleGutterAdded", column, grid[1][column].Style)
		}
		if grid[1][column].BgStyle != term.StyleDiffAdded {
			t.Fatalf("added gutter column %d background = %v, want StyleDiffAdded", column, grid[1][column].BgStyle)
		}
	}
}

func TestDiffGutterRightAlignsLineNumbers(t *testing.T) {
	grid := makeGrid(12, 1)
	surface := NewRenderSurface(grid, Rect{W: 12, H: 1})
	renderDiffGutterWithSigns(surface, 0, 0, 6, diff.SideLine{Num: 1, Kind: diff.Added}, term.StyleDefault, true)
	renderDiffGutterWithSigns(surface, 6, 0, 6, diff.SideLine{Num: 123, Kind: diff.Deleted}, term.StyleDefault, true)
	if got := cellRow(grid, 0); got != "   1 + 123 −" {
		t.Fatalf("aligned diff gutters = %q, want right-aligned line numbers", got)
	}
}

func TestTextSegmentDoesNotDrawFullwidthRuneInLastColumn(t *testing.T) {
	grid := makeGrid(2, 1)
	surface := NewRenderSurface(grid, Rect{W: 2, H: 1})
	drawTextSegment(surface, 0, 0, 2, "a界", 0, 0, term.Cell{Ch: ' '}, func(_ int, ch rune) term.Cell {
		return term.Cell{Ch: ch, BgStyle: term.StyleCommitHeader}
	})
	if grid[0][0].Ch != 'a' || grid[0][1].Ch != ' ' {
		t.Fatalf("last-column fullwidth rendering = %q / %q, want 'a' then a safe space", grid[0][0].Ch, grid[0][1].Ch)
	}
	if grid[0][1].BgStyle != term.StyleCommitHeader {
		t.Fatalf("substituted last-column cell lost its background: %+v", grid[0][1])
	}
}
