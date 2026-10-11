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
