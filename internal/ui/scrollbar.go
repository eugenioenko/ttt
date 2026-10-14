package ui

import (
	"math"
	"slices"

	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

type Scrollbar struct {
	X          int // absolute screen X for hit testing
	Y          int // absolute screen Y for hit testing
	Height     int
	TotalItems int
	TopItem    int
	Marks      []ScrollMark
	// Symbols for Legacy Computing glyphs are missing from many fonts.
	LegacyGlyphs bool
	dragging     bool
	dragOffset   int
}

// ScrollMark highlights Count items starting at Item on the track. Its style
// must set the same color as foreground and background: a marked cell is a
// block glyph whose two colors can each come from either side.
type ScrollMark struct {
	Item  int
	Count int
	Style term.Style
	Rank  int // wins over lower ranks sharing a slot
}

func (s *Scrollbar) Visible() bool {
	return s.TotalItems > s.Height && s.Height > 0
}

func (s *Scrollbar) ThumbPos() (top, height int) {
	if s.TotalItems <= s.Height {
		return 0, s.Height
	}
	height = s.Height * s.Height / s.TotalItems
	if height < 1 {
		height = 1
	}
	scrollable := s.TotalItems - s.Height
	top = s.TopItem * (s.Height - height) / scrollable
	if top+height > s.Height {
		top = s.Height - height
	}
	return
}

func (s *Scrollbar) Render(surface Surface, rx, ry int) {
	if !s.Visible() {
		return
	}
	thumbTop, thumbH := s.ThumbPos()
	slots := s.markSlots()
	glyphs := blockGlyphs
	if s.LegacyGlyphs {
		glyphs = legacyGlyphs
	}
	for y := 0; y < s.Height; y++ {
		track := term.StyleScrollbar
		if y >= thumbTop && y < thumbTop+thumbH {
			track = term.StyleScrollbarThumb
		}
		cell := term.Cell{Ch: '█', Style: track}
		if slots != nil {
			cell = markCell(track, slots[y*cellSlots:(y+1)*cellSlots], glyphs)
		}
		surface.SetCell(rx, ry+y, cell)
	}
}

// cellSlots is how many marks fit in one track cell, one per eighth.
const cellSlots = 8

// markSlots returns the mark style for every slot of the track, zero where
// there is none. Every mark keeps at least one slot however many items share
// it, so a single changed line in a huge file stays visible.
func (s *Scrollbar) markSlots() []term.Style {
	if len(s.Marks) == 0 || s.TotalItems <= 0 {
		return nil
	}
	total := cellSlots * s.Height
	slots := make([]term.Style, total)
	ranks := make([]int, total)
	slotOf := func(item int) int {
		return max(0, min(item*total/s.TotalItems, total-1))
	}
	for _, mark := range s.Marks {
		first := slotOf(mark.Item)
		last := slotOf(mark.Item + max(mark.Count, 1) - 1)
		for slot := first; slot <= last; slot++ {
			if slots[slot] == term.StyleDefault || mark.Rank > ranks[slot] {
				slots[slot], ranks[slot] = mark.Style, mark.Rank
			}
		}
	}
	return slots
}

// scrollGlyph paints eighths top through bottom (0 is the top of the cell) in
// its foreground and the rest in its background.
type scrollGlyph struct {
	ch          rune
	top, bottom int
}

func (glyph scrollGlyph) draw(fg, bg term.Style) [cellSlots]term.Style {
	var drawn [cellSlots]term.Style
	for slot := range drawn {
		drawn[slot] = bg
		if slot >= glyph.top && slot <= glyph.bottom {
			drawn[slot] = fg
		}
	}
	return drawn
}

var blockGlyphs = []scrollGlyph{
	{'█', 0, 7},
	{'▀', 0, 3}, {'▄', 4, 7},
	{'▔', 0, 0}, {'▁', 7, 7}, {'▂', 6, 7}, {'▃', 5, 7},
	{'▅', 3, 7}, {'▆', 2, 7}, {'▇', 1, 7},
}

// Symbols for Legacy Computing: thin bars at every eighth and the upper blocks
// that Block Elements lacks.
var legacyGlyphs = append(slices.Clone(blockGlyphs),
	scrollGlyph{'\U0001FB76', 1, 1}, scrollGlyph{'\U0001FB77', 2, 2},
	scrollGlyph{'\U0001FB78', 3, 3}, scrollGlyph{'\U0001FB79', 4, 4},
	scrollGlyph{'\U0001FB7A', 5, 5}, scrollGlyph{'\U0001FB7B', 6, 6},
	scrollGlyph{'\U0001FB82', 0, 1}, scrollGlyph{'\U0001FB83', 0, 2},
	scrollGlyph{'\U0001FB84', 0, 4}, scrollGlyph{'\U0001FB85', 0, 5},
	scrollGlyph{'\U0001FB86', 0, 6},
)

// markCell draws one track cell holding up to cellSlots marks (zero means no
// mark). A cell shows only two colors, so it tries every glyph with every
// color pair and keeps the one that looks closest to the marks.
func markCell(track term.Style, marks []term.Style, glyphs []scrollGlyph) term.Cell {
	var want [cellSlots]term.Style
	colors := []term.Style{track}
	for slot, style := range marks {
		if style == term.StyleDefault {
			style = track
		}
		want[slot] = style
		if !slices.Contains(colors, style) {
			colors = append(colors, style)
		}
	}
	if len(colors) == 1 {
		return term.Cell{Ch: '█', Style: track}
	}

	var best term.Cell
	bestCost := math.MaxInt
	for _, glyph := range glyphs {
		for _, fg := range colors {
			for _, bg := range colors {
				if cost := drawCost(want, glyph.draw(fg, bg), track); cost < bestCost {
					best = term.Cell{Ch: glyph.ch, Style: fg, BgStyle: trackFill(bg)}
					bestCost = cost
				}
			}
		}
	}
	return best
}

// Hiding a mark costs more than painting over some track, and a mark whose
// color is missing from the cell entirely is never worth it.
const (
	costTrackAsMark = 1
	costMarkAsOther = 1
	costMarkAsTrack = 3
	costMarkHidden  = 100
)

func drawCost(want, drawn [cellSlots]term.Style, track term.Style) int {
	cost := 0
	for slot := range want {
		switch {
		case drawn[slot] == want[slot]:
		case want[slot] == track:
			cost += costTrackAsMark
		case drawn[slot] == track:
			cost += costMarkAsTrack
		default:
			cost += costMarkAsOther
		}
		if want[slot] != track && !slices.Contains(drawn[:], want[slot]) {
			cost += costMarkHidden
		}
	}
	return cost
}

// Track styles only set a foreground, so they cannot serve as BgStyle.
func trackFill(style term.Style) term.Style {
	switch style {
	case term.StyleScrollbar:
		return term.StyleScrollbarFill
	case term.StyleScrollbarThumb:
		return term.StyleScrollbarThumbFill
	}
	return style
}

func (s *Scrollbar) HandleEvent(ev tcell.Event) (newTopItem int, consumed bool) {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return s.TopItem, false
	}

	mx, my := mev.Position()
	btn := mev.Buttons()

	if s.dragging {
		if btn == tcell.ButtonNone {
			s.dragging = false
			return s.TopItem, false
		}
		if btn&tcell.Button1 != 0 {
			relY := my - s.Y
			return s.posToTopItem(relY - s.dragOffset), true
		}
	}

	if btn&tcell.Button1 != 0 && mx == s.X && my >= s.Y && my < s.Y+s.Height {
		relY := my - s.Y
		thumbTop, thumbH := s.ThumbPos()

		s.dragging = true
		if relY >= thumbTop && relY < thumbTop+thumbH {
			s.dragOffset = relY - thumbTop
		} else {
			s.dragOffset = thumbH / 2
			return s.posToTopItem(relY - s.dragOffset), true
		}
		return s.TopItem, true
	}

	return s.TopItem, false
}

