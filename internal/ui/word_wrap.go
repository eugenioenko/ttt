package ui

import (
	"github.com/eugenioenko/ttt/internal/textwidth"
)

type wrapEntry struct {
	bufLine  int
	startCol int
}

// buildWrapMap computes a screen-row to (bufferLine, startCol) mapping for word wrap.
// topLine is the first buffer line; wrapOffset is the visual row offset within that line
// (0 means start from the beginning of the line). Returns exactly screenHeight entries.
func buildWrapMap(lines []string, topLine, wrapOffset, screenHeight, width, tabW int) []wrapEntry {
	if width < 1 {
		width = 1
	}
	result := make([]wrapEntry, 0, screenHeight)
	bufLine := topLine

	for len(result) < screenHeight {
		if bufLine >= len(lines) {
			result = append(result, wrapEntry{bufLine: bufLine, startCol: 0})
			bufLine++
			continue
		}

		runes := []rune(lines[bufLine])
		segments := wrapLineSegments(runes, width, tabW)

		startSeg := 0
		if bufLine == topLine {
			startSeg = wrapOffset
			if startSeg >= len(segments) {
				startSeg = 0
			}
		}

		for i := startSeg; i < len(segments) && len(result) < screenHeight; i++ {
			result = append(result, wrapEntry{bufLine: bufLine, startCol: segments[i]})
		}

		bufLine++
	}

	return result
}

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

// totalVisualLines returns the total number of visual rows for all buffer lines when wrapped.
func totalVisualLines(lines []string, width, tabW int) int {
	total := 0
	for _, line := range lines {
		total += wrapLineVisualRows(line, width, tabW)
	}
	return total
}

// bufferPosToWrapScreenPos converts a buffer (line, col) to a screen position
// relative to the wrap layout. Returns (visualRow, screenCol) where visualRow
// is the absolute visual row from the top of the document.
func bufferPosToWrapScreenPos(lines []string, line, col, width, tabW int) (visualRow, screenCol int) {
	if width < 1 {
		width = 1
	}
	for i := 0; i < line && i < len(lines); i++ {
		visualRow += wrapLineVisualRows(lines[i], width, tabW)
	}

	if line >= len(lines) {
		return visualRow, 0
	}

	runes := []rune(lines[line])
	segments := wrapLineSegments(runes, width, tabW)

	segIdx := 0
	for i := len(segments) - 1; i >= 0; i-- {
		if col >= segments[i] {
			segIdx = i
			break
		}
	}

	visualRow += segIdx
	segStart := segments[segIdx]

	screenCol = 0
	for i := segStart; i < col && i < len(runes); i++ {
		if runes[i] == '\t' {
			screenCol = ((screenCol / tabW) + 1) * tabW
		} else {
			screenCol += textwidth.Rune(runes[i])
		}
	}

	return visualRow, screenCol
}

// wrapVisualRowToTopLine finds the buffer line and wrap offset for a given
// absolute visual row.
func wrapVisualRowToTopLine(lines []string, targetVisRow, width, tabW int) (bufLine, wrapOffset int) {
	if targetVisRow <= 0 {
		return 0, 0
	}
	visRow := 0
	for i, line := range lines {
		rows := wrapLineVisualRows(line, width, tabW)
		if visRow+rows > targetVisRow {
			return i, targetVisRow - visRow
		}
		visRow += rows
	}
	if len(lines) == 0 {
		return 0, 0
	}
	return len(lines) - 1, 0
}
