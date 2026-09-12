package image

import (
	"image"
	"testing"
)

func TestEqual(t *testing.T) {
	base := Placement{
		SourceID:    7,
		PlacementID: 1,
		Cell:        Rect{X: 2, Y: 3, W: 4, H: 5},
		Src:         image.Rect(0, 0, 10, 10),
		Z:           -1,
	}
	same := base
	same.PlacementID = 99
	if !Equal(base, same) {
		t.Error("placements differing only in PlacementID should be equal")
	}

	for _, tc := range []struct {
		name   string
		mutate func(*Placement)
	}{
		{"source", func(p *Placement) { p.SourceID++ }},
		{"cell", func(p *Placement) { p.Cell.X++ }},
		{"src", func(p *Placement) { p.Src.Max.X++ }},
		{"z", func(p *Placement) { p.Z++ }},
	} {
		other := base
		tc.mutate(&other)
		if Equal(base, other) {
			t.Errorf("%s: expected placements to differ", tc.name)
		}
	}
}
