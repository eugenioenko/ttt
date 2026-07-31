package undo

import (
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"testing"
)

func TestInsertRuneCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abc"}}
	cmd := &InsertRuneCommand{Line: 0, Col: 1, Rune: 'X'}
	cmd.Apply(b)
	if b.Lines[0] != "aXbc" {
		t.Errorf("expected 'aXbc', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if b.Lines[0] != "abc" {
		t.Errorf("expected 'abc', got '%s'", b.Lines[0])
	}
}

func TestInsertLineCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"a", "c"}}
	cmd := &InsertLineCommand{Idx: 1, Text: "b"}
	cmd.Apply(b)
	if len(b.Lines) != 3 || b.Lines[1] != "b" {
		t.Errorf("expected line 'b' at index 1, got '%v'", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[1] != "c" {
		t.Errorf("expected lines [a c], got '%v'", b.Lines)
	}
}

func TestPasteCommandSingleLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello world"}}
	cmd := &PasteCommand{Line: 0, Col: 5, Text: " beautiful", Suffix: " world"}
	cmd.Apply(b)
	if b.Lines[0] != "hello beautiful world" {
		t.Errorf("expected 'hello beautiful world', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if b.Lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", b.Lines[0])
	}
}

func TestPasteCommandMultiLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"ab"}}
	cmd := &PasteCommand{Line: 0, Col: 1, Text: "X\nY\nZ", Suffix: "b"}
	cmd.Apply(b)
	if len(b.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(b.Lines), b.Lines)
	}
	if b.Lines[0] != "aX" || b.Lines[1] != "Y" || b.Lines[2] != "Zb" {
		t.Errorf("unexpected lines: %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 1 || b.Lines[0] != "ab" {
		t.Errorf("expected ['ab'], got %v", b.Lines)
	}
}

func TestDeleteSelectionCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello", "world", "foo"}}
	cmd := &DeleteSelectionCommand{StartLine: 0, StartCol: 3, EndLine: 1, EndCol: 2}
	cmd.Apply(b)
	if len(b.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(b.Lines), b.Lines)
	}
	if b.Lines[0] != "helrld" {
		t.Errorf("expected 'helrld', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if len(b.Lines) != 3 || b.Lines[0] != "hello" || b.Lines[1] != "world" {
		t.Errorf("undo failed: %v", b.Lines)
	}
}

func TestDeleteSelectionUndoCursorPosition(t *testing.T) {
	cmd := &DeleteSelectionCommand{StartLine: 0, StartCol: 3, EndLine: 1, EndCol: 2}

	// Default: cursor at end of restored text
	s := &UndoStack{}
	pos := s.cursorAfterUndo(cmd)
	if pos.Line != 1 || pos.Col != 2 {
		t.Errorf("default: want {1,2}, got {%d,%d}", pos.Line, pos.Col)
	}

	// DeleteCursorStart: cursor at start of restored text
	s.DeleteCursorStart = true
	pos = s.cursorAfterUndo(cmd)
	if pos.Line != 0 || pos.Col != 3 {
		t.Errorf("DeleteCursorStart: want {0,3}, got {%d,%d}", pos.Line, pos.Col)
	}
}

func TestBatchCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abc"}}
	batch := &BatchCommand{
		Commands: []EditCommand{
			&InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'},
			&InsertRuneCommand{Line: 0, Col: 1, Rune: 'Y'},
			&InsertRuneCommand{Line: 0, Col: 2, Rune: 'Z'},
		},
	}
	batch.Apply(b)
	if b.Lines[0] != "XYZabc" {
		t.Errorf("expected 'XYZabc', got '%s'", b.Lines[0])
	}
	batch.Undo(b)
	if b.Lines[0] != "abc" {
		t.Errorf("expected 'abc' after undo, got '%s'", b.Lines[0])
	}
}

