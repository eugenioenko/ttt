package selection

type Position struct {
	Line int
	Col  int
}

type Selection struct {
	Active bool
	Anchor Position
}

func (s *Selection) Start(line, col int) {
	s.Active = true
	s.Anchor = Position{Line: line, Col: col}
}

func (s *Selection) Clear() {
	s.Active = false
}

func (s *Selection) Range(curLine, curCol int) (start, end Position) {
	a := s.Anchor
	c := Position{Line: curLine, Col: curCol}
	if a.Line < c.Line || (a.Line == c.Line && a.Col <= c.Col) {
		return a, c
	}
	return c, a
}

func (s *Selection) Contains(line, col, curLine, curCol int) bool {
	if !s.Active {
		return false
	}
	start, end := s.Range(curLine, curCol)
	if line < start.Line || line > end.Line {
		return false
	}
	if col < 0 {
		return line != end.Line
	}
	if line == start.Line && col < start.Col {
		return false
	}
	if line == end.Line && col >= end.Col {
		return false
	}
	return true
}

func (s *Selection) Text(lines []string, curLine, curCol int) string {
	if !s.Active {
		return ""
	}
	start, end := s.Range(curLine, curCol)
	if start.Line >= len(lines) || end.Line >= len(lines) || start.Line < 0 || end.Line < 0 {
		return ""
	}
	if start.Line == end.Line {
		runes := []rune(lines[start.Line])
		sc, ec := start.Col, end.Col
		if sc > len(runes) {
			sc = len(runes)
		}
		if ec > len(runes) {
			ec = len(runes)
		}
		return string(runes[sc:ec])
	}
	var result []rune
	// First line: from start.Col to end
	first := []rune(lines[start.Line])
	sc := start.Col
	if sc > len(first) {
		sc = len(first)
	}
	result = append(result, first[sc:]...)
	result = append(result, '\n')
	// Middle lines: full
	for l := start.Line + 1; l < end.Line; l++ {
		result = append(result, []rune(lines[l])...)
		result = append(result, '\n')
	}
	// Last line: from 0 to end.Col
	last := []rune(lines[end.Line])
	ec := end.Col
	if ec > len(last) {
		ec = len(last)
	}
	result = append(result, last[:ec]...)
	return string(result)
}
