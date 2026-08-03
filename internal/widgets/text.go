package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

// drawTextWidthAware draws text starting at x, advancing by each rune's display
// width. maxW is an absolute column limit (0 means "the surface edge"). A
// fullwidth rune that would straddle the limit is replaced by a space, because
// the terminal paints such a rune across two columns regardless of clipping.
func drawTextWidthAware(surface Surface, x, y int, text string, maxW, surfaceW int, style term.Style) int {
	limit := surfaceW
	if maxW > 0 && maxW < limit {
		limit = maxW
	}
	for _, ch := range text {
		if x >= limit {
			break
		}
		w := textwidth.Rune(ch)
		if x+w > limit {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
			x++
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
		x += w
	}
	return x
}

// drawRunesClipped draws runes from x up to (but not including) maxX, advancing
// by display width. When the runes do not fit, the last column that would be
// drawn becomes an ellipsis. Returns the column after the last one written.
func drawRunesClipped(surface Surface, x, y, maxX int, runes []rune, style term.Style) int {
	fits := x+textwidth.Runes(runes) <= maxX
	for i, ch := range runes {
		w := textwidth.Rune(ch)
		if x+w > maxX {
			break
		}
		if !fits && x+w >= maxX && i < len(runes)-1 {
			surface.SetCell(x, y, term.Cell{Ch: '…', Style: style})
			return x + 1
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
		x += w
	}
	return x
}

// truncateRunesLeft keeps the tail of runes that fits in avail columns,
// prefixed with an ellipsis. Used where the end of a label (a filename, a path
// segment) matters more than its beginning.
func truncateRunesLeft(runes []rune, avail int) []rune {
	if avail <= 0 || textwidth.Runes(runes) <= avail {
		return runes
	}
	tailW := 0
	i := len(runes)
	for i > 0 {
		w := textwidth.Rune(runes[i-1])
		if tailW+w > avail-1 {
			break
		}
		tailW += w
		i--
	}
	return append([]rune{'…'}, runes[i:]...)
}
