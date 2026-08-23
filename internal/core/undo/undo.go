package undo

import (
	"strings"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/buffer"
)

// EditCommand defines the interface for undoable buffer edits.
type EditCommand interface {
	Apply(b *buffer.Buffer)
	Undo(b *buffer.Buffer)
}

// UndoStack manages undo and redo stacks with automatic grouping of
// consecutive character inserts and deletes.
type UndoStack struct {
	undo           []EditCommand
	redo           []EditCommand
	grouping       bool
	savePoint      int
	inTransaction  bool
	transactionIdx int
	// DeleteCursorStart controls where the cursor lands after undoing a
	// DeleteSelectionCommand. When true, cursor goes to the start of the
	// restored text (Emacs convention); when false, to the end (default).
	DeleteCursorStart bool
}

func (s *UndoStack) BeginTransaction() {
	s.grouping = false
	s.inTransaction = true
	s.transactionIdx = len(s.undo)
}

func (s *UndoStack) EndTransaction() {
	if !s.inTransaction {
		return
	}
	idx := s.transactionIdx
	s.inTransaction = false
	s.grouping = false

	if idx >= len(s.undo) {
		return
	}

	cmds := make([]EditCommand, len(s.undo)-idx)
	copy(cmds, s.undo[idx:])
	s.undo = s.undo[:idx]
	s.undo = append(s.undo, &BatchCommand{Commands: cmds})
}

func (s *UndoStack) MarkSaved() {
	s.grouping = false
	s.savePoint = len(s.undo)
}

func (s *UndoStack) AtSavePoint() bool {
	return len(s.undo) == s.savePoint
}

// Push adds a command to the undo stack and clears the redo stack.
// Consecutive InsertRuneCommand or DeleteRuneCommand at adjacent positions
// are automatically grouped so a single undo reverses the whole sequence.
func (s *UndoStack) Push(cmd EditCommand) {
	s.redo = nil
	if s.grouping {
		if len(s.undo) > 0 {
			if grp, ok := s.undo[len(s.undo)-1].(*BatchCommand); ok {
				if canGroup(grp, cmd) {
					grp.Commands = append(grp.Commands, cmd)
					return
				}
			}
		}
		s.grouping = false
	}

	switch cmd.(type) {
	case *InsertRuneCommand, *DeleteRuneCommand:
		grp := &BatchCommand{Commands: []EditCommand{cmd}}
		s.undo = append(s.undo, grp)
		s.grouping = true
	default:
		s.undo = append(s.undo, cmd)
	}
}

// BreakGroup ends the current undo group so the next Push starts a new one.
func (s *UndoStack) BreakGroup() {
	s.grouping = false
}

// ContinueGroup allows subsequent rune inserts to be appended to the
// current top-of-stack entry. Used after replaceSelection so that typing
// over a selection groups the replacement with continued typing.
func (s *UndoStack) ContinueGroup() {
	s.grouping = true
}

func canGroup(grp *BatchCommand, cmd EditCommand) bool {
	if len(grp.Commands) == 0 {
		return false
	}
	last := grp.Commands[len(grp.Commands)-1]
	switch lc := last.(type) {
	case *InsertRuneCommand:
		if ic, ok := cmd.(*InsertRuneCommand); ok {
			if ic.Line != lc.Line || ic.Col != lc.Col+1 {
				return false
			}
			// Space/tab after non-space starts a new group (space belongs with the next word)
			if (ic.Rune == ' ' || ic.Rune == '\t') && lc.Rune != ' ' && lc.Rune != '\t' {
				return false
			}
			return true
		}
	case *DeleteRuneCommand:
		if dc, ok := cmd.(*DeleteRuneCommand); ok {
			return dc.Line == lc.Line && (dc.Col == lc.Col-1 || dc.Col == lc.Col)
		}
	}
	return false
}

// CursorPos represents a cursor position returned by Undo/Redo.
type CursorPos struct {
	Line, Col int
}

// Undo undoes the last command and returns where the cursor should be placed.
func (s *UndoStack) Undo(b *buffer.Buffer) *CursorPos {
	if len(s.undo) == 0 {
		return nil
	}
	s.grouping = false
	cmd := s.undo[len(s.undo)-1]
	s.undo = s.undo[:len(s.undo)-1]
	cmd.Undo(b)
	s.redo = append(s.redo, cmd)
	return s.cursorAfterUndo(cmd)
}

