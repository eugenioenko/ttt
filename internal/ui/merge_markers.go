package ui

import "strings"

// Git's conflict-marker-size attribute changes all four marker widths.
// Requiring the same width and delimiter avoids treating source lines such
// as "======= title" or indented examples as a marker in a visual merge.
func gitMarker(line string, width int) byte {
	if width < 1 || len(line) < width || !strings.ContainsRune("<|=>", rune(line[0])) {
		return 0
	}
	n := 1
	for n < len(line) && line[n] == line[0] {
		n++
	}
	if n != width || (n < len(line) && (line[n] != ' ' || line[0] == '=')) {
		return 0
	}
	return line[0]
}
func gitConflictAt(lines []string, start, width int) (mergeConflict, bool) {
	if start < 0 || start >= len(lines) {
		return mergeConflict{}, false
	}
	kind := gitMarker(lines[start], width)
	if kind != '<' {
		return mergeConflict{}, false
	}
	c := mergeConflict{start: start, separator: -1, base: -1}
	for i := start + 1; i < len(lines); i++ {
		switch gitMarker(lines[i], width) {
		case '<':
			return c, false
		case '|':
			if c.base >= 0 || c.separator >= 0 {
				return c, false
			}
			c.base = i
		case '=':
			if c.separator >= 0 {
				return c, false
			}
			c.separator = i
		case '>':
			if c.separator < 0 {
				return c, false
			}
			c.end = i
			return c, true
		}
	}
	return c, false
}
