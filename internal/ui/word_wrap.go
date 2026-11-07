package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/textwidth"
)

// wrapLineSegments returns the starting rune offsets for each visual row of a wrapped line.
// For a line "abcdefgh" with width 3, returns [0, 3, 6].
func wrapLineSegments(runes []rune, width, tabW int) []int {
	if len(runes) == 0 {
		return []int{0}
	}
	segments := []int{0}
	visCol := 0
	for i, ch := range runes {
		var advance int
		if ch == '\t' {
			advance = ((visCol/tabW)+1)*tabW - visCol
		} else {
			advance = textwidth.Rune(ch)
		}
		if visCol+advance > width && visCol > 0 {
			segments = append(segments, i)
			if ch == '\t' {
				advance = tabW
			}
			visCol = advance
		} else {
			visCol += advance
		}
	}
	return segments
}

// wrapLineVisualRows returns len(wrapLineSegments) for line without allocating.
func wrapLineVisualRows(line string, width, tabW int) int {
	// No rune is wider than its UTF-8 encoding, so only a tab can make a line
	// wider than its byte length.
	if len(line) <= width && strings.IndexByte(line, '\t') < 0 {
		return 1
	}
	rows, visCol := 1, 0
	for _, ch := range line {
		advance := 1
		if ch == '\t' {
			advance = ((visCol/tabW)+1)*tabW - visCol
		} else if ch >= utf8.RuneSelf {
			advance = textwidth.Rune(ch)
		}
		if visCol+advance > width && visCol > 0 {
			rows++
			if ch == '\t' {
				advance = tabW
			}
			visCol = advance
		} else {
			visCol += advance
		}
	}
	return rows
}
