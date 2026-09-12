package image

import "image"

type Rect struct {
	X, Y, W, H int
}

type Placement struct {
	SourceID    uint64
	PlacementID uint32
	Cell        Rect
	Src         image.Rectangle
	Z           int
}

func Equal(a, b Placement) bool {
	return a.SourceID == b.SourceID &&
		a.Cell == b.Cell &&
		a.Src == b.Src &&
		a.Z == b.Z
}