func (s *Scrollbar) IsDragging() bool { return s.dragging }

func (s *Scrollbar) posToTopItem(thumbTop int) int {
	_, thumbH := s.ThumbPos()
	maxThumbTop := s.Height - thumbH
	if maxThumbTop <= 0 {
		return 0
	}
	if thumbTop < 0 {
		thumbTop = 0
	}
	if thumbTop > maxThumbTop {
		thumbTop = maxThumbTop
	}
	scrollable := s.TotalItems - s.Height
	top := thumbTop * scrollable / maxThumbTop
	if top < 0 {
		top = 0
	}
	if top > scrollable {
		top = scrollable
	}
	return top
}

type HScrollbar struct {
	X          int
	Y          int
	Width      int
	TotalCols  int
	LeftCol    int
	dragging   bool
	dragOffset int
}

func (s *HScrollbar) Visible() bool {
	return s.TotalCols > s.Width && s.Width > 0
}

func (s *HScrollbar) ThumbPos() (left, width int) {
	if s.TotalCols <= s.Width {
		return 0, s.Width
	}
	width = s.Width * s.Width / s.TotalCols
	if width < 1 {
		width = 1
	}
	scrollable := s.TotalCols - s.Width
	left = s.LeftCol * (s.Width - width) / scrollable
	if left+width > s.Width {
		left = s.Width - width
	}
	return
}

