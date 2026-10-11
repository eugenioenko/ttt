package ui

import (
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
			visCol = advance
		} else {
			visCol += advance
		}
	}
	return segments
}

// wrapLineVisualRows returns the number of visual rows a line occupies when wrapped.
func wrapLineVisualRows(line string, width, tabW int) int {
	runes := []rune(line)
	return len(wrapLineSegments(runes, width, tabW))
}
