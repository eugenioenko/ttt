package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestHighlightSelectedFallsBackToSwapWithoutThemeSelection(t *testing.T) {
	fg := term.DirectColor{R: 200, G: 200, B: 200, Set: true}
	bg := term.DirectColor{R: 10, G: 10, B: 10, Set: true}
	tw := &TerminalWidget{Palette: &TerminalColorPalette{Fg: fg, Bg: bg}}

	c := term.Cell{Ch: 'a', Direct: true, Fg: fg, Bg: bg}
	tw.highlightSelected(&c)
	if c.Fg != bg || c.Bg != fg || c.Attrs&term.CellAttrReverse != 0 {
		t.Errorf("opaque cell = %+v, want swapped colors and no reverse attribute", c)
	}
}

func TestHighlightSelectedFallsBackToSwapForExplicitBackground(t *testing.T) {
	fg := term.DirectColor{R: 200, G: 200, B: 200, Set: true}
	blue := term.DirectColor{R: 0, G: 0, B: 200, Set: true}
	tw := &TerminalWidget{Palette: &TerminalColorPalette{Fg: fg}}

	c := term.Cell{Ch: 'a', Direct: true, Fg: fg, Bg: blue}
	tw.highlightSelected(&c)
	if c.Fg != blue || c.Bg != fg || c.Attrs&term.CellAttrReverse != 0 {
		t.Errorf("explicit-bg cell = %+v, want swapped colors and no reverse attribute", c)
	}
}

func TestHighlightSelectedUsesThemeSelectionBackground(t *testing.T) {
	fg := term.DirectColor{R: 200, G: 200, B: 200, Set: true}
	sel := term.DirectColor{R: 40, G: 40, B: 40, Set: true}

	for name, palette := range map[string]*TerminalColorPalette{
		"opaque":      {Fg: fg, Bg: term.DirectColor{R: 10, G: 10, B: 10, Set: true}, SelectionBg: sel},
		"transparent": {Fg: fg, SelectionBg: sel},
	} {
		tw := &TerminalWidget{Palette: palette}
		c := term.Cell{Ch: 'a', Direct: true, Fg: fg}
		tw.highlightSelected(&c)
		if c.Bg != sel || c.Fg != fg || c.Attrs&term.CellAttrReverse != 0 {
			t.Errorf("%s: cell = %+v, want the selection background with the text color kept", name, c)
		}
	}
}

func TestHighlightSelectedWithInvertStyleSwapsColors(t *testing.T) {
	fg := term.DirectColor{R: 200, G: 200, B: 200, Set: true}
	bg := term.DirectColor{R: 10, G: 10, B: 10, Set: true}
	sel := term.DirectColor{R: 40, G: 40, B: 40, Set: true}

	t.Run("widget selection style invert", func(t *testing.T) {
		tw := &TerminalWidget{
			Palette:        &TerminalColorPalette{Fg: fg, Bg: bg, SelectionBg: sel},
			SelectionStyle: "invert",
		}
		c := term.Cell{Ch: 'a', Direct: true, Fg: fg, Bg: bg}
		tw.highlightSelected(&c)
		if c.Fg != bg || c.Bg != fg || c.Attrs&term.CellAttrReverse != 0 {
			t.Errorf("cell = %+v, want swapped colors and no reverse attribute", c)
		}
	})

	t.Run("palette selection style invert", func(t *testing.T) {
		tw := &TerminalWidget{
			Palette: &TerminalColorPalette{Fg: fg, Bg: bg, SelectionBg: sel, SelectionStyle: "invert"},
		}
		c := term.Cell{Ch: 'a', Direct: true, Fg: fg, Bg: bg}
		tw.highlightSelected(&c)
		if c.Fg != bg || c.Bg != fg || c.Attrs&term.CellAttrReverse != 0 {
			t.Errorf("cell = %+v, want swapped colors and no reverse attribute", c)
		}
	})

	t.Run("invert with explicit cell background", func(t *testing.T) {
		blue := term.DirectColor{R: 0, G: 0, B: 200, Set: true}
		tw := &TerminalWidget{
			Palette:        &TerminalColorPalette{Fg: fg, Bg: bg, SelectionBg: sel},
			SelectionStyle: "invert",
		}
		c := term.Cell{Ch: 'a', Direct: true, Fg: fg, Bg: blue}
		tw.highlightSelected(&c)
		if c.Fg != blue || c.Bg != fg || c.Attrs&term.CellAttrReverse != 0 {
			t.Errorf("cell = %+v, want swapped colors and no reverse attribute", c)
		}
	})

	t.Run("background style uses theme selection", func(t *testing.T) {
		tw := &TerminalWidget{
			Palette:        &TerminalColorPalette{Fg: fg, Bg: bg, SelectionBg: sel},
			SelectionStyle: "background",
		}
		c := term.Cell{Ch: 'a', Direct: true, Fg: fg, Bg: bg}
		tw.highlightSelected(&c)
		if c.Bg != sel || c.Fg != fg || c.Attrs&term.CellAttrReverse != 0 {
			t.Errorf("cell = %+v, want theme selection background", c)
		}
	})
}
