package undo

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
)

// assertChangesCover replays the buffer's recorded changes over a copy of the
// lines from before an edit and fails if a line the changes do not cover
// differs afterwards.
func assertChangesCover(t *testing.T, name string, before []string, v uint64, b *buffer.Buffer) {
	t.Helper()
	const changed = "\x00changed"
	mirror := slices.Clone(before)
	if !b.ChangesSince(v, func(c buffer.Change) {
		mirror = slices.Replace(mirror, c.Start, c.Start+c.Removed, slices.Repeat([]string{changed}, c.Added)...)
	}) {
		t.Fatalf("%s: changes not replayable", name)
	}
	if len(mirror) != len(b.Lines) {
		t.Fatalf("%s: changes give %d lines, buffer has %d", name, len(mirror), len(b.Lines))
	}
	for i, l := range mirror {
		if l != changed && l != b.Lines[i] {
			t.Fatalf("%s: line %d changed to %q without a recorded change", name, i, b.Lines[i])
		}
	}
}

func TestEveryEditCommandRecordsItsLineChanges(t *testing.T) {
	cmds := map[string]func() EditCommand{
		"insert rune":    func() EditCommand { return &InsertRuneCommand{Line: 1, Col: 1, Rune: 'x'} },
		"delete rune":    func() EditCommand { return &DeleteRuneCommand{Line: 1, Col: 0} },
		"split":          func() EditCommand { return &SplitLineCommand{Line: 1, Col: 2} },
		"join":           func() EditCommand { return &JoinLineCommand{Line: 2} },
		"join next":      func() EditCommand { return &JoinNextLineCommand{Line: 1} },
		"insert line":    func() EditCommand { return &InsertLineCommand{Idx: 2, Text: "new"} },
		"delete line":    func() EditCommand { return &DeleteLineCommand{Idx: 2} },
		"swap":           func() EditCommand { return &SwapLineCommand{Line1: 0, Line2: 3} },
		"insert string":  func() EditCommand { return &InsertStringCommand{Line: 0, Col: 1, Text: "abc"} },
		"delete range":   func() EditCommand { return &DeleteSelectionCommand{StartLine: 0, StartCol: 1, EndLine: 2, EndCol: 2} },
		"paste one line": func() EditCommand { return &PasteCommand{Line: 1, Col: 1, Text: "pp"} },
		"paste lines":    func() EditCommand { return &PasteCommand{Line: 1, Col: 1, Text: "p\nq\nr"} },
		"replace lines": func() EditCommand {
			return &ReplaceLinesCommand{Start: 1, OldLines: []string{"line1", "  line2"}, NewLines: []string{"z"}}
		},
		"batch": func() EditCommand {
			return &BatchCommand{Commands: []EditCommand{&InsertLineCommand{Idx: 0, Text: "a"}, &DeleteLineCommand{Idx: 3}}}
		},
		"replace to more": func() EditCommand {
			return &ReplaceLinesCommand{Start: 0, OldLines: []string{"line0"}, NewLines: []string{"x", "y", "z"}}
		},
	}
	for name, mk := range cmds {
		b := &buffer.Buffer{Lines: []string{"line0", "line1", "  line2", "line3"}}
		cmd := mk()
		before, v := slices.Clone(b.Lines), b.Version()
		cmd.Apply(b)
		assertChangesCover(t, name+" apply", before, v, b)
		before, v = slices.Clone(b.Lines), b.Version()
		cmd.Undo(b)
		assertChangesCover(t, name+" undo", before, v, b)
	}
}
