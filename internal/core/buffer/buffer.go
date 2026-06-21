package buffer

import "time"

type IndentInfo struct {
	UseTabs bool
	Size    int
}

func DetectIndent(lines []string) IndentInfo {
	tabs := 0
	spaces := 0
	diffs := make(map[int]int)
	limit := len(lines)
	if limit > 200 {
		limit = 200
	}
	prevIndent := 0
	for i := 0; i < limit; i++ {
		line := lines[i]
		if len(line) == 0 {
			continue
		}
		indent := 0
		isTabs := false
		for _, r := range line {
			if r == '\t' {
				isTabs = true
				indent++
			} else if r == ' ' {
				indent++
			} else {
				break
			}
		}
		if indent == 0 {
			prevIndent = 0
			continue
		}
		if isTabs {
			tabs++
		} else {
			spaces++
		}
		diff := indent - prevIndent
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 && diff <= 8 {
			diffs[diff]++
		}
		prevIndent = indent
	}
	if tabs > spaces {
		return IndentInfo{UseTabs: true}
	}
	bestSize := 0
	bestCount := 0
	for size, count := range diffs {
		if count > bestCount {
			bestSize = size
			bestCount = count
		}
	}
	if bestSize == 0 {
		return IndentInfo{}
	}
	return IndentInfo{Size: bestSize}
}

// Buffer represents a text buffer with line-based storage.
type Buffer struct {
	Lines                  []string
	Dirty                  bool
	InsertFinalNewline     bool
	TrimTrailingWhitespace bool
	LineEnding             string // "\n" (LF) or "\r\n" (CRLF); defaults to "\n"

	// diskModTime and diskSize record the state of the file on disk the last
	// time we loaded or saved it, so we can detect external modifications
	// before overwriting. diskInfoSet is false for buffers never backed by a
	// file on disk (e.g. a new untitled buffer).
	diskModTime time.Time
	diskSize    int64
	diskInfoSet bool
}

func (b *Buffer) ClampLine(line int) int {
	if line < 0 {
		return 0
	}
	if line >= len(b.Lines) {
		return len(b.Lines) - 1
	}
	return line
}

// InsertRune inserts a rune at the given line and column.
func (b *Buffer) InsertRune(line, col int, r rune) {
	if line < 0 || line >= len(b.Lines) {
		return
	}
	l := b.Lines[line]
	if col < 0 || col > len([]rune(l)) {
		return
	}
	runes := []rune(l)
	runes = append(runes[:col], append([]rune{r}, runes[col:]...)...)
	b.Lines[line] = string(runes)
	b.Dirty = true
}

// DeleteRune deletes a rune at the given line and column.
func (b *Buffer) DeleteRune(line, col int) {
	if line < 0 || line >= len(b.Lines) {
		return
	}
	l := b.Lines[line]
	runes := []rune(l)
	if col < 0 || col >= len(runes) {
		return
	}
	runes = append(runes[:col], runes[col+1:]...)
	b.Lines[line] = string(runes)
	b.Dirty = true
}

// InsertLine inserts a new line at the given index.
func (b *Buffer) InsertLine(idx int, text string) {
	if idx < 0 || idx > len(b.Lines) {
		return
	}
	b.Lines = append(b.Lines[:idx], append([]string{text}, b.Lines[idx:]...)...)
	b.Dirty = true
}

// DeleteLine deletes the line at the given index.
func (b *Buffer) DeleteLine(idx int) {
	if idx < 0 || idx >= len(b.Lines) {
		return
	}
	b.Lines = append(b.Lines[:idx], b.Lines[idx+1:]...)
	b.Dirty = true
}
