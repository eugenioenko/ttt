package term

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
)

func TestTcellScreen_SetCellWritesRune(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(4, 1)
	ts := NewTcellScreenFrom(sim)
	for _, r := range []rune{'a', ' ', 'é', '中'} {
		ts.SetCell(0, 0, Cell{Ch: r})
		if got, _, _ := sim.Get(0, 0); got != string(r) {
			t.Errorf("SetCell(%q): screen has %q", r, got)
		}
	}
}

func TestTcellScreen_SetCellASCIIDoesNotAllocate(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(4, 1)
	ts := NewTcellScreenFrom(sim)
	cell := Cell{Ch: 'a'}
	ts.SetCell(0, 0, cell)
	if allocs := testing.AllocsPerRun(100, func() { ts.SetCell(0, 0, cell) }); allocs != 0 {
		t.Errorf("SetCell of an ASCII rune allocated %v times, want 0", allocs)
	}
}

func TestTcellScreen_SetCellReversesAStyledCell(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(2, 1)
	ts := NewTcellScreenFrom(sim)
	ts.SetCell(0, 0, Cell{Ch: 'a', Attrs: CellAttrReverse})
	ts.SetCell(1, 0, Cell{Ch: 'b'})
	if _, style, _ := sim.Get(0, 0); !style.HasReverse() {
		t.Error("cell with CellAttrReverse is not reversed")
	}
	if _, style, _ := sim.Get(1, 0); style.HasReverse() {
		t.Error("plain cell is reversed")
	}
}

func TestTcellScreen_SetCellInheritsReverseFromBgStyle(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(2, 1)
	ts := NewTcellScreenFrom(sim)
	styles := DefaultStyleMap()
	styles[StyleSelection] = tcell.StyleDefault.Reverse(true)
	styles[StyleActiveLine] = tcell.StyleDefault.Background(color.Green)
	ts.SetStyleMap(styles)

	ts.SetCell(0, 0, Cell{Ch: 'a', BgStyle: StyleSelection})
	ts.SetCell(1, 0, Cell{Ch: 'b', BgStyle: StyleActiveLine})

	if _, style, _ := sim.Get(0, 0); !style.HasReverse() {
		t.Error("cell with reversed BgStyle is not reversed")
	}
	if _, style, _ := sim.Get(1, 0); style.HasReverse() {
		t.Error("cell with normal BgStyle is reversed")
	}
}

func TestTcellScreen_HideUsesTheRenderedBackground(t *testing.T) {
	for _, tc := range []struct {
		name string
		cell Cell
		want tcell.Color
	}{
		{"plain", Cell{Ch: 'a', Attrs: CellAttrHide}, color.Blue},
		{"layered", Cell{Ch: 'a', BgStyle: StyleActiveLine, Attrs: CellAttrHide}, color.Green},
		{"reverse", Cell{Ch: 'a', Attrs: CellAttrHide | CellAttrReverse}, color.Red},
		{"named style", Cell{Ch: 'a', Style: StyleHide}, color.Blue},
		{"wide", Cell{Ch: '界', Attrs: CellAttrHide}, color.Blue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sim := NewSimScreen()
			sim.SetSize(4, 1)
			ts := NewTcellScreenFrom(sim)
			styles := DefaultStyleMap()
			styles[StyleDefault] = tcell.StyleDefault.Foreground(color.Red).Background(color.Blue)
			styles[StyleHide] = styles[StyleDefault]
			styles[StyleActiveLine] = tcell.StyleDefault.Background(color.Green)
			ts.SetStyleMap(styles)
			cell := tc.cell
			cell.Underline, cell.Bold = true, true
			cell.UlStyle = StyleDiagError
			ts.SetCell(0, 0, cell)
			text, style, _ := sim.Get(0, 0)
			if text != string(cell.Ch) {
				t.Fatalf("text = %q, want %q", text, cell.Ch)
			}
			if style.GetForeground() != tc.want || style.GetBackground() != tc.want {
				t.Fatalf("foreground/background = %v/%v, want %v", style.GetForeground(), style.GetBackground(), tc.want)
			}
			if style.GetAttributes() != 0 || style.GetUnderlineStyle() != tcell.UnderlineStyleNone {
				t.Fatal("hidden text retained visible attributes")
			}
		})
	}
}

func TestTcellScreen_HideWithDefaultBackgroundClearsWideCharacters(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(4, 1)
	ts := NewTcellScreenFrom(sim)
	ts.SetCell(0, 0, Cell{Ch: '界'})
	ts.SetCell(2, 0, Cell{Ch: 'x'})
	ts.SetCell(0, 0, Cell{Ch: '界', Attrs: CellAttrHide})
	for x := range 2 {
		if text, style, _ := sim.Get(x, 0); text != " " || style.GetBackground() != tcell.ColorDefault {
			t.Errorf("column %d = %q with background %v, want a default-background space", x, text, style.GetBackground())
		}
	}
	if text, _, _ := sim.Get(2, 0); text != "x" {
		t.Fatal("hiding a wide character changed its neighbor")
	}
	ts.SetCell(0, 0, Cell{Ch: '界'})
	if text, _, _ := sim.Get(0, 0); text != "界" {
		t.Fatal("clearing concealment did not restore the character")
	}
}

func TestTcellScreen_HideReversedDefaultForegroundKeepsBackground(t *testing.T) {
	for _, fromStyle := range []bool{false, true} {
		name := "cell attribute"
		if fromStyle {
			name = "theme style"
		}
		t.Run(name, func(t *testing.T) {
			sim := NewSimScreen()
			sim.SetSize(2, 1)
			ts := NewTcellScreenFrom(sim)
			styles := DefaultStyleMap()
			styles[StyleDefault] = tcell.StyleDefault.Background(color.Blue).Reverse(fromStyle)
			ts.SetStyleMap(styles)
			cell := Cell{Ch: '界', Attrs: CellAttrHide, Bold: true, Underline: true}
			if !fromStyle {
				cell.Attrs |= CellAttrReverse
			}
			ts.SetCell(0, 0, cell)
			for x := range 2 {
				text, style, _ := sim.Get(x, 0)
				if text != " " || !style.HasReverse() || style.GetForeground() != tcell.ColorDefault {
					t.Errorf("column %d: concealment lost the terminal-default foreground as its displayed background", x)
				}
				if style.HasBold() || style.GetUnderlineStyle() != tcell.UnderlineStyleNone {
					t.Errorf("column %d: fallback space retained visible attributes", x)
				}
			}
		})
	}
}