func TestBatchCommandUndoStack(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello"}}
	s := &UndoStack{}
	batch := &BatchCommand{
		Commands: []EditCommand{
			&InsertRuneCommand{Line: 0, Col: 5, Rune: '!'},
			&InsertRuneCommand{Line: 0, Col: 6, Rune: '!'},
		},
	}
	batch.Apply(b)
	s.Push(batch)
	if b.Lines[0] != "hello!!" {
		t.Errorf("expected 'hello!!', got '%s'", b.Lines[0])
	}
	s.Undo(b)
	if b.Lines[0] != "hello" {
		t.Errorf("expected 'hello' after single undo, got '%s'", b.Lines[0])
	}
	s.Redo(b)
	if b.Lines[0] != "hello!!" {
		t.Errorf("expected 'hello!!' after redo, got '%s'", b.Lines[0])
	}
}

func TestUndoStack(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abc"}}
	s := &UndoStack{}
	cmd := &InsertRuneCommand{Line: 0, Col: 3, Rune: '!'}
	s.Push(cmd)
	cmd.Apply(b)
	s.Undo(b)
	if b.Lines[0] != "abc" {
		t.Errorf("expected 'abc', got '%s'", b.Lines[0])
	}
	s.Redo(b)
	if b.Lines[0] != "abc!" {
		t.Errorf("expected 'abc!', got '%s'", b.Lines[0])
	}
}

func TestUndoGroupsConsecutiveInserts(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{""}}
	s := &UndoStack{}
	for i, r := range []rune("hello") {
		cmd := &InsertRuneCommand{Line: 0, Col: i, Rune: r}
		cmd.Apply(b)
		s.Push(cmd)
	}
	if b.Lines[0] != "hello" {
		t.Fatalf("expected 'hello', got '%s'", b.Lines[0])
	}
	s.Undo(b)
	if b.Lines[0] != "" {
		t.Errorf("expected '' after single undo, got '%s'", b.Lines[0])
	}
	s.Redo(b)
	if b.Lines[0] != "hello" {
		t.Errorf("expected 'hello' after redo, got '%s'", b.Lines[0])
	}
}

func TestUndoBreaksGroupOnSpace(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{""}}
	s := &UndoStack{}
	for i, r := range []rune("hi there") {
		cmd := &InsertRuneCommand{Line: 0, Col: i, Rune: r}
		cmd.Apply(b)
		s.Push(cmd)
	}
	if b.Lines[0] != "hi there" {
		t.Fatalf("expected 'hi there', got '%s'", b.Lines[0])
	}
	// Space belongs with the next word: undo removes " there", then "hi"
	s.Undo(b)
	if b.Lines[0] != "hi" {
		t.Errorf("expected 'hi' after first undo, got '%s'", b.Lines[0])
	}
	s.Undo(b)
	if b.Lines[0] != "" {
		t.Errorf("expected '' after second undo, got '%s'", b.Lines[0])
	}
}

func TestUndoBreaksGroupOnBreakGroup(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{""}}
	s := &UndoStack{}
	for i, r := range []rune("ab") {
		cmd := &InsertRuneCommand{Line: 0, Col: i, Rune: r}
		cmd.Apply(b)
		s.Push(cmd)
	}
	s.BreakGroup()
	for i, r := range []rune("cd") {
		cmd := &InsertRuneCommand{Line: 0, Col: 2 + i, Rune: r}
		cmd.Apply(b)
		s.Push(cmd)
	}
	if b.Lines[0] != "abcd" {
		t.Fatalf("expected 'abcd', got '%s'", b.Lines[0])
	}
	s.Undo(b)
	if b.Lines[0] != "ab" {
		t.Errorf("expected 'ab' after first undo, got '%s'", b.Lines[0])
	}
	s.Undo(b)
	if b.Lines[0] != "" {
		t.Errorf("expected '' after second undo, got '%s'", b.Lines[0])
	}
}

func TestUndoGroupsConsecutiveDeletes(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello"}}
	s := &UndoStack{}
	for i := 4; i >= 0; i-- {
		cmd := &DeleteRuneCommand{Line: 0, Col: i}
		cmd.Apply(b)
		s.Push(cmd)
	}
	if b.Lines[0] != "" {
		t.Fatalf("expected '', got '%s'", b.Lines[0])
	}
	s.Undo(b)
	if b.Lines[0] != "hello" {
		t.Errorf("expected 'hello' after single undo, got '%s'", b.Lines[0])
	}
}

// --- DeleteRuneCommand ---

func TestDeleteRuneCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abcde"}}
	cmd := &DeleteRuneCommand{Line: 0, Col: 2}
	cmd.Apply(b)
	if b.Lines[0] != "abde" {
		t.Errorf("expected 'abde', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if b.Lines[0] != "abcde" {
		t.Errorf("expected 'abcde', got '%s'", b.Lines[0])
	}
}

func TestDeleteRuneCommandCapturesRune(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello"}}
	cmd := &DeleteRuneCommand{Line: 0, Col: 0}
	cmd.Apply(b)
	if cmd.Rune != 'h' {
		t.Errorf("expected captured rune 'h', got '%c'", cmd.Rune)
	}
	if b.Lines[0] != "ello" {
		t.Errorf("expected 'ello', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if b.Lines[0] != "hello" {
		t.Errorf("expected 'hello', got '%s'", b.Lines[0])
	}
}

func TestDeleteRuneCommandLastChar(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"xy"}}
	cmd := &DeleteRuneCommand{Line: 0, Col: 1}
	cmd.Apply(b)
	if b.Lines[0] != "x" {
		t.Errorf("expected 'x', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if b.Lines[0] != "xy" {
		t.Errorf("expected 'xy', got '%s'", b.Lines[0])
	}
}

func TestDeleteRuneCommandCursorPositions(t *testing.T) {
	undoPos := (&UndoStack{}).cursorAfterUndo(&DeleteRuneCommand{Line: 2, Col: 5})
	if undoPos == nil || undoPos.Line != 2 || undoPos.Col != 6 {
		t.Errorf("expected undo cursor {2, 6}, got %+v", undoPos)
	}
	redoPos := cursorAfterRedo(&DeleteRuneCommand{Line: 2, Col: 5})
	if redoPos == nil || redoPos.Line != 2 || redoPos.Col != 5 {
		t.Errorf("expected redo cursor {2, 5}, got %+v", redoPos)
	}
}

// --- SplitLineCommand ---

func TestSplitLineCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello world"}}
	cmd := &SplitLineCommand{Line: 0, Col: 5}
	cmd.Apply(b)
	if len(b.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(b.Lines))
	}
	if b.Lines[0] != "hello" || b.Lines[1] != " world" {
		t.Errorf("expected ['hello', ' world'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 1 || b.Lines[0] != "hello world" {
		t.Errorf("expected ['hello world'], got %v", b.Lines)
	}
}

func TestSplitLineCommandAtStart(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abc"}}
	cmd := &SplitLineCommand{Line: 0, Col: 0}
	cmd.Apply(b)
	if len(b.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(b.Lines))
	}
	if b.Lines[0] != "" || b.Lines[1] != "abc" {
		t.Errorf("expected ['', 'abc'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 1 || b.Lines[0] != "abc" {
		t.Errorf("expected ['abc'], got %v", b.Lines)
	}
}

func TestSplitLineCommandAtEnd(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abc"}}
	cmd := &SplitLineCommand{Line: 0, Col: 3}
	cmd.Apply(b)
	if len(b.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(b.Lines))
	}
	if b.Lines[0] != "abc" || b.Lines[1] != "" {
		t.Errorf("expected ['abc', ''], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 1 || b.Lines[0] != "abc" {
		t.Errorf("expected ['abc'], got %v", b.Lines)
	}
}

func TestSplitLineCommandMiddleOfBuffer(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"aaa", "bbbccc", "ddd"}}
	cmd := &SplitLineCommand{Line: 1, Col: 3}
	cmd.Apply(b)
	if len(b.Lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(b.Lines))
	}
	if b.Lines[0] != "aaa" || b.Lines[1] != "bbb" || b.Lines[2] != "ccc" || b.Lines[3] != "ddd" {
		t.Errorf("expected ['aaa', 'bbb', 'ccc', 'ddd'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 3 || b.Lines[0] != "aaa" || b.Lines[1] != "bbbccc" || b.Lines[2] != "ddd" {
		t.Errorf("expected ['aaa', 'bbbccc', 'ddd'], got %v", b.Lines)
	}
}