// Redo re-applies the last undone command and returns where the cursor should be placed.
func (s *UndoStack) Redo(b *buffer.Buffer) *CursorPos {
	if len(s.redo) == 0 {
		return nil
	}
	s.grouping = false
	cmd := s.redo[len(s.redo)-1]
	s.redo = s.redo[:len(s.redo)-1]
	cmd.Apply(b)
	s.undo = append(s.undo, cmd)
	return cursorAfterRedo(cmd)
}

func (s *UndoStack) cursorAfterUndo(cmd EditCommand) *CursorPos {
	switch c := cmd.(type) {
	case *InsertRuneCommand:
		return &CursorPos{c.Line, c.Col}
	case *DeleteRuneCommand:
		return &CursorPos{c.Line, c.Col + 1}
	case *InsertStringCommand:
		return &CursorPos{c.Line, c.Col}
	case *SplitLineCommand:
		return &CursorPos{c.Line, c.Col}
	case *JoinLineCommand:
		return &CursorPos{c.Line - 1, c.PrevLen}
	case *JoinNextLineCommand:
		return &CursorPos{c.Line, c.JoinCol}
	case *DeleteSelectionCommand:
		if s.DeleteCursorStart {
			return &CursorPos{c.StartLine, c.StartCol}
		}
		return &CursorPos{c.EndLine, c.EndCol}
	case *PasteCommand:
		return &CursorPos{c.Line, c.Col}
	case *BatchCommand:
		if len(c.Commands) > 0 {
			return s.cursorAfterUndo(c.Commands[0])
		}
	}
	return nil
}

func cursorAfterRedo(cmd EditCommand) *CursorPos {
	switch c := cmd.(type) {
	case *InsertRuneCommand:
		return &CursorPos{c.Line, c.Col + 1}
	case *DeleteRuneCommand:
		return &CursorPos{c.Line, c.Col}
	case *InsertStringCommand:
		return &CursorPos{c.Line, c.Col + len([]rune(c.Text))}
	case *SplitLineCommand:
		return &CursorPos{c.Line + 1, 0}
	case *JoinLineCommand:
		return &CursorPos{c.Line - 1, c.PrevLen}
	case *JoinNextLineCommand:
		return &CursorPos{c.Line, c.JoinCol}
	case *DeleteSelectionCommand:
		return &CursorPos{c.StartLine, c.StartCol}
	case *PasteCommand:
		lines := splitLines(c.Text)
		if len(lines) == 1 {
			return &CursorPos{c.Line, c.Col + len([]rune(lines[0]))}
		}
		return &CursorPos{c.Line + len(lines) - 1, len([]rune(lines[len(lines)-1]))}
	case *BatchCommand:
		if len(c.Commands) > 0 {
			return cursorAfterRedo(c.Commands[len(c.Commands)-1])
		}
	}
	return nil
}

// InsertRuneCommand implements EditCommand for inserting a rune.
type InsertRuneCommand struct {
	Line, Col int
	Rune      rune
}

func (c *InsertRuneCommand) Apply(b *buffer.Buffer) {
	b.InsertRune(c.Line, c.Col, c.Rune)
}

func (c *InsertRuneCommand) Undo(b *buffer.Buffer) {
	b.DeleteRune(c.Line, c.Col)
}

// DeleteRuneCommand implements EditCommand for deleting a rune.
type DeleteRuneCommand struct {
	Line, Col int
	Rune      rune
}

func (c *DeleteRuneCommand) Apply(b *buffer.Buffer) {
	if c.Line < 0 || c.Line >= len(b.Lines) {
		return
	}
	runes := []rune(b.Lines[c.Line])
	if c.Col < 0 || c.Col >= len(runes) {
		return
	}
	c.Rune = runes[c.Col]
	b.DeleteRune(c.Line, c.Col)
}

func (c *DeleteRuneCommand) Undo(b *buffer.Buffer) {
	b.InsertRune(c.Line, c.Col, c.Rune)
}

// SplitLineCommand implements EditCommand for splitting a line (Enter key).
type SplitLineCommand struct {
	Line, Col int
}

func (c *SplitLineCommand) Apply(b *buffer.Buffer) {
	if c.Line < 0 || c.Line >= len(b.Lines) {
		return
	}
	line := []rune(b.Lines[c.Line])
	col := c.Col
	if col > len(line) {
		col = len(line)
	}
	left := string(line[:col])
	right := string(line[col:])
	b.Lines[c.Line] = left
	b.InsertLine(c.Line+1, right)
}

