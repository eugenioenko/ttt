package undo

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
)

func TestSavePointSurvivesRedoWithoutBranching(t *testing.T) {
	for _, noop := range []bool{false, true} {
		t.Run(map[bool]string{false: "redo", true: "noop then redo"}[noop], func(t *testing.T) {
			b := &buffer.Buffer{Lines: []string{""}}
			s := &UndoStack{}
			cmd := &InsertStringCommand{Line: 0, Col: 0, Text: "A"}
			cmd.Apply(b)
			s.Push(cmd)
			s.MarkSaved()
			s.Undo(b)
			if noop {
				cmd := &DeleteSelectionCommand{StartLine: 0, EndLine: 0}
				cmd.Apply(b)
				s.Push(cmd)
			}
			if s.AtSavePoint() {
				t.Fatal("undo matched the save point")
			}
			s.Redo(b)
			if b.Lines[0] != "A" || !s.AtSavePoint() {
				t.Fatalf("redo: text = %q, AtSavePoint = %v", b.Lines[0], s.AtSavePoint())
			}
		})
	}
}

func TestTransactionRemapsSavePoint(t *testing.T) {
	for _, tc := range []struct {
		name          string
		savedText     string
		edits         string
		undoBeforeEnd int
		branch        bool
		nested        bool
		reachable     bool
	}{
		{name: "before transaction", edits: "AB", reachable: true},
		{name: "transaction start", savedText: "P", edits: "AB", reachable: true},
		{name: "inside transaction", savedText: "PA", edits: "AB"},
		{name: "transaction end", savedText: "PAB", edits: "AB", reachable: true},
		{name: "saved redo", savedText: "PABC", edits: "ABC", undoBeforeEnd: 1, reachable: true},
		{name: "empty transaction", savedText: "P", reachable: true},
		{name: "single edit", savedText: "PA", edits: "A", reachable: true},
		{name: "all edits undone", savedText: "PA", edits: "A", undoBeforeEnd: 1, reachable: true},
		{name: "discarded save point", savedText: "PA", edits: "AB", undoBeforeEnd: 2, branch: true},
		{name: "nested end", savedText: "PAB", edits: "AB", nested: true, reachable: true},
		{name: "nested interior", savedText: "PA", edits: "AB", nested: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &buffer.Buffer{Lines: []string{""}}
			s := &UndoStack{}
			insert := func(text string) {
				cmd := &InsertStringCommand{Line: 0, Col: len([]rune(b.Lines[0])), Text: text}
				cmd.Apply(b)
				s.Push(cmd)
				if b.Lines[0] == tc.savedText {
					s.MarkSaved()
				}
			}
			insert("P")
			s.BeginTransaction()
			for i, r := range tc.edits {
				if tc.nested && i == 1 {
					s.BeginTransaction()
				}
				insert(string(r))
			}
			for i := 0; i < tc.undoBeforeEnd; i++ {
				s.Undo(b)
			}
			if tc.branch {
				insert("X")
			}
			if tc.nested {
				s.EndTransaction()
			}
			s.EndTransaction()
			check := func() {
				t.Helper()
				want := tc.reachable && b.Lines[0] == tc.savedText
				if got := s.AtSavePoint(); got != want {
					t.Errorf("text = %q, AtSavePoint = %v, want %v", b.Lines[0], got, want)
				}
			}
			check()
			for s.Undo(b) != nil {
				check()
			}
			for s.Redo(b) != nil {
				check()
			}
			insert("Q")
			check()
			s.Undo(b)
			check()
		})
	}
}