func TestSplitLineCommandCursorPositions(t *testing.T) {
	undoPos := (&UndoStack{}).cursorAfterUndo(&SplitLineCommand{Line: 3, Col: 7})
	if undoPos == nil || undoPos.Line != 3 || undoPos.Col != 7 {
		t.Errorf("expected undo cursor {3, 7}, got %+v", undoPos)
	}
	redoPos := cursorAfterRedo(&SplitLineCommand{Line: 3, Col: 7})
	if redoPos == nil || redoPos.Line != 4 || redoPos.Col != 0 {
		t.Errorf("expected redo cursor {4, 0}, got %+v", redoPos)
	}
}

// --- JoinLineCommand ---

func TestJoinLineCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello", " world"}}
	cmd := &JoinLineCommand{Line: 1}
	cmd.Apply(b)
	if len(b.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(b.Lines))
	}
	if b.Lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", b.Lines[0])
	}
	if cmd.PrevLen != 5 {
		t.Errorf("expected PrevLen 5, got %d", cmd.PrevLen)
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "hello" || b.Lines[1] != " world" {
		t.Errorf("expected ['hello', ' world'], got %v", b.Lines)
	}
}

func TestJoinLineCommandEmptySecondLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abc", ""}}
	cmd := &JoinLineCommand{Line: 1}
	cmd.Apply(b)
	if len(b.Lines) != 1 || b.Lines[0] != "abc" {
		t.Errorf("expected ['abc'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "abc" || b.Lines[1] != "" {
		t.Errorf("expected ['abc', ''], got %v", b.Lines)
	}
}

func TestJoinLineCommandEmptyFirstLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"", "xyz"}}
	cmd := &JoinLineCommand{Line: 1}
	cmd.Apply(b)
	if len(b.Lines) != 1 || b.Lines[0] != "xyz" {
		t.Errorf("expected ['xyz'], got %v", b.Lines)
	}
	if cmd.PrevLen != 0 {
		t.Errorf("expected PrevLen 0, got %d", cmd.PrevLen)
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "" || b.Lines[1] != "xyz" {
		t.Errorf("expected ['', 'xyz'], got %v", b.Lines)
	}
}

func TestJoinLineCommandCursorPositions(t *testing.T) {
	cmd := &JoinLineCommand{Line: 2, PrevLen: 5}
	cmd.PrevLen = 5 // simulate what Apply would set
	undoPos := (&UndoStack{}).cursorAfterUndo(cmd)
	if undoPos == nil || undoPos.Line != 1 || undoPos.Col != 5 {
		t.Errorf("expected undo cursor {1, 5}, got %+v", undoPos)
	}
	redoPos := cursorAfterRedo(cmd)
	if redoPos == nil || redoPos.Line != 1 || redoPos.Col != 5 {
		t.Errorf("expected redo cursor {1, 5}, got %+v", redoPos)
	}
}

// --- JoinNextLineCommand ---

func TestJoinNextLineCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello", "world"}}
	cmd := &JoinNextLineCommand{Line: 0}
	cmd.Apply(b)
	if len(b.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(b.Lines))
	}
	if b.Lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", b.Lines[0])
	}
	if cmd.JoinCol != 5 {
		t.Errorf("expected JoinCol 5, got %d", cmd.JoinCol)
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "hello" || b.Lines[1] != "world" {
		t.Errorf("expected ['hello', 'world'], got %v", b.Lines)
	}
}

func TestJoinNextLineCommandTrimsWhitespace(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello", "   world"}}
	cmd := &JoinNextLineCommand{Line: 0}
	cmd.Apply(b)
	if b.Lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "hello" || b.Lines[1] != "   world" {
		t.Errorf("expected ['hello', '   world'], got %v", b.Lines)
	}
}

func TestJoinNextLineCommandCurrentEndsWithSpace(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello ", "world"}}
	cmd := &JoinNextLineCommand{Line: 0}
	cmd.Apply(b)
	// Should not add an extra space because current line already ends with space
	if b.Lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "hello " || b.Lines[1] != "world" {
		t.Errorf("expected ['hello ', 'world'], got %v", b.Lines)
	}
}

func TestJoinNextLineCommandEmptyNextLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abc", ""}}
	cmd := &JoinNextLineCommand{Line: 0}
	cmd.Apply(b)
	// Empty next line, trimmed is "", so no separator added
	if b.Lines[0] != "abc" {
		t.Errorf("expected 'abc', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "abc" || b.Lines[1] != "" {
		t.Errorf("expected ['abc', ''], got %v", b.Lines)
	}
}