func (c *SplitLineCommand) Undo(b *buffer.Buffer) {
	if c.Line+1 >= len(b.Lines) {
		return
	}
	b.Lines[c.Line] += b.Lines[c.Line+1]
	b.DeleteLine(c.Line + 1)
	b.Dirty = true
}

// JoinLineCommand implements EditCommand for joining a line with the previous one (Backspace at col 0).
type JoinLineCommand struct {
	Line    int
	PrevLen int
}

func (c *JoinLineCommand) Apply(b *buffer.Buffer) {
	if c.Line <= 0 || c.Line >= len(b.Lines) {
		return
	}
	c.PrevLen = len([]rune(b.Lines[c.Line-1]))
	b.Lines[c.Line-1] += b.Lines[c.Line]
	b.DeleteLine(c.Line)
	b.Dirty = true
}

func (c *JoinLineCommand) Undo(b *buffer.Buffer) {
	if c.Line-1 < 0 || c.Line-1 >= len(b.Lines) {
		return
	}
	combined := []rune(b.Lines[c.Line-1])
	left := string(combined[:c.PrevLen])
	right := string(combined[c.PrevLen:])
	b.Lines[c.Line-1] = left
	b.InsertLine(c.Line, right)
}

// JoinNextLineCommand implements EditCommand for joining the current line with the next one.
// It trims leading whitespace from the next line and joins with a single space
// (unless the current line ends with a space or the trimmed next line is empty).
type JoinNextLineCommand struct {
	Line     int    // the current line (joins with Line+1)
	JoinCol  int    // column where the join happened (set by Apply)
	NextText string // original text of the next line (for undo)
}

func (c *JoinNextLineCommand) Apply(b *buffer.Buffer) {
	if c.Line < 0 || c.Line >= len(b.Lines)-1 {
		return
	}
	currentLine := b.Lines[c.Line]
	nextLine := b.Lines[c.Line+1]
	c.NextText = nextLine

	trimmed := strings.TrimLeftFunc(nextLine, unicode.IsSpace)
	currentRunes := []rune(currentLine)
	c.JoinCol = len(currentRunes)

	separator := ""
	if len(currentRunes) > 0 && trimmed != "" && currentRunes[len(currentRunes)-1] != ' ' {
		separator = " "
	}

	b.Lines[c.Line] = currentLine + separator + trimmed
	b.DeleteLine(c.Line + 1)
	b.Dirty = true
}

func (c *JoinNextLineCommand) Undo(b *buffer.Buffer) {
	if c.Line < 0 || c.Line >= len(b.Lines) {
		return
	}
	// Restore the current line to its original length
	currentRunes := []rune(b.Lines[c.Line])
	b.Lines[c.Line] = string(currentRunes[:c.JoinCol])
	b.InsertLine(c.Line+1, c.NextText)
}

// InsertLineCommand implements EditCommand for inserting a line.
type InsertLineCommand struct {
	Idx  int
	Text string
}

func (c *InsertLineCommand) Apply(b *buffer.Buffer) {
	b.InsertLine(c.Idx, c.Text)
}

func (c *InsertLineCommand) Undo(b *buffer.Buffer) {
	b.DeleteLine(c.Idx)
}

type DeleteLineCommand struct {
	Idx  int
	Text string
}

func (c *DeleteLineCommand) Apply(b *buffer.Buffer) {
	if c.Idx < 0 || c.Idx >= len(b.Lines) {
		return
	}
	c.Text = b.Lines[c.Idx]
	b.DeleteLine(c.Idx)
	b.Dirty = true
}

func (c *DeleteLineCommand) Undo(b *buffer.Buffer) {
	b.InsertLine(c.Idx, c.Text)
}

type SwapLineCommand struct {
	Line1, Line2 int
}

func (c *SwapLineCommand) Apply(b *buffer.Buffer) {
	if c.Line1 < 0 || c.Line2 < 0 || c.Line1 >= len(b.Lines) || c.Line2 >= len(b.Lines) {
		return
	}
	b.Lines[c.Line1], b.Lines[c.Line2] = b.Lines[c.Line2], b.Lines[c.Line1]
	b.Dirty = true
}

func (c *SwapLineCommand) Undo(b *buffer.Buffer) {
	c.Apply(b)
}

// InsertStringCommand implements EditCommand for inserting multiple runes (e.g. tab spaces).
type InsertStringCommand struct {
	Line, Col int
	Text      string
}

