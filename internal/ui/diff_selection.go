package ui

import (
	"strings"
	"unicode"
)

type diffSelPos struct {
	Line int
	Col  int
}

// diffTextSelection stores positions in logical, unwrapped text. Diff surfaces
// provide their own line lookup so split panes and commit-detail document rows
// can share selection mechanics without coupling their different row models.
type diffTextSelection struct {
	Anchor  diffSelPos
	Current diffSelPos
}

type diffSelectionTextAt func(line int) (text string, selectable bool)

func (s diffTextSelection) Range() (start, end diffSelPos) {
	if s.Anchor.Line < s.Current.Line ||
		(s.Anchor.Line == s.Current.Line && s.Anchor.Col <= s.Current.Col) {
		return s.Anchor, s.Current
	}
	return s.Current, s.Anchor
}

func (s diffTextSelection) Contains(line, col int) bool {
	start, end := s.Range()
	if line < start.Line || line > end.Line {
		return false
	}
	if start.Line == end.Line {
		return col >= start.Col && col < end.Col
	}
	if line == start.Line {
		return col >= start.Col
	}
	if line == end.Line {
		return col < end.Col
	}
	return true
}

func (s diffTextSelection) Text(lineCount int, textAt diffSelectionTextAt) string {
	start, end := s.Range()
	if lineCount <= 0 || end.Line < 0 || start.Line >= lineCount {
		return ""
	}
	firstLine := max(0, start.Line)
	lastLine := min(end.Line, lineCount-1)
	lines := make([]string, 0, lastLine-firstLine+1)
	for line := firstLine; line <= lastLine; line++ {
		text, selectable := textAt(line)
		if !selectable {
			if line != start.Line && line != end.Line {
				lines = append(lines, "")
			}
			continue
		}
		runes := []rune(text)
		startCol, endCol := 0, len(runes)
		if line == start.Line {
			startCol = min(max(0, start.Col), len(runes))
		}
		if line == end.Line {
			endCol = min(max(0, end.Col), len(runes))
		}
		if startCol < endCol {
			lines = append(lines, string(runes[startCol:endCol]))
		} else {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func (s *diffTextSelection) SelectWord(line, col int, text string) bool {
	runes := []rune(text)
	if len(runes) == 0 {
		return false
	}
	col = min(max(0, col), len(runes)-1)
	isWord := func(ch rune) bool {
		return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
	}
	start, end := col, col
	if isWord(runes[col]) {
		for start > 0 && isWord(runes[start-1]) {
			start--
		}
		for end < len(runes)-1 && isWord(runes[end+1]) {
			end++
		}
	}
	s.Anchor = diffSelPos{Line: line, Col: start}
	s.Current = diffSelPos{Line: line, Col: end + 1}
	return true
}
