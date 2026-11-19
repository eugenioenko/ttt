package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
)

func TestAutoSurroundSelection(t *testing.T) {
	for _, pair := range []string{"()", "[]", "{}", "\"\"", "''", "``", "**", "«»"} {
		for _, text := range []string{"word", "界🙂", "one\n界🙂", "one\n"} {
			for _, reversed := range []bool{false, true} {
				e := autoClosingEditor(t, text, "plain.txt", 0)
				e.AutoClosing = config.AutoClosingSettings{}
				e.AutoSurround.ExtraPairs = []config.SurroundingPair{{"*", "*"}, {"«", "»"}}
				end := selection.Position{Line: len(e.Buf.Lines) - 1, Col: len([]rune(e.Buf.Lines[len(e.Buf.Lines)-1]))}
				e.Selection.Start(0, 0)
				e.Cursor.Line, e.Cursor.Col = end.Line, end.Col
				if reversed {
					e.Selection.Anchor = end
					e.Cursor.Line, e.Cursor.Col = 0, 0
				}
				runes := []rune(pair)
				for n := 1; n <= 2; n++ {
					typeAutoClosing(e, string(runes[0]))
					want := strings.Repeat(string(runes[0]), n) + text + strings.Repeat(string(runes[1]), n)
					if got := strings.Join(e.Buf.Lines, "\n"); got != want {
						t.Fatalf("pair %q reversed %v: got %q, want %q", pair, reversed, got, want)
					}
					if got := e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col); got != text {
						t.Fatalf("selection = %q, want %q", got, text)
					}
					start, _ := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
					if (selection.Position{Line: e.Cursor.Line, Col: e.Cursor.Col} == start) != reversed {
						t.Fatal("selection direction changed")
					}
				}
				for n := 1; n >= 0; n-- {
					e.Undo.Undo(e.Buf)
					want := strings.Repeat(string(runes[0]), n) + text + strings.Repeat(string(runes[1]), n)
					if got := strings.Join(e.Buf.Lines, "\n"); got != want {
						t.Fatalf("undo = %q, want %q", got, want)
					}
				}
				e.Undo.Redo(e.Buf)
				if got := strings.Join(e.Buf.Lines, "\n"); got != string(runes[0])+text+string(runes[1]) {
					t.Fatalf("redo = %q", got)
				}
			}
		}
	}
}

func TestAutoSurroundLiteralInput(t *testing.T) {
	for _, tc := range []struct {
		name, typed, want                string
		disabled, readOnly, empty, paste bool
	}{
		{name: "disabled", typed: "(", want: "(", disabled: true},
		{name: "unconfigured extra", typed: "%", want: "%"},
		{name: "closer", typed: ")", want: ")"},
		{name: "ordinary typing", typed: "x", want: "x"},
		{name: "read only", typed: "(", want: "word", readOnly: true},
		{name: "empty selection", typed: "(", want: "(word", empty: true},
		{name: "paste", typed: "(", want: "(", paste: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := autoClosingEditor(t, "word", "plain.txt", 4)
			e.Selection.Start(0, 0)
			if tc.empty {
				e.Cursor.Col = 0
			}
			enabled := !tc.disabled
			e.AutoSurround.Enabled = &enabled
			e.ReadOnly = tc.readOnly
			if tc.paste {
				e.pasteText(tc.typed)
			} else {
				typeAutoClosing(e, tc.typed)
			}
			if e.Buf.Lines[0] != tc.want {
				t.Fatalf("got %q, want %q", e.Buf.Lines[0], tc.want)
			}
		})
	}
}

func TestAutoSurroundMultiCursor(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		e := autoClosingEditor(t, "word word\nword ", "plain.txt", 0)
		e.AutoClosing = config.AutoClosingSettings{}
		e.Multi = &multicursor.MultiCursor{}
		for _, start := range []selection.Position{{Line: 0, Col: 0}, {Line: 0, Col: 5}, {Line: 1, Col: 0}} {
			cs := multicursor.CursorState{Line: start.Line, Col: start.Col + 4, Sel: selection.Selection{Active: true, Anchor: start}}
			if reversed {
				cs.Sel.Anchor.Col += 4
				cs.Col -= 4
			}
			e.Multi.Cursors = append(e.Multi.Cursors, cs)
		}
		e.Multi.Add(1, 5)
		e.syncFromMulti()
		for n := 1; n <= 2; n++ {
			typeAutoClosing(e, "(")
			word := strings.Repeat("(", n) + "word" + strings.Repeat(")", n)
			tail := " " + strings.Repeat("(", n)
			if got := strings.Join(e.Buf.Lines, "\n"); got != word+" "+word+"\n"+word+tail {
				t.Fatalf("reversed %v: %q", reversed, got)
			}
			for _, cs := range e.Multi.Cursors {
				if cs.Sel.Active && cs.Sel.Text(e.Buf.Lines, cs.Line, cs.Col) != "word" {
					t.Fatalf("selection changed: %+v", cs)
				}
			}
		}
		e.Undo.Undo(e.Buf)
		e.Undo.Undo(e.Buf)
		if got := strings.Join(e.Buf.Lines, "\n"); got != "word word\nword " {
			t.Fatalf("undo = %q", got)
		}
	}
}