func (c *InsertStringCommand) Apply(b *buffer.Buffer) {
	if c.Line < 0 || c.Line >= len(b.Lines) {
		return
	}
	runes := []rune(b.Lines[c.Line])
	col := c.Col
	if col > len(runes) {
		col = len(runes)
	}
	insert := []rune(c.Text)
	newRunes := append([]rune{}, runes[:col]...)
	newRunes = append(newRunes, insert...)
	newRunes = append(newRunes, runes[col:]...)
	b.Lines[c.Line] = string(newRunes)
	b.Dirty = true
}

func (c *InsertStringCommand) Undo(b *buffer.Buffer) {
	if c.Line < 0 || c.Line >= len(b.Lines) {
		return
	}
	runes := []rune(b.Lines[c.Line])
	tLen := len([]rune(c.Text))
	col := c.Col
	if col+tLen > len(runes) {
		return
	}
	newRunes := append(runes[:col], runes[col+tLen:]...)
	b.Lines[c.Line] = string(newRunes)
	b.Dirty = true
}

// DeleteSelectionCommand deletes a multi-line range and stores the deleted text for undo.
type DeleteSelectionCommand struct {
	StartLine, StartCol int
	EndLine, EndCol     int
	Deleted             string
}

func (c *DeleteSelectionCommand) ComputeDeleted(b *buffer.Buffer) string {
	if c.StartLine >= len(b.Lines) {
		return ""
	}
	if c.EndLine >= len(b.Lines) {
		c.EndLine = len(b.Lines) - 1
		c.EndCol = len([]rune(b.Lines[c.EndLine]))
	}

	startRunes := []rune(b.Lines[c.StartLine])
	if c.StartCol > len(startRunes) {
		c.StartCol = len(startRunes)
	}

	if c.StartLine == c.EndLine {
		endRunes := []rune(b.Lines[c.StartLine])
		if c.EndCol > len(endRunes) {
			c.EndCol = len(endRunes)
		}
		if c.StartCol > c.EndCol {
			c.StartCol = c.EndCol
		}
		c.Deleted = string(endRunes[c.StartCol:c.EndCol])
		return c.Deleted
	}

	endRunes := []rune(b.Lines[c.EndLine])
	if c.EndCol > len(endRunes) {
		c.EndCol = len(endRunes)
	}

	var del []rune
	del = append(del, startRunes[c.StartCol:]...)
	del = append(del, '\n')
	for l := c.StartLine + 1; l < c.EndLine; l++ {
		del = append(del, []rune(b.Lines[l])...)
		del = append(del, '\n')
	}
	del = append(del, endRunes[:c.EndCol]...)
	c.Deleted = string(del)
	return c.Deleted
}

func (c *DeleteSelectionCommand) Apply(b *buffer.Buffer) {
	if c.StartLine >= len(b.Lines) {
		return
	}
	c.ComputeDeleted(b)

	startRunes := []rune(b.Lines[c.StartLine])
	endRunes := []rune(b.Lines[c.EndLine])
	b.Lines[c.StartLine] = string(startRunes[:c.StartCol]) + string(endRunes[c.EndCol:])
	if c.EndLine > c.StartLine {
		b.Lines = append(b.Lines[:c.StartLine+1], b.Lines[c.EndLine+1:]...)
	}
	b.Dirty = true
}

func (c *DeleteSelectionCommand) Undo(b *buffer.Buffer) {
	if c.StartLine >= len(b.Lines) {
		return
	}

	// Split current merged line at StartCol
	runes := []rune(b.Lines[c.StartLine])
	sc := c.StartCol
	if sc > len(runes) {
		sc = len(runes)
	}
	prefix := string(runes[:sc])
	suffix := string(runes[sc:])

	// Re-insert deleted text
	delLines := splitLines(c.Deleted)
	if len(delLines) == 1 {
		b.Lines[c.StartLine] = prefix + delLines[0] + suffix
	} else {
		newLines := make([]string, 0, len(b.Lines)+len(delLines)-1)
		newLines = append(newLines, b.Lines[:c.StartLine]...)
		newLines = append(newLines, prefix+delLines[0])
		for i := 1; i < len(delLines)-1; i++ {
			newLines = append(newLines, delLines[i])
		}
		newLines = append(newLines, delLines[len(delLines)-1]+suffix)
		newLines = append(newLines, b.Lines[c.StartLine+1:]...)
		b.Lines = newLines
	}
	b.Dirty = true
}

type PasteCommand struct {
	Line, Col int
	Text      string
	Suffix    string
}

