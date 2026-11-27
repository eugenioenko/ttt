package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

// A fullwidth rune in a signature label must advance two columns, so the
// following character keeps its own cell instead of being overpainted (#1722).
func TestSignatureHelpFullwidthAdvancesByDisplayWidth(t *testing.T) {
	grid := makeGrid(20, 4)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 4})

	w := NewSignatureHelpWidget("加b", 0, 0)
	w.AnchorX = 0
	w.AnchorY = 3
	w.Render(s)

	// The popup is drawn above the anchor, so its content row is row 1 and the
	// interior starts at column 1. 加 covers columns 1-2, b lands on column 3.
	row := 1
	if grid[row][1].Ch != '加' {
		t.Fatalf("grid[%d][1] = %q, want 加", row, grid[row][1].Ch)
	}
	if grid[row][2].Ch == 'b' {
		t.Errorf("b overpaints the column covered by the fullwidth rune")
	}
	if grid[row][3].Ch != 'b' {
		t.Errorf("grid[%d][3] = %q, want b after the fullwidth rune", row, grid[row][3].Ch)
	}
}

// The active-parameter highlight must map to display columns, so it stays on
// the parameter even when a fullwidth rune precedes it (#1722).
func TestSignatureHelpFullwidthHighlightAlignsToDisplayColumns(t *testing.T) {
	grid := makeGrid(20, 4)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 4})

	// label bytes: 加[0:3] ([3:4] 乙[4:7] )[7:8]; highlight the 乙 parameter.
	w := NewSignatureHelpWidget("加(乙)", 4, 7)
	w.AnchorX = 0
	w.AnchorY = 3
	w.Render(s)

	// Interior starts at column 1: 加 -> 1-2, ( -> 3, 乙 -> 4-5.
	row := 1
	if grid[row][4].Ch != '乙' {
		t.Fatalf("grid[%d][4] = %q, want 乙", row, grid[row][4].Ch)
	}
	if grid[row][4].Style != term.StylePaletteSelected {
		t.Errorf("the active parameter 乙 at column 4 is not highlighted")
	}
	if grid[row][3].Style == term.StylePaletteSelected {
		t.Errorf("the highlight bleeds onto ( at column 3")
	}
}
