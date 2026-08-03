package ui

import (
	"unicode"

	"github.com/eugenioenko/ttt/internal/textwidth"
)

// wordAt returns the maximal run of word characters (letters, digits, or '_')
// surrounding col in lineText, or "" if col is not on a word character.
func wordAt(lineText string, col int) string {
	runes := []rune(lineText)
	if len(runes) == 0 || col < 0 {
		return ""
	}
	if col >= len(runes) {
		col = len(runes) - 1
	}
	isWord := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}
	if !isWord(runes[col]) {
		return ""
	}
	start, end := col, col
	for start > 0 && isWord(runes[start-1]) {
		start--
	}
	for end < len(runes)-1 && isWord(runes[end+1]) {
		end++
	}
	return string(runes[start : end+1])
}

func leadingIndentWidth(line string, tabSize int) int {
	runes := []rune(line)
	if len(runes) > 0 && runes[0] == '\t' {
		return 1
	}
	remove := 0
	for remove < tabSize && remove < len(runes) && runes[remove] == ' ' {
		remove++
	}
	return remove
}

func leadingWhitespace(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}

// bufColToVisualCol converts a rune index into the terminal column it starts
// at, accounting for tab stops and for fullwidth runes that occupy two columns.
func bufColToVisualCol(line string, bufCol, tabW int) int {
	visCol := 0
	ri := 0
	for _, ch := range line {
		if ri >= bufCol {
			break
		}
		if ch == '\t' {
			visCol = ((visCol / tabW) + 1) * tabW
		} else {
			visCol += textwidth.Rune(ch)
		}
		ri++
	}
	return visCol
}

// visualColToBufCol converts a terminal column into a rune index. A column
// inside a multi-column rune (a tab's whitespace or the second half of a
// fullwidth rune) resolves to that rune's index, matching how a click lands on
// the character the user pointed at.
func visualColToBufCol(line string, targetVisCol, tabW int) int {
	visCol := 0
	ri := 0
	for _, ch := range line {
		if visCol >= targetVisCol {
			return ri
		}
		if ch == '\t' {
			nextStop := ((visCol / tabW) + 1) * tabW
			if targetVisCol < nextStop {
				return ri
			}
			visCol = nextStop
		} else {
			w := textwidth.Rune(ch)
			if targetVisCol < visCol+w {
				return ri
			}
			visCol += w
		}
		ri++
	}
	return len([]rune(line))
}

func isEditorIdentRune(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
