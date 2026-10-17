package ui

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/textwidth"
)

// wrapNotes rewraps NoteTexts into Notes, each under the line it belongs
// to, so that no row is wider than width, when width changed or force is
// set. It reports whether it did.
func (o *DiffOverlay) wrapNotes(width int, force bool) bool {
	if o == nil || (len(o.NoteTexts) == 0 && len(o.Notes) == 0) || (o.noteWidth == width && !force) {
		return false
	}
	o.noteWidth = width
	o.Notes = make(map[int][]NoteRow, len(o.NoteTexts))
	for line, notes := range o.NoteTexts {
		for _, n := range notes {
			o.Notes[line+1] = append(o.Notes[line+1], wrapNoteText(n, width)...)
		}
	}
	return true
}

func wrapNoteText(n NoteText, width int) []NoteRow {
	indent := n.Indent
	if indent > width/2 {
		indent = 0
	}
	avail := max(width-indent-1, 1)
	if n.MaxWidth > 0 {
		avail = min(avail, n.MaxWidth)
	}
	var rows []NoteRow
	add := func(text string) {
		rows = append(rows, NoteRow{Text: " " + text, Indent: indent, Style: n.Style})
	}
	for i, para := range strings.Split(strings.TrimSpace(n.Text), "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			if i > 0 {
				add("")
			}
			continue
		}
		cur, curW := "", 0
		for _, word := range words {
			for _, piece := range splitWide(word, avail) {
				w := textwidth.String(piece)
				switch {
				case curW == 0:
					cur, curW = piece, w
				case curW+1+w > avail:
					add(cur)
					cur, curW = piece, w
				default:
					cur, curW = cur+" "+piece, curW+1+w
				}
			}
		}
		add(cur)
	}
	return rows
}

// splitWide breaks a word wider than width into pieces that fit.
func splitWide(word string, width int) []string {
	if textwidth.String(word) <= width {
		return []string{word}
	}
	var pieces []string
	var cur []rune
	curW := 0
	for _, r := range word {
		w := textwidth.Rune(r)
		if curW+w > width && len(cur) > 0 {
			pieces = append(pieces, string(cur))
			cur, curW = nil, 0
		}
		cur = append(cur, r)
		curW += w
	}
	return append(pieces, string(cur))
}

// leadingIndent is the width in columns of line's leading blanks.
func leadingIndent(line string, tabSize int) int {
	if tabSize <= 0 {
		tabSize = 4
	}
	indent := 0
	for _, r := range line {
		switch r {
		case '\t':
			indent = (indent/tabSize + 1) * tabSize
		case ' ':
			indent++
		default:
			return indent
		}
	}
	return indent
}
