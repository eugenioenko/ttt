package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/textwidth"
)

// wrapLineSegments returns the starting rune offsets for each visual row of a wrapped line.
// For a line "abcdefgh" with width 3, returns [0, 3, 6].
func wrapLineSegments(line string, width, tabW int, words bool) []int {
	return wrapLineSegmentsWithGhost(line, width, tabW, words, ghostText{})
}

func wrapLineSegmentsWithGhost(line string, width, tabW int, words bool, ghost ghostText) []int {
	segments := []int{0}
	walkWrap(line, width, tabW, words, ghost, func(col int) { segments = append(segments, col) })
	return segments
}

// wrapLineVisualRows returns len(wrapLineSegments) for line without allocating.
func wrapLineVisualRows(line string, width, tabW int, words bool) int {
	return wrapLineVisualRowsWithGhost(line, width, tabW, words, ghostText{})
}

func wrapLineVisualRowsWithGhost(line string, width, tabW int, words bool, ghost ghostText) int {
	// No rune is wider than its UTF-8 encoding, so only a tab can make a line
	// wider than its byte length.
	if len(line)+ghost.width() <= width && strings.IndexByte(line, '\t') < 0 {
		return 1
	}
	rows := 1
	walkWrap(line, width, tabW, words, ghost, func(int) { rows++ })
	return rows
}

func cappedGhostWidth(text string, maxW int) int {
	if maxW <= 0 || text == "" {
		return 0
	}
	return textwidth.String(textwidth.Truncate(text, maxW, ""))
}

// walkWrap calls next with the rune offset of each wrapped row after the
// first.
func walkWrap(line string, width, tabW int, words bool, ghost ghostText, next func(col int)) {
	if words {
		walkWrapWords(line, width, tabW, ghost, next)
		return
	}
	ghostW := ghost.width()
	visCol, shift, col := 0, 0, 0
	for _, ch := range line {
		advance := 1
		if ch == '\t' || ch >= utf8.RuneSelf {
			advance = wrapAdvance(ch, visCol, tabW)
		}
		if ghostW > 0 && col == ghost.col {
			gw := ghostW
			if visCol+gw+advance > width && visCol > 0 {
				next(col)
				visCol = 0
				if ch == '\t' {
					advance = tabW
				}
			}
			if visCol+gw+advance > width {
				gw = cappedGhostWidth(ghost.text, max(width-visCol-advance, 0))
			}
			shift = gw
		} else if visCol+shift+advance > width && visCol > 0 {
			next(col)
			visCol = 0
			shift = 0
			if ch == '\t' {
				advance = tabW
			}
		}
		visCol += advance
		col++
	}
}

// walkWrapWords ends a row at the last break opportunity that fits. A space
// that just overflows hangs past the edge instead of starting the next row,
// and a run with no opportunity breaks at the character that overflows.
func walkWrapWords(line string, width, tabW int, ghost ghostText, next func(col int)) {
	ghostW := ghost.width()
	start, rowStartCol, visCol, shift, col := 0, 0, 0, 0, 0
	brk, brkCol := 0, 0
	var prev rune
	for i, ch := range line {
		if i > start {
			p, c := uint32(prev), uint32(ch)
			if p < utf8.RuneSelf && c < utf8.RuneSelf {
				if (breakAfter[p] && !breakAfter[c]) || (breakBefore[c] && !breakBefore[p]) {
					brk, brkCol = i, col
				}
			} else if wordBreakBetween(prev, ch) {
				brk, brkCol = i, col
			}
		}
		prev = ch
		advance := 1
		if ch == '\t' || ch >= utf8.RuneSelf {
			advance = wrapAdvance(ch, visCol, tabW)
		}
		if ghostW > 0 && col == ghost.col {
			gw := ghostW
			if visCol+gw+advance > width && visCol > 0 {
				if brk > start {
					start = brk
					rowStartCol = brkCol
					next(brkCol)
					visCol = wrapTextWidth(line[start:i], tabW)
					advance = wrapAdvance(ch, visCol, tabW)
				}
				if visCol+gw+advance > width && visCol > 0 {
					start = i
					rowStartCol = col
					next(col)
					visCol = 0
					advance = wrapAdvance(ch, 0, tabW)
				}
			}
			if visCol+gw+advance > width {
				gw = cappedGhostWidth(ghost.text, max(width-visCol-advance, 0))
			}
			shift = gw
		} else if visCol+shift+advance > width && visCol > 0 && (ch != ' ' || visCol+shift != width) {
			if brk > start {
				start = brk
				rowStartCol = brkCol
				next(brkCol)
				visCol = wrapTextWidth(line[start:i], tabW)
				advance = wrapAdvance(ch, visCol, tabW)
				if ghostW > 0 && ghost.col >= rowStartCol && ghost.col < col {
					shift = ghostW
				} else {
					shift = 0
				}
			}
			if visCol+shift+advance > width && visCol > 0 {
				start = i
				rowStartCol = col
				next(col)
				visCol = 0
				shift = 0
				advance = wrapAdvance(ch, 0, tabW)
			}
		}
		visCol += advance
		col++
	}
}

func wrapAdvance(ch rune, visCol, tabW int) int {
	switch {
	case ch == '\t':
		return ((visCol/tabW)+1)*tabW - visCol
	case ch < utf8.RuneSelf:
		return 1
	default:
		return textwidth.Rune(ch)
	}
}

func wrapTextWidth(text string, tabW int) int {
	w := 0
	for _, ch := range text {
		w += wrapAdvance(ch, w, tabW)
	}
	return w
}

var breakAfter, breakBefore [utf8.RuneSelf]bool

func init() {
	for _, ch := range " \t})]?|/&.,;" {
		breakAfter[ch] = true
	}
	for _, ch := range "([{" {
		breakBefore[ch] = true
	}
}

// wordBreakBetween allows a break at the end of a run of break-after
// characters or the start of a run of break-before ones, so "))" and "(("
// stay together, and on either side of a wide rune unless that splits it from
// adjacent punctuation.
func wordBreakBetween(prev, next rune) bool {
	prevAfter, nextAfter := isBreakAfter(prev), isBreakAfter(next)
	prevBefore, nextBefore := isBreakBefore(prev), isBreakBefore(next)
	return (prevAfter && !nextAfter) || (nextBefore && !prevBefore) ||
		(isWideRune(prev) && !nextAfter) || (isWideRune(next) && !prevBefore)
}

func isBreakAfter(ch rune) bool  { return ch >= 0 && ch < utf8.RuneSelf && breakAfter[ch] }
func isBreakBefore(ch rune) bool { return ch >= 0 && ch < utf8.RuneSelf && breakBefore[ch] }

func isWideRune(ch rune) bool {
	return ch >= utf8.RuneSelf && textwidth.Rune(ch) > 1
}
