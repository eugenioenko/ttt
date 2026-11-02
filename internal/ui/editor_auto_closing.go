package ui

import (
	"strings"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/highlight"
)

type autoClosingPair struct {
	open, close    selection.Position
	opener, closer rune
}

type autoClosingState struct {
	buf     *buffer.Buffer
	version uint64
	pairs   []autoClosingPair
	path    string
	syntax  *highlight.Highlighter
}

type autoClosingInsertCommand struct{ undo.InsertStringCommand }

func (c *autoClosingInsertCommand) UndoCursor() (int, int) { return c.Line, c.Col }
func (c *autoClosingInsertCommand) RedoCursor() (int, int) { return c.Line, c.Col + 1 }

func (e *EditorPaneWidget) syncAutoClosing() *autoClosingState {
	if e.autoClosing == nil {
		e.autoClosing = &autoClosingState{}
	}
	s := e.autoClosing
	// Undo, reloads and edits outside this pane must never turn existing text into generated pairs.
	if s.buf != e.Buf || s.version != e.Buf.Version() {
		s.pairs = nil
		s.buf, s.version = e.Buf, e.Buf.Version()
	}
	return s
}

func (e *EditorPaneWidget) autoClosingSyntax() *highlight.Highlighter {
	if e.plainText {
		return nil
	}
	if e.Highlighter != nil {
		return e.Highlighter
	}
	s := e.syncAutoClosing()
	if s.path != e.autoClosingPath || s.syntax == nil {
		s.syntax.Close()
		s.path = e.autoClosingPath
		s.syntax = highlight.NewForLines(s.path, e.Buf.Lines)
	}
	return s.syntax
}

func (e *EditorPaneWidget) autoClosingEnabled(r rune) bool {
	enabled := e.AutoClosing.Brackets
	if r == '\'' || r == '"' {
		enabled = e.AutoClosing.Quotes
	}
	return enabled && (e.AutoClosing.PlainText || e.autoClosingSyntax() != nil)
}

func (e *EditorPaneWidget) autoClosingCloser(line, col int, r rune) rune {
	var close rune
	switch r {
	case '(':
		close = ')'
	case '[':
		close = ']'
	case '{':
		close = '}'
	case '\'', '"':
		close = r
	default:
		return 0
	}
	if !e.autoClosingEnabled(r) {
		return 0
	}
	runes := []rune(e.Buf.Lines[line])
	if col < 0 || col > len(runes) {
		return 0
	}
	if col < len(runes) && !unicode.IsSpace(runes[col]) && !closingBrackets[runes[col]] {
		return 0
	}
	if r == close {
		if autoClosingEscaped(runes, col) {
			return 0
		}
		if col > 0 && (unicode.IsLetter(runes[col-1]) || unicode.IsDigit(runes[col-1]) || runes[col-1] == '_') {
			return 0
		}
	}
	if !e.AutoClosing.InStrings {
		if h := e.autoClosingSyntax(); h != nil {
			inside, known := h.StringAt(e.Buf, line, col)
			if !known || inside {
				return 0
			}
		}
	}
	return close
}

func (e *EditorPaneWidget) rememberAutoClosing(line, col int, open, close rune) {
	s := e.syncAutoClosing()
	s.pairs = append(s.pairs, autoClosingPair{
		open:   selection.Position{Line: line, Col: col},
		close:  selection.Position{Line: line, Col: col + 1},
		opener: open, closer: close,
	})
}

func (e *EditorPaneWidget) autoClosingPairAt(line, col int) int {
	s := e.syncAutoClosing()
	for i := len(s.pairs) - 1; i >= 0; i-- {
		p := s.pairs[i]
		if p.close.Line == line && p.close.Col == col && e.autoClosingEnabled(p.opener) {
			return i
		}
	}
	return -1
}

