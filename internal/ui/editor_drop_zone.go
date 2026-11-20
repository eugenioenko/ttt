package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/widgets"
)

// EditorDropZone identifies where a dragged tab will land on an editor group.
type EditorDropZone int

const (
	EditorDropNone EditorDropZone = iota
	EditorDropCenter
	EditorDropLeft
	EditorDropRight
	EditorDropTop
	EditorDropBottom
)

const editorSplitEdgeRatio = 0.25

func (z EditorDropZone) IsSplit() bool {
	return z == EditorDropLeft || z == EditorDropRight || z == EditorDropTop || z == EditorDropBottom
}

func (g *EditorGroupWidget) SetDropZone(zone EditorDropZone) {
	g.DropZone = zone
	g.DropTarget = zone != EditorDropNone
}

func (g *EditorGroupWidget) effectiveDropZone() EditorDropZone {
	if g.DropZone != EditorDropNone {
		return g.DropZone
	}
	if g.DropTarget {
		return EditorDropCenter
	}
	return EditorDropNone
}

// DropZoneAt resolves the drop zone for screen position (x, y) over this group.
// Hovering another group's tab bar or content center targets EditorDropCenter;
// hovering near an edge of the content area targets a split when canSplit is true.
func (g *EditorGroupWidget) DropZoneAt(x, y int, sameGroup, canSplit bool) EditorDropZone {
	if !g.GetRect().Contains(x, y) {
		return EditorDropNone
	}
	if g.TabBar.GetRect().Contains(x, y) {
		if sameGroup {
			return EditorDropNone
		}
		return EditorDropCenter
	}
	content := g.ContentRect()
	if !content.Contains(x, y) || content.W <= 0 || content.H <= 0 {
		if sameGroup {
			return EditorDropNone
		}
		return EditorDropCenter
	}
	if canSplit {
		rx := (float64(x-content.X) + 0.5) / float64(content.W)
		ry := (float64(y-content.Y) + 0.5) / float64(content.H)

		distX, horiz := rx, EditorDropLeft
		if right := 1.0 - rx; right < distX {
			distX, horiz = right, EditorDropRight
		}
		distY, vert := ry, EditorDropTop
		if bottom := 1.0 - ry; bottom < distY {
			distY, vert = bottom, EditorDropBottom
		}
		if distX < editorSplitEdgeRatio || distY < editorSplitEdgeRatio {
			if distX <= distY {
				return horiz
			}
			return vert
		}
	}
	if sameGroup {
		return EditorDropNone
	}
	return EditorDropCenter
}

// DropPreviewRect returns the local rectangle inside a (w, h) content surface
// that previews where a dropped tab will open.
func DropPreviewRect(w, h int, zone EditorDropZone) Rect {
	if w <= 0 || h <= 0 || zone == EditorDropNone {
		return Rect{}
	}
	switch zone {
	case EditorDropCenter:
		return Rect{X: 0, Y: 0, W: w, H: h}
	case EditorDropLeft:
		return Rect{X: 0, Y: 0, W: max(w/2, 1), H: h}
	case EditorDropRight:
		hw := max(w/2, 1)
		return Rect{X: w - hw, Y: 0, W: hw, H: h}
	case EditorDropTop:
		return Rect{X: 0, Y: 0, W: w, H: max(h/2, 1)}
	case EditorDropBottom:
		hh := max(h/2, 1)
		return Rect{X: 0, Y: h - hh, W: w, H: hh}
	default:
		return Rect{}
	}
}

func (g *EditorGroupWidget) renderDropPreview(surface Surface) {
	zone := g.effectiveDropZone()
	if zone == EditorDropNone {
		return
	}
	w, h := surface.Size()
	pr := DropPreviewRect(w, h, zone)
	if pr.W <= 0 || pr.H <= 0 {
		return
	}

	reader, hasReader := surface.(widgets.CellReader)
	for y := pr.Y; y < pr.Y+pr.H; y++ {
		for x := pr.X; x < pr.X+pr.W; x++ {
			cell := term.Cell{Ch: ' ', Style: term.StyleDefault}
			if hasReader {
				if existing, ok := reader.CellAt(x, y); ok {
					cell = existing
					if cell.Direct {
						cell.Direct = false
						cell.Style = term.StyleDefault
					}
					if cell.Ch == 0 && cell.Text == "" {
						// Keep continuation cells of fullwidth runes empty so we
						// don't corrupt the glyph rendered in column x-1.
						isCont := false
						if x > pr.X {
							if prev, ok := reader.CellAt(x-1, y); ok && prev.Ch != 0 && textwidth.Rune(prev.Ch) > 1 {
								isCont = true
							}
						}
						if !isCont {
							cell.Ch = ' '
						}
					}
				}
			}
			cell.BgStyle = term.StyleHighlightStrong
			surface.SetCell(x, y, cell)
		}
	}

	if hasReader {
		for y := pr.Y; y < pr.Y+pr.H; y++ {
			clearWideGlyphUnder(surface, reader, pr.X, y)
			if pr.W > 1 {
				clearWideGlyphUnder(surface, reader, pr.X+pr.W-1, y)
			}
		}
	}

	b := term.SingleBorderSet()
	if g.Borders != nil {
		b = *g.Borders
	}
	borderCell := func(ch rune) term.Cell {
		return term.Cell{Ch: ch, Style: term.StyleBorderActive, BgStyle: term.StyleHighlightStrong}
	}
	if pr.W == 1 {
		for y := pr.Y; y < pr.Y+pr.H; y++ {
			surface.SetCell(pr.X, y, borderCell(b.Vertical))
		}
		return
	}
	if pr.H == 1 {
		for x := pr.X; x < pr.X+pr.W; x++ {
			surface.SetCell(x, pr.Y, borderCell(b.Horizontal))
		}
		return
	}
	for x := pr.X + 1; x < pr.X+pr.W-1; x++ {
		surface.SetCell(x, pr.Y, borderCell(b.Horizontal))
		surface.SetCell(x, pr.Y+pr.H-1, borderCell(b.Horizontal))
	}
	for y := pr.Y + 1; y < pr.Y+pr.H-1; y++ {
		surface.SetCell(pr.X, y, borderCell(b.Vertical))
		surface.SetCell(pr.X+pr.W-1, y, borderCell(b.Vertical))
	}
	surface.SetCell(pr.X, pr.Y, borderCell(b.TopLeft))
	surface.SetCell(pr.X+pr.W-1, pr.Y, borderCell(b.TopRight))
	surface.SetCell(pr.X, pr.Y+pr.H-1, borderCell(b.BottomLeft))
	surface.SetCell(pr.X+pr.W-1, pr.Y+pr.H-1, borderCell(b.BottomRight))
}

// clearWideGlyphUnder blanks both columns of a fullwidth glyph that covers
// column bx, so the border drawn there does not overlap half of it.
func clearWideGlyphUnder(surface Surface, reader widgets.CellReader, bx, y int) {
	for _, gx := range []int{bx - 1, bx} {
		cell, ok := reader.CellAt(gx, y)
		if !ok || cell.Ch == 0 || textwidth.Rune(cell.Ch) < 2 {
			continue
		}
		cell.Ch = ' '
		cell.Text = ""
		surface.SetCell(gx, y, cell)
		surface.SetCell(gx+1, y, cell)
	}
}
