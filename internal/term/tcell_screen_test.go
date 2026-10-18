package term

import "testing"

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
