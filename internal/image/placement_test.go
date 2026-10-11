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

func TestFitCells(t *testing.T) {
	cases := []struct {
		name                                 string
		srcW, srcH, boxW, boxH, cellW, cellH int
		wantW, wantH                         int
		wantOK                               bool
	}{
		{"downscale wide", 1600, 1200, 40, 20, 10, 20, 40, 15, true},
		{"downscale tall", 1000, 4000, 40, 20, 10, 20, 10, 20, true},
		{"never upscale", 4, 4, 60, 20, 10, 20, 1, 1, true},
		{"unknown cell size", 100, 100, 10, 10, 0, 0, 0, 0, false},
		{"empty box", 100, 100, 0, 10, 10, 20, 0, 0, false},
	}
	for _, tc := range cases {
		w, h, ok := FitCells(tc.srcW, tc.srcH, tc.boxW, tc.boxH, tc.cellW, tc.cellH)
		if w != tc.wantW || h != tc.wantH || ok != tc.wantOK {
			t.Errorf("%s: got %dx%d %v, want %dx%d %v", tc.name, w, h, ok, tc.wantW, tc.wantH, tc.wantOK)
		}
	}
}