func (c *PasteCommand) Apply(b *buffer.Buffer) {
	if c.Line < 0 || c.Line >= len(b.Lines) {
		return
	}
	lines := splitLines(c.Text)
	currentRunes := []rune(b.Lines[c.Line])
	col := c.Col
	if col > len(currentRunes) {
		col = len(currentRunes)
	}
	prefix := string(currentRunes[:col])

	if len(lines) == 1 {
		b.Lines[c.Line] = prefix + lines[0] + c.Suffix
	} else {
		newLines := make([]string, 0, len(b.Lines)+len(lines)-1)
		newLines = append(newLines, b.Lines[:c.Line]...)
		newLines = append(newLines, prefix+lines[0])
		for i := 1; i < len(lines)-1; i++ {
			newLines = append(newLines, lines[i])
		}
		newLines = append(newLines, lines[len(lines)-1]+c.Suffix)
		newLines = append(newLines, b.Lines[c.Line+1:]...)
		b.Lines = newLines
	}
	b.Dirty = true
}

func (c *PasteCommand) Undo(b *buffer.Buffer) {
	lines := splitLines(c.Text)
	if len(lines) == 1 {
		if c.Line < 0 || c.Line >= len(b.Lines) {
			return
		}
		runes := []rune(b.Lines[c.Line])
		tLen := len([]rune(lines[0]))
		col := c.Col
		if col+tLen > len(runes) {
			return
		}
		newRunes := append(runes[:col], runes[col+tLen:]...)
		b.Lines[c.Line] = string(newRunes)
	} else {
		if c.Line < 0 || c.Line >= len(b.Lines) {
			return
		}
		runes := []rune(b.Lines[c.Line])
		col := c.Col
		if col > len(runes) {
			col = len(runes)
		}
		restored := string(runes[:col]) + c.Suffix
		endLine := c.Line + len(lines) - 1
		if endLine >= len(b.Lines) {
			endLine = len(b.Lines) - 1
		}
		newLines := make([]string, 0, len(b.Lines)-(endLine-c.Line))
		newLines = append(newLines, b.Lines[:c.Line]...)
		newLines = append(newLines, restored)
		if endLine+1 < len(b.Lines) {
			newLines = append(newLines, b.Lines[endLine+1:]...)
		}
		b.Lines = newLines
	}
	b.Dirty = true
}

type BatchCommand struct {
	Commands []EditCommand
}

func (c *BatchCommand) Apply(b *buffer.Buffer) {
	for _, cmd := range c.Commands {
		cmd.Apply(b)
	}
}

func (c *BatchCommand) Undo(b *buffer.Buffer) {
	for i := len(c.Commands) - 1; i >= 0; i-- {
		c.Commands[i].Undo(b)
	}
}

// ReplaceLinesCommand replaces a contiguous range of buffer lines with new content.
// Used by sort, reverse, and unique line operations.
// The caller must set OldLines to the current buffer content being replaced
// and NewLines to the desired replacement.
type ReplaceLinesCommand struct {
	Start    int      // first line index (inclusive)
	OldLines []string // original lines (caller must populate before exec)
	NewLines []string // replacement lines
}

func (c *ReplaceLinesCommand) Apply(b *buffer.Buffer) {
	if c.Start < 0 || c.Start >= len(b.Lines) {
		return
	}
	end := c.Start + len(c.OldLines)
	if end > len(b.Lines) {
		end = len(b.Lines)
	}
	newBuf := make([]string, 0, len(b.Lines)-len(c.OldLines)+len(c.NewLines))
	newBuf = append(newBuf, b.Lines[:c.Start]...)
	newBuf = append(newBuf, c.NewLines...)
	newBuf = append(newBuf, b.Lines[end:]...)
	b.Lines = newBuf
	b.Dirty = true
}

func (c *ReplaceLinesCommand) Undo(b *buffer.Buffer) {
	if c.Start < 0 || c.Start >= len(b.Lines) {
		return
	}
	end := c.Start + len(c.NewLines)
	if end > len(b.Lines) {
		end = len(b.Lines)
	}
	newBuf := make([]string, 0, len(b.Lines)-len(c.NewLines)+len(c.OldLines))
	newBuf = append(newBuf, b.Lines[:c.Start]...)
	newBuf = append(newBuf, c.OldLines...)
	newBuf = append(newBuf, b.Lines[end:]...)
	b.Lines = newBuf
	b.Dirty = true
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, ch := range s {
		if ch == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
