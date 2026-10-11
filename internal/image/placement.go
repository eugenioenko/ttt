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

func FitCells(srcW, srcH, boxW, boxH, cellW, cellH int) (w, h int, ok bool) {
	if boxW <= 0 || boxH <= 0 || srcW <= 0 || srcH <= 0 || cellW <= 0 || cellH <= 0 {
		return 0, 0, false
	}
	scale := min(float64(boxW*cellW)/float64(srcW), float64(boxH*cellH)/float64(srcH), 1)
	w = min(max(int(float64(srcW)*scale/float64(cellW)+0.5), 1), boxW)
	h = min(max(int(float64(srcH)*scale/float64(cellH)+0.5), 1), boxH)
	return w, h, true
}
