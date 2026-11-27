package lsp

// Positions on the wire use UTF-16: ttt does not advertise another encoding.
// Editor columns count runes, independently of bytes and terminal cells.
func RuneToUTF16(text string, col int) int {
	offset := 0
	for _, r := range text {
		if col <= 0 {
			break
		}
		offset++
		if r > 0xffff {
			offset++
		}
		col--
	}
	return offset
}

func UTF16ToRune(text string, offset int) int {
	col := 0
	for _, r := range text {
		width := 1
		if r > 0xffff {
			width = 2
		}
		if offset < width {
			break
		}
		offset -= width
		col++
	}
	return col
}

func positionText(lines []string, line int) string {
	if line < 0 || line >= len(lines) {
		return ""
	}
	return lines[line]
}

func PositionToUTF16(lines []string, p Position) Position {
	p.Character = RuneToUTF16(positionText(lines, p.Line), p.Character)
	return p
}

func PositionToRunes(lines []string, p Position) Position {
	p.Character = UTF16ToRune(positionText(lines, p.Line), p.Character)
	return p
}

func RangeToRunes(lines []string, r Range) Range {
	return Range{Start: PositionToRunes(lines, r.Start), End: PositionToRunes(lines, r.End)}
}
