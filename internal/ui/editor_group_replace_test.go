package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/undo"
)

func TestEditorGroupReplaceIsOneUndoStep(t *testing.T) {
	for _, tc := range []struct {
		name, text, query, replacement, want string
		all                                  bool
	}{
		{"all matches", "foo Bar foo\nfoobar FOO\nhello 世界 世界\nline4 foo", "foo", "X", "X Bar X\nXbar X\nhello 世界 世界\nline4 X", true},
		{"all unicode", "前 世界 世界\n世界 後", "世界", "你好呀", "前 你好呀 你好呀\n你好呀 後", true},
		{"all delete", "foo foo\nfoo", "foo", "", " \n", true},
		{"single match", "foo foo", "foo", "longer", "longer foo", false},
		{"single delete", "foo foo", "foo", "", " foo", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewEditorGroupWidget(nil, 4, false, "extended")
			g.NewFile()
			t.Cleanup(g.CloseAllTabs)
			e := g.Editor
			e.Buf.SetText(tc.text)
			checkText := func(want string) {
				t.Helper()
				if got := strings.Join(e.Buf.Lines, "\n"); got != want {
					t.Fatalf("buffer = %q, want %q", got, want)
				}
			}
			appendRune := func(r rune) {
				line := len(e.Buf.Lines) - 1
				e.exec(&undo.InsertRuneCommand{Line: line, Col: len([]rune(e.Buf.Lines[line])), Rune: r})
			}
			appendRune('!')
			if tc.all {
				g.ReplaceAll(tc.query, tc.replacement, SearchOptions{})
			} else {
				matches, err := FindInLines(e.Buf.Lines, tc.query, SearchOptions{})
				if err != nil || len(matches) == 0 {
					t.Fatalf("find matches = %v, error = %v", matches, err)
				}
				g.ReplaceMatch(matches[0], tc.replacement)
			}
			checkText(tc.want + "!")
			appendRune('?')
			g.Undo()
			checkText(tc.want + "!")
			g.Undo()
			checkText(tc.text + "!")
			g.Undo()
			checkText(tc.text)
			if e.Buf.Dirty {
				t.Fatal("undoing all edits did not restore the clean state")
			}
			g.Redo()
			checkText(tc.text + "!")
			g.Redo()
			checkText(tc.want + "!")
			g.Redo()
			checkText(tc.want + "!?")
		})
	}
}

func TestEditorGroupReplaceAllWithoutMatchesPreservesRedo(t *testing.T) {
	for _, query := range []string{"", "missing", "["} {
		t.Run(query, func(t *testing.T) {
			g := NewEditorGroupWidget(nil, 4, false, "extended")
			g.NewFile()
			t.Cleanup(g.CloseAllTabs)
			e := g.Editor
			e.Buf.SetText("foo")
			e.exec(&undo.InsertRuneCommand{Line: 0, Col: 3, Rune: '!'})
			g.Undo()
			g.ReplaceAll(query, "bar", SearchOptions{UseRegex: true})
			if got := strings.Join(e.Buf.Lines, "\n"); got != "foo" || e.Buf.Dirty {
				t.Fatalf("no-match replacement changed buffer: text=%q, dirty=%v", got, e.Buf.Dirty)
			}
			g.Redo()
			if got := strings.Join(e.Buf.Lines, "\n"); got != "foo!" {
				t.Fatalf("redo = %q, want foo!", got)
			}
			g.Undo()
			if got := strings.Join(e.Buf.Lines, "\n"); got != "foo" {
				t.Fatalf("undo = %q, want foo", got)
			}
		})
	}
}
