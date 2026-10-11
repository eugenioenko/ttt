package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestDiffGutterRightAlignsLineNumbers(t *testing.T) {
	grid := makeGrid(12, 1)
	surface := NewRenderSurface(grid, Rect{W: 12, H: 1})
	o := &DiffOverlay{Signs: true, SignsColor: true, GutterHighlight: true}
	renderDiffGutterWithSigns(surface, 0, 0, 6, 1, o.gutterLook(diff.Added, term.StyleLineNumber, term.StyleDefault), true)
	renderDiffGutterWithSigns(surface, 6, 0, 6, 123, o.gutterLook(diff.Deleted, term.StyleLineNumber, term.StyleDefault), true)
	if got := cellRow(grid, 0); got != "   1 + 123 −" {
		t.Fatalf("aligned diff gutters = %q, want right-aligned line numbers", got)
	}
}

func TestDiffLineDoesNotDrawFullwidthRuneInLastColumn(t *testing.T) {
	e := newRowsEditor([]string{"a界"}, 10, 2)
	cells := e.renderLineToScreen([]rune("a界"), nil, false, nil, 4, 0, 2)
	if cells[0].ch != 'a' || cells[1].ch != ' ' {
		t.Fatalf("last-column fullwidth rendering = %q / %q, want 'a' then a safe space", cells[0].ch, cells[1].ch)
	}
}
