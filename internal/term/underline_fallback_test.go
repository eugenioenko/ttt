package term

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
)

// A UlStyle whose style defines no underline of its own must still render a
// curly squiggle, coloured by the style's foreground. This is what lets a
// plugin pass an arbitrary named colour style for a diagnostic.
func TestUlStyleForcesCurlyForPlainStyle(t *testing.T) {
	sim := NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatalf("sim init: %v", err)
	}
	sim.SetSize(4, 1)
	ts := NewTcellScreenFrom(sim)

	// Override a style slot with a plain foreground colour (no underline).
	sm := DefaultStyleMap()
	sm[StyleSyntaxKeyword] = tcell.StyleDefault.Foreground(color.XTerm9)
	ts.SetStyleMap(sm)

	ts.SetCell(0, 0, Cell{Ch: 'x', UlStyle: StyleSyntaxKeyword})
	ts.Show()

	cells, _, _ := sim.GetContents()
	got := cells[0].Style
	if us := got.GetUnderlineStyle(); us != tcell.UnderlineStyleCurly {
		t.Errorf("underline style = %v, want curly", us)
	}
	if uc := got.GetUnderlineColor(); uc != color.XTerm9 {
		t.Errorf("underline colour = %v, want red (from foreground)", uc)
	}
}
