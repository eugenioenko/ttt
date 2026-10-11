package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestDiffGutterMarksAndColorsChangedLines(t *testing.T) {
	grid := makeGrid(10, 2)
	surface := NewRenderSurface(grid, Rect{W: 10, H: 2})
	renderDiffGutter(surface, 0, 0, 5, diff.SideLine{Num: 12, Kind: diff.Deleted})
	renderDiffGutter(surface, 0, 1, 5, diff.SideLine{Num: 13, Kind: diff.Added})

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
	renderDiffGutter(surface, 0, 0, 6, diff.SideLine{Num: 1, Kind: diff.Added})
	renderDiffGutter(surface, 6, 0, 6, diff.SideLine{Num: 123, Kind: diff.Deleted})
	if got := cellRow(grid, 0); got != "   1 + 123 −" {
		t.Fatalf("aligned diff gutters = %q, want right-aligned line numbers", got)
	}
}

func TestDiffTextDoesNotDrawFullwidthRuneInLastColumn(t *testing.T) {
	grid := makeGrid(2, 1)
	surface := NewRenderSurface(grid, Rect{W: 2, H: 1})
	renderDiffText(surface, 0, 0, 2, "a界", term.StyleDiffAdded, term.StyleDefault, nil, 0, 0, nil)
	if grid[0][0].Ch != 'a' || grid[0][1].Ch != ' ' {
		t.Fatalf("last-column fullwidth rendering = %q / %q, want 'a' then a safe space", grid[0][0].Ch, grid[0][1].Ch)
	}
	if grid[0][1].BgStyle != term.StyleDiffAdded {
		t.Fatalf("substituted last-column cell lost diff background: %+v", grid[0][1])
	}
}