func (e *EditorPaneWidget) autoClosingSkip(line, col int, r rune) bool {
	if !e.AutoClosing.SkipClosing {
		return false
	}
	if (r == '\'' || r == '"') && autoClosingEscaped([]rune(e.Buf.Lines[line]), col) {
		return false
	}
	i := e.autoClosingPairAt(line, col)
	if i < 0 || e.autoClosing.pairs[i].closer != r {
		return false
	}
	e.autoClosing.pairs = append(e.autoClosing.pairs[:i], e.autoClosing.pairs[i+1:]...)
	if e.Undo != nil {
		e.Undo.BreakGroup()
	}
	return true
}

func (e *EditorPaneWidget) autoClosingDelete(line, col int) bool {
	if !e.AutoClosing.DeletePairs {
		return false
	}
	i := e.autoClosingPairAt(line, col)
	return i >= 0 && e.autoClosing.pairs[i].open == (selection.Position{Line: line, Col: col - 1})
}

func (e *EditorPaneWidget) applyAutoClosingEdit(cmd undo.EditCommand) {
	if batch, ok := cmd.(*undo.BatchCommand); ok {
		for _, child := range batch.Commands {
			e.applyAutoClosingEdit(child)
		}
		return
	}
	if e.autoClosing == nil {
		cmd.Apply(e.Buf)
		return
	}
	s := e.syncAutoClosing()
	if len(s.pairs) != 0 {
		var start, end, inserted selection.Position
		switch c := cmd.(type) {
		case *autoClosingInsertCommand:
			start = selection.Position{Line: c.Line, Col: c.Col}
			end = start
			inserted = selection.Position{Col: 2}
		case *undo.InsertRuneCommand:
			start = selection.Position{Line: c.Line, Col: c.Col}
			end = start
			inserted.Col = 1
		case *undo.InsertStringCommand:
			start = selection.Position{Line: c.Line, Col: c.Col}
			end = start
			inserted.Col = len([]rune(c.Text))
		case *undo.DeleteRuneCommand:
			start = selection.Position{Line: c.Line, Col: c.Col}
			end = selection.Position{Line: c.Line, Col: c.Col + 1}
		case *undo.DeleteSelectionCommand:
			start = selection.Position{Line: c.StartLine, Col: c.StartCol}
			end = selection.Position{Line: c.EndLine, Col: c.EndCol}
		case *undo.SplitLineCommand:
			start = selection.Position{Line: c.Line, Col: c.Col}
			end = start
			inserted.Line = 1
		case *undo.JoinLineCommand:
			if c.Line > 0 && c.Line < len(e.Buf.Lines) {
				start = selection.Position{Line: c.Line - 1, Col: len([]rune(e.Buf.Lines[c.Line-1]))}
				end = selection.Position{Line: c.Line}
			}
		case *undo.PasteCommand:
			start = selection.Position{Line: c.Line, Col: c.Col}
			end = start
			lines := strings.Split(c.Text, "\n")
			inserted = selection.Position{Line: len(lines) - 1, Col: len([]rune(lines[len(lines)-1]))}
		default:
			s.pairs = nil
		}
		kept := s.pairs[:0]
		for _, p := range s.pairs {
			open, openOK := autoClosingMove(p.open, start, end, inserted)
			close, closeOK := autoClosingMove(p.close, start, end, inserted)
			if openOK && closeOK {
				p.open, p.close = open, close
				kept = append(kept, p)
			}
		}
		s.pairs = kept
	}
	cmd.Apply(e.Buf)
	s.version = e.Buf.Version()
}

func autoClosingMove(p, start, end, inserted selection.Position) (selection.Position, bool) {
	before := func(a, b selection.Position) bool { return a.Line < b.Line || (a.Line == b.Line && a.Col < b.Col) }
	if before(p, start) {
		return p, true
	}
	if before(p, end) {
		return p, false
	}
	if p.Line == end.Line {
		p.Col -= end.Col
		p.Col += inserted.Col
		if inserted.Line == 0 {
			p.Col += start.Col
		}
	}
	p.Line += start.Line + inserted.Line - end.Line
	return p, true
}

func autoClosingEscaped(runes []rune, col int) bool {
	slashes := 0
	for i := min(col, len(runes)) - 1; i >= 0 && runes[i] == '\\'; i-- {
		slashes++
	}
	return slashes%2 != 0
}