func TestJoinNextLineCommandWhitespaceOnlyNextLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abc", "   "}}
	cmd := &JoinNextLineCommand{Line: 0}
	cmd.Apply(b)
	// Next line is all whitespace, trimmed is "", so no separator added
	if b.Lines[0] != "abc" {
		t.Errorf("expected 'abc', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "abc" || b.Lines[1] != "   " {
		t.Errorf("expected ['abc', '   '], got %v", b.Lines)
	}
}

func TestJoinNextLineCommandCursorPositions(t *testing.T) {
	cmd := &JoinNextLineCommand{Line: 3, JoinCol: 10}
	undoPos := (&UndoStack{}).cursorAfterUndo(cmd)
	if undoPos == nil || undoPos.Line != 3 || undoPos.Col != 10 {
		t.Errorf("expected undo cursor {3, 10}, got %+v", undoPos)
	}
	redoPos := cursorAfterRedo(cmd)
	if redoPos == nil || redoPos.Line != 3 || redoPos.Col != 10 {
		t.Errorf("expected redo cursor {3, 10}, got %+v", redoPos)
	}
}

// --- DeleteLineCommand ---

func TestDeleteLineCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"aaa", "bbb", "ccc"}}
	cmd := &DeleteLineCommand{Idx: 1}
	cmd.Apply(b)
	if len(b.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(b.Lines))
	}
	if b.Lines[0] != "aaa" || b.Lines[1] != "ccc" {
		t.Errorf("expected ['aaa', 'ccc'], got %v", b.Lines)
	}
	if cmd.Text != "bbb" {
		t.Errorf("expected captured text 'bbb', got '%s'", cmd.Text)
	}
	cmd.Undo(b)
	if len(b.Lines) != 3 || b.Lines[0] != "aaa" || b.Lines[1] != "bbb" || b.Lines[2] != "ccc" {
		t.Errorf("expected ['aaa', 'bbb', 'ccc'], got %v", b.Lines)
	}
}

func TestDeleteLineCommandFirstLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"first", "second"}}
	cmd := &DeleteLineCommand{Idx: 0}
	cmd.Apply(b)
	if len(b.Lines) != 1 || b.Lines[0] != "second" {
		t.Errorf("expected ['second'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "first" || b.Lines[1] != "second" {
		t.Errorf("expected ['first', 'second'], got %v", b.Lines)
	}
}

func TestDeleteLineCommandLastLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"first", "second"}}
	cmd := &DeleteLineCommand{Idx: 1}
	cmd.Apply(b)
	if len(b.Lines) != 1 || b.Lines[0] != "first" {
		t.Errorf("expected ['first'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 2 || b.Lines[0] != "first" || b.Lines[1] != "second" {
		t.Errorf("expected ['first', 'second'], got %v", b.Lines)
	}
}

// --- SwapLineCommand ---

func TestSwapLineCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"aaa", "bbb", "ccc"}}
	cmd := &SwapLineCommand{Line1: 0, Line2: 2}
	cmd.Apply(b)
	if b.Lines[0] != "ccc" || b.Lines[1] != "bbb" || b.Lines[2] != "aaa" {
		t.Errorf("expected ['ccc', 'bbb', 'aaa'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if b.Lines[0] != "aaa" || b.Lines[1] != "bbb" || b.Lines[2] != "ccc" {
		t.Errorf("expected ['aaa', 'bbb', 'ccc'], got %v", b.Lines)
	}
}

func TestSwapLineCommandAdjacent(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"first", "second", "third"}}
	cmd := &SwapLineCommand{Line1: 1, Line2: 2}
	cmd.Apply(b)
	if b.Lines[1] != "third" || b.Lines[2] != "second" {
		t.Errorf("expected lines 1,2 to be ['third', 'second'], got ['%s', '%s']", b.Lines[1], b.Lines[2])
	}
	cmd.Undo(b)
	if b.Lines[1] != "second" || b.Lines[2] != "third" {
		t.Errorf("expected lines 1,2 to be ['second', 'third'], got ['%s', '%s']", b.Lines[1], b.Lines[2])
	}
}

func TestSwapLineCommandSameLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"aaa", "bbb"}}
	cmd := &SwapLineCommand{Line1: 0, Line2: 0}
	cmd.Apply(b)
	if b.Lines[0] != "aaa" || b.Lines[1] != "bbb" {
		t.Errorf("swapping same line should be no-op, got %v", b.Lines)
	}
	cmd.Undo(b)
	if b.Lines[0] != "aaa" || b.Lines[1] != "bbb" {
		t.Errorf("undo of same-line swap should be no-op, got %v", b.Lines)
	}
}

// --- InsertStringCommand ---

func TestInsertStringCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello world"}}
	cmd := &InsertStringCommand{Line: 0, Col: 5, Text: " beautiful"}
	cmd.Apply(b)
	if b.Lines[0] != "hello beautiful world" {
		t.Errorf("expected 'hello beautiful world', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if b.Lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", b.Lines[0])
	}
}

func TestInsertStringCommandAtStart(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"world"}}
	cmd := &InsertStringCommand{Line: 0, Col: 0, Text: "hello "}
	cmd.Apply(b)
	if b.Lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if b.Lines[0] != "world" {
		t.Errorf("expected 'world', got '%s'", b.Lines[0])
	}
}

func TestInsertStringCommandAtEnd(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello"}}
	cmd := &InsertStringCommand{Line: 0, Col: 5, Text: " world"}
	cmd.Apply(b)
	if b.Lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if b.Lines[0] != "hello" {
		t.Errorf("expected 'hello', got '%s'", b.Lines[0])
	}
}

func TestInsertStringCommandTabSpaces(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"func()"}}
	cmd := &InsertStringCommand{Line: 0, Col: 0, Text: "    "}
	cmd.Apply(b)
	if b.Lines[0] != "    func()" {
		t.Errorf("expected '    func()', got '%s'", b.Lines[0])
	}
	cmd.Undo(b)
	if b.Lines[0] != "func()" {
		t.Errorf("expected 'func()', got '%s'", b.Lines[0])
	}
}

func TestInsertStringCommandCursorPositions(t *testing.T) {
	undoPos := (&UndoStack{}).cursorAfterUndo(&InsertStringCommand{Line: 1, Col: 3, Text: "abc"})
	if undoPos == nil || undoPos.Line != 1 || undoPos.Col != 3 {
		t.Errorf("expected undo cursor {1, 3}, got %+v", undoPos)
	}
	redoPos := cursorAfterRedo(&InsertStringCommand{Line: 1, Col: 3, Text: "abc"})
	if redoPos == nil || redoPos.Line != 1 || redoPos.Col != 6 {
		t.Errorf("expected redo cursor {1, 6}, got %+v", redoPos)
	}
}

// --- ReplaceLinesCommand ---

func TestReplaceLinesCommand(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"ccc", "aaa", "bbb", "ddd"}}
	cmd := &ReplaceLinesCommand{
		Start:    1,
		OldLines: []string{"aaa", "bbb"},
		NewLines: []string{"aaa", "bbb"},
	}
	// Sort: replace with sorted version
	cmd.NewLines = []string{"aaa", "bbb"}
	cmd.Apply(b)
	if len(b.Lines) != 4 || b.Lines[1] != "aaa" || b.Lines[2] != "bbb" {
		t.Errorf("expected ['ccc', 'aaa', 'bbb', 'ddd'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 4 || b.Lines[1] != "aaa" || b.Lines[2] != "bbb" {
		t.Errorf("expected ['ccc', 'aaa', 'bbb', 'ddd'], got %v", b.Lines)
	}
}

func TestReplaceLinesCommandSort(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"cherry", "apple", "banana"}}
	cmd := &ReplaceLinesCommand{
		Start:    0,
		OldLines: []string{"cherry", "apple", "banana"},
		NewLines: []string{"apple", "banana", "cherry"},
	}
	cmd.Apply(b)
	if b.Lines[0] != "apple" || b.Lines[1] != "banana" || b.Lines[2] != "cherry" {
		t.Errorf("expected sorted lines, got %v", b.Lines)
	}
	cmd.Undo(b)
	if b.Lines[0] != "cherry" || b.Lines[1] != "apple" || b.Lines[2] != "banana" {
		t.Errorf("expected original lines, got %v", b.Lines)
	}
}