func (s *HScrollbar) Render(surface Surface, rx, ry int) {
	if !s.Visible() {
		return
	}
	thumbLeft, thumbW := s.ThumbPos()
	for x := 0; x < s.Width; x++ {
		if x >= thumbLeft && x < thumbLeft+thumbW {
			surface.SetCell(rx+x, ry, term.Cell{Ch: '▄', Style: term.StyleScrollbarThumb})
		} else {
			surface.SetCell(rx+x, ry, term.Cell{Ch: '▄', Style: term.StyleScrollbar})
		}
	}
}

func (s *HScrollbar) HandleEvent(ev tcell.Event) (newLeftCol int, consumed bool) {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return s.LeftCol, false
	}

	mx, my := mev.Position()
	btn := mev.Buttons()

	if s.dragging {
		if btn == tcell.ButtonNone {
			s.dragging = false
			return s.LeftCol, false
		}
		if btn&tcell.Button1 != 0 {
			relX := mx - s.X
			return s.posToLeftCol(relX - s.dragOffset), true
		}
	}

	if btn&tcell.Button1 != 0 && my == s.Y && mx >= s.X && mx < s.X+s.Width {
		relX := mx - s.X
		thumbLeft, thumbW := s.ThumbPos()

		s.dragging = true
		if relX >= thumbLeft && relX < thumbLeft+thumbW {
			s.dragOffset = relX - thumbLeft
		} else {
			s.dragOffset = thumbW / 2
			return s.posToLeftCol(relX - s.dragOffset), true
		}
		return s.LeftCol, true
	}

	return s.LeftCol, false
}

func (s *HScrollbar) IsDragging() bool { return s.dragging }

func (s *HScrollbar) posToLeftCol(thumbLeft int) int {
	_, thumbW := s.ThumbPos()
	maxThumbLeft := s.Width - thumbW
	if maxThumbLeft <= 0 {
		return 0
	}
	if thumbLeft < 0 {
		thumbLeft = 0
	}
	if thumbLeft > maxThumbLeft {
		thumbLeft = maxThumbLeft
	}
	scrollable := s.TotalCols - s.Width
	left := thumbLeft * scrollable / maxThumbLeft
	if left < 0 {
		left = 0
	}
	if left > scrollable {
		left = scrollable
	}
	return left
}