func TestReplaceLinesCommandDifferentLength(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"aaa", "bbb", "bbb", "ccc"}}
	cmd := &ReplaceLinesCommand{
		Start:    1,
		OldLines: []string{"bbb", "bbb"},
		NewLines: []string{"bbb"},
	}
	cmd.Apply(b)
	if len(b.Lines) != 3 || b.Lines[0] != "aaa" || b.Lines[1] != "bbb" || b.Lines[2] != "ccc" {
		t.Errorf("expected ['aaa', 'bbb', 'ccc'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 4 || b.Lines[1] != "bbb" || b.Lines[2] != "bbb" {
		t.Errorf("expected ['aaa', 'bbb', 'bbb', 'ccc'], got %v", b.Lines)
	}
}

func TestReplaceLinesCommandReverse(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"first", "aaa", "bbb", "ccc", "last"}}
	cmd := &ReplaceLinesCommand{
		Start:    1,
		OldLines: []string{"aaa", "bbb", "ccc"},
		NewLines: []string{"ccc", "bbb", "aaa"},
	}
	cmd.Apply(b)
	if b.Lines[1] != "ccc" || b.Lines[2] != "bbb" || b.Lines[3] != "aaa" {
		t.Errorf("expected reversed lines, got %v", b.Lines)
	}
	if b.Lines[0] != "first" || b.Lines[4] != "last" {
		t.Errorf("surrounding lines should be unchanged, got %v", b.Lines)
	}
	cmd.Undo(b)
	if b.Lines[1] != "aaa" || b.Lines[2] != "bbb" || b.Lines[3] != "ccc" {
		t.Errorf("expected original lines after undo, got %v", b.Lines)
	}
}

func TestReplaceLinesCommandExpandLines(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"aaa", "bbb", "ccc"}}
	cmd := &ReplaceLinesCommand{
		Start:    1,
		OldLines: []string{"bbb"},
		NewLines: []string{"xxx", "yyy", "zzz"},
	}
	cmd.Apply(b)
	if len(b.Lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(b.Lines), b.Lines)
	}
	if b.Lines[0] != "aaa" || b.Lines[1] != "xxx" || b.Lines[2] != "yyy" || b.Lines[3] != "zzz" || b.Lines[4] != "ccc" {
		t.Errorf("expected ['aaa', 'xxx', 'yyy', 'zzz', 'ccc'], got %v", b.Lines)
	}
	cmd.Undo(b)
	if len(b.Lines) != 3 || b.Lines[0] != "aaa" || b.Lines[1] != "bbb" || b.Lines[2] != "ccc" {
		t.Errorf("expected ['aaa', 'bbb', 'ccc'], got %v", b.Lines)
	}
}

// --- Transaction (undo grouping) ---

func TestTransactionGroupsMultipleEdits(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello world"}}
	s := &UndoStack{}

	s.BeginTransaction()

	del := &DeleteSelectionCommand{StartLine: 0, StartCol: 6, EndLine: 0, EndCol: 11}
	del.Apply(b)
	s.Push(del)

	ins := &InsertStringCommand{Line: 0, Col: 6, Text: "there"}
	ins.Apply(b)
	s.Push(ins)

	if b.Lines[0] != "hello there" {
		t.Fatalf("expected 'hello there', got '%s'", b.Lines[0])
	}

	s.EndTransaction()

	s.Undo(b)
	if b.Lines[0] != "hello world" {
		t.Errorf("expected 'hello world' after single undo, got '%s'", b.Lines[0])
	}

	s.Redo(b)
	if b.Lines[0] != "hello there" {
		t.Errorf("expected 'hello there' after redo, got '%s'", b.Lines[0])
	}
}

func TestTransactionNoEdits(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"abc"}}
	s := &UndoStack{}

	cmd := &InsertRuneCommand{Line: 0, Col: 3, Rune: '!'}
	cmd.Apply(b)
	s.Push(cmd)

	s.BeginTransaction()
	s.EndTransaction()

	s.Undo(b)
	if b.Lines[0] != "abc" {
		t.Errorf("expected 'abc', got '%s'", b.Lines[0])
	}
}

func TestEndTransactionWithoutBegin(t *testing.T) {
	s := &UndoStack{}
	s.EndTransaction()
}
