package ui

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/gdamore/tcell/v3"
)

func autoClosingEditor(t *testing.T, text, filename string, col int) *EditorPaneWidget {
	t.Helper()
	e := bracketEditor(strings.Split(text, "\n"), 0, col)
	e.Selection = &selection.Selection{}
	e.Undo = &undo.UndoStack{}
	e.AutoClosing = config.DefaultAutoClosingSettings()
	e.AutoClosing.Brackets, e.AutoClosing.Quotes = true, true
	e.autoClosingPath = filename
	e.Highlighter = highlight.NewForLines(filename, e.Buf.Lines)
	t.Cleanup(func() {
		e.Highlighter.Close()
		if e.autoClosing != nil {
			e.autoClosing.syntax.Close()
		}
	})
	return e
}

func typeAutoClosing(e *EditorPaneWidget, text string) {
	for _, r := range text {
		e.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), 0))
	}
}

func autoClosingKey(e *EditorPaneWidget, key tcell.Key) { e.HandleEvent(tcell.NewEventKey(key, "", 0)) }

func assertAutoClosing(t *testing.T, e *EditorPaneWidget, text string, line, col int) {
	t.Helper()
	if got := strings.Join(e.Buf.Lines, "\n"); got != text {
		t.Fatalf("text = %q, want %q", got, text)
	}
	if e.Cursor.Line != line || e.Cursor.Col != col {
		t.Fatalf("cursor = %d:%d, want %d:%d", e.Cursor.Line, e.Cursor.Col, line, col)
	}
}

func TestAutoClosingSettingsMatrix(t *testing.T) {
	for mask := 0; mask < 64; mask++ {
		s := config.AutoClosingSettings{Brackets: mask&1 != 0, Quotes: mask&2 != 0, SkipClosing: mask&4 != 0, DeletePairs: mask&8 != 0, InStrings: mask&16 != 0, PlainText: mask&32 != 0}
		for _, pair := range []string{"()", "[]", "{}", "''", "\"\""} {
			for _, ctx := range []struct {
				name, text, file string
				col              int
				inString, plain  bool
			}{
				{"code", "", "a.go", 0, false, false},
				{"string", "var s = \"a  b\"", "a.go", 11, true, false},
				{"plain", "", "a.txt", 0, false, true},
			} {
				t.Run(fmt.Sprintf("%02d/%s/%s", mask, pair, ctx.name), func(t *testing.T) {
					enabled := s.Brackets
					if pair[0] == '\'' || pair[0] == '"' {
						enabled = s.Quotes
					}
					enabled = enabled && (!ctx.plain || s.PlainText) && (!ctx.inString || s.InStrings)
					e := autoClosingEditor(t, ctx.text, ctx.file, ctx.col)
					e.AutoClosing = s
					prefix, suffix := string([]rune(ctx.text)[:ctx.col]), string([]rune(ctx.text)[ctx.col:])
					typeAutoClosing(e, pair[:1])
					want := prefix + pair[:1] + suffix
					if enabled {
						want = prefix + pair + suffix
					}
					assertAutoClosing(t, e, want, 0, ctx.col+1)
					if enabled {
						typeAutoClosing(e, pair[1:])
						want = prefix + pair + suffix
						if !s.SkipClosing {
							want = prefix + pair + pair[1:] + suffix
						}
						assertAutoClosing(t, e, want, 0, ctx.col+2)
					}
					e = autoClosingEditor(t, ctx.text, ctx.file, ctx.col)
					e.AutoClosing = s
					typeAutoClosing(e, pair[:1])
					autoClosingKey(e, tcell.KeyBackspace2)
					want = ctx.text
					if enabled && !s.DeletePairs {
						want = prefix + pair[1:] + suffix
					}
					assertAutoClosing(t, e, want, 0, ctx.col)
				})
			}
		}
	}
}

func TestAutoClosingInsertionBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, text  string
		col         int
		typed, want string
	}{
		{"end", "call", 4, "(", "call()"},
		{"space", " ", 0, "[", "[] "},
		{"tab", "\t", 0, "{", "{}\t"},
		{"unicode space", "\u3000", 0, "(", "()\u3000"},
		{"closer", ")", 0, "(", "())"},
		{"letter", "value", 0, "(", "(value"},
		{"number", "1", 0, "[", "[1"},
		{"unicode letter", "界", 0, "{", "{界"},
		{"opening bracket", "(", 0, "[", "[("},
		{"punctuation", ";", 0, "(", "(;"},
		{"single apostrophe", "don", 3, "'", "don'"},
		{"unicode apostrophe", "lété", 4, "'", "lété'"},
		{"quote after digit", "1", 1, "\"", "1\""},
		{"quote after underscore", "_", 1, "\"", "_\""},
		{"escaped quote", `\`, 1, "\"", `\"`},
		{"even escapes", `\\`, 2, "\"", `\\""`},
		{"odd escapes", `\\\`, 3, "'", `\\\'`},
		{"backtick", "", 0, "`", "`"},
		{"angle bracket", "", 0, "<", "<"},
		{"unicode prefix", "界🙂", 2, "[", "界🙂[]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := autoClosingEditor(t, tc.text, "a.go", tc.col)
			typeAutoClosing(e, tc.typed)
			assertAutoClosing(t, e, tc.want, 0, tc.col+1)
		})
	}
}

func TestAutoClosingSequences(t *testing.T) {
	for _, tc := range []struct {
		name, typed, want string
		backs, col        int
	}{
		{"nested", "([{", "([{}])", 0, 3},
		{"nested skips", "([{}])", "([{}])", 0, 6},
		{"delete nested", "([{", "", 3, 0},
		{"delete content then pair", "(ab", "", 3, 0},
		{"skip after content", "(abc)", "(abc)", 0, 5},
		{"repeat closer", "())", "())", 0, 3},
		{"adjacent", "()[]{}", "()[]{}", 0, 6},
		{"quote skip", "\"abc\"", "\"abc\"", 0, 5},
		{"escaped generated quote", "\"a\\\"", "\"a\\\"\"", 0, 4},
		{"skip after escaped quote", "\"a\\\"b\"", "\"a\\\"b\"", 0, 6},
		{"quote deletion", "'abc", "", 4, 0},
		{"bracket after closed string", "\"abc\"(", "\"abc\"()", 0, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := autoClosingEditor(t, "", "a.go", 0)
			typeAutoClosing(e, tc.typed)
			for range tc.backs {
				autoClosingKey(e, tcell.KeyBackspace2)
			}
			assertAutoClosing(t, e, tc.want, 0, tc.col)
		})
	}
}

func TestAutoClosingExistingTextIsLiteral(t *testing.T) {
	for _, pair := range []string{"()", "[]", "{}", "''", "\"\""} {
		t.Run(pair, func(t *testing.T) {
			e := autoClosingEditor(t, pair, "a.go", 1)
			typeAutoClosing(e, pair[1:])
			assertAutoClosing(t, e, pair+pair[1:], 0, 2)
			e = autoClosingEditor(t, pair, "a.go", 1)
			autoClosingKey(e, tcell.KeyBackspace2)
			assertAutoClosing(t, e, pair[1:], 0, 0)
			e = autoClosingEditor(t, "", "a.go", 0)
			e.pasteText(pair)
			e.Cursor.Col = 1
			typeAutoClosing(e, pair[1:])
			assertAutoClosing(t, e, pair+pair[1:], 0, 2)
		})
	}
}

func TestAutoClosingMovementAndMultiline(t *testing.T) {
	e := autoClosingEditor(t, "", "a.go", 0)
	typeAutoClosing(e, "(")
	autoClosingKey(e, tcell.KeyLeft)
	autoClosingKey(e, tcell.KeyRight)
	typeAutoClosing(e, ")")
	assertAutoClosing(t, e, "()", 0, 2)
	e = autoClosingEditor(t, "", "a.go", 0)
	typeAutoClosing(e, "{")
	autoClosingKey(e, tcell.KeyEnter)
	assertAutoClosing(t, e, "{\n    \n}", 1, 4)
	autoClosingKey(e, tcell.KeyDown)
	autoClosingKey(e, tcell.KeyEnd)
	autoClosingKey(e, tcell.KeyLeft)
	typeAutoClosing(e, "}")
	assertAutoClosing(t, e, "{\n    \n}", 2, 1)
	e = autoClosingEditor(t, "", "a.go", 0)
	e.AutoIndent = false
	typeAutoClosing(e, "(")
	autoClosingKey(e, tcell.KeyEnter)
	autoClosingKey(e, tcell.KeyBackspace2)
	autoClosingKey(e, tcell.KeyBackspace2)
	assertAutoClosing(t, e, "", 0, 0)
}

func TestAutoClosingSelectionsAreReplaced(t *testing.T) {
	for _, text := range []string{"hello", "one\ntwo"} {
		e := autoClosingEditor(t, text, "a.go", 0)
		e.Selection.Start(0, 0)
		e.Cursor.Line = len(e.Buf.Lines) - 1
		e.Cursor.Col = len([]rune(e.Buf.Lines[e.Cursor.Line]))
		typeAutoClosing(e, "(")
		assertAutoClosing(t, e, "(", 0, 1)
		pos := e.Undo.Undo(e.Buf)
		if pos == nil || strings.Join(e.Buf.Lines, "\n") != text {
			t.Fatal("selection replacement did not undo in one step")
		}
	}
}

func TestAutoClosingPasteAndDeletion(t *testing.T) {
	e := autoClosingEditor(t, "", "a.go", 0)
	typeAutoClosing(e, "(")
	e.pasteText("界\n🙂")
	typeAutoClosing(e, ")")
	assertAutoClosing(t, e, "(界\n🙂)", 1, 2)
	e = autoClosingEditor(t, "", "a.go", 0)
	typeAutoClosing(e, "(")
	autoClosingKey(e, tcell.KeyDelete)
	typeAutoClosing(e, ")")
	assertAutoClosing(t, e, "()", 0, 2)
	e = autoClosingEditor(t, "", "a.go", 0)
	typeAutoClosing(e, "(")
	e.Selection.Start(0, 0)
	autoClosingKey(e, tcell.KeyDelete)
	typeAutoClosing(e, ")")
	assertAutoClosing(t, e, "))", 0, 1)
}

func TestAutoClosingUndoRedo(t *testing.T) {
	for _, pair := range []string{"()", "[]", "{}", "''", "\"\""} {
		t.Run(pair, func(t *testing.T) {
			e := autoClosingEditor(t, "", "a.go", 0)
			typeAutoClosing(e, pair[:1])
			p := e.Undo.Undo(e.Buf)
			if p == nil {
				t.Fatal("no undo")
			}
			e.Cursor.Line, e.Cursor.Col = p.Line, p.Col
			assertAutoClosing(t, e, "", 0, 0)
			p = e.Undo.Redo(e.Buf)
			if p == nil {
				t.Fatal("no redo")
			}
			e.Cursor.Line, e.Cursor.Col = p.Line, p.Col
			assertAutoClosing(t, e, pair, 0, 1)
			e = autoClosingEditor(t, "", "a.go", 0)
			typeAutoClosing(e, pair[:1])
			autoClosingKey(e, tcell.KeyBackspace2)
			p = e.Undo.Undo(e.Buf)
			if p == nil || strings.Join(e.Buf.Lines, "\n") != pair {
				t.Fatal("pair deletion did not undo in one step")
			}
			e.Undo.Redo(e.Buf)
			if e.Buf.Lines[0] != "" {
				t.Fatal("pair deletion did not redo in one step")
			}
		})
	}
}

func TestAutoClosingInvalidatesExternalEdits(t *testing.T) {
	for _, change := range []struct {
		name  string
		apply func(*EditorPaneWidget)
	}{
		{"reload", func(e *EditorPaneWidget) { e.Buf.SetLines([]string{"()"}) }},
		{"replace", func(e *EditorPaneWidget) {
			e.ExecCommand(&undo.ReplaceLinesCommand{Start: 0, OldLines: []string{"()"}, NewLines: []string{"()"}})
		}},
		{"buffer switch", func(e *EditorPaneWidget) { e.Buf = &buffer.Buffer{Lines: []string{"()"}} }},
		{"undo redo", func(e *EditorPaneWidget) { e.Undo.Undo(e.Buf); e.Undo.Redo(e.Buf) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			e := autoClosingEditor(t, "", "a.go", 0)
			typeAutoClosing(e, "(")
			change.apply(e)
			typeAutoClosing(e, ")")
			assertAutoClosing(t, e, "())", 0, 2)
		})
	}
}

func TestAutoClosingLiveSettingsAndReadOnly(t *testing.T) {
	e := autoClosingEditor(t, "", "a.go", 0)
	e.ReadOnly = true
	typeAutoClosing(e, "(")
	assertAutoClosing(t, e, "", 0, 0)
	e.ReadOnly = false
	typeAutoClosing(e, "(")
	e.AutoClosing.Brackets = false
	typeAutoClosing(e, ")")
	assertAutoClosing(t, e, "())", 0, 2)
	e = autoClosingEditor(t, "", "a.go", 0)
	typeAutoClosing(e, "(")
	e.AutoClosing.DeletePairs = false
	autoClosingKey(e, tcell.KeyBackspace2)
	assertAutoClosing(t, e, ")", 0, 0)
	e = autoClosingEditor(t, "", "a.go", 0)
	typeAutoClosing(e, "(")
	autoClosingKey(e, tcell.KeyLeft)
	typeAutoClosing(e, "[")
	autoClosingKey(e, tcell.KeyBackspace2)
	autoClosingKey(e, tcell.KeyRight)
	typeAutoClosing(e, ")")
	assertAutoClosing(t, e, "()", 0, 2)
}

func TestAutoClosingMultiCursor(t *testing.T) {
	for _, sameLine := range []bool{false, true} {
		t.Run(fmt.Sprint(sameLine), func(t *testing.T) {
			text := " \n "
			positions := [][2]int{{0, 0}, {1, 0}}
			want := "() \n() "
			if sameLine {
				text = "  "
				positions = [][2]int{{0, 0}, {0, 1}}
				want = "() () "
			}
			e := newMultiCursorEditor(strings.Split(text, "\n"), positions)
			e.Highlighter = highlight.New("a.go")
			t.Cleanup(e.Highlighter.Close)
			e.AutoClosing.Brackets = true
			e.Undo = &undo.UndoStack{}
			typeAutoClosing(e, "(")
			if got := strings.Join(e.Buf.Lines, "\n"); got != want {
				t.Fatalf("pairs = %q, want %q", got, want)
			}
			typeAutoClosing(e, ")")
			if got := strings.Join(e.Buf.Lines, "\n"); got != want {
				t.Fatalf("skip = %q, want %q", got, want)
			}
			e = newMultiCursorEditor(strings.Split(text, "\n"), positions)
			e.Highlighter = highlight.New("a.go")
			t.Cleanup(e.Highlighter.Close)
			e.AutoClosing.Brackets = true
			e.Undo = &undo.UndoStack{}
			typeAutoClosing(e, "(")
			autoClosingKey(e, tcell.KeyBackspace2)
			if got := strings.Join(e.Buf.Lines, "\n"); got != text {
				t.Fatalf("delete = %q, want %q", got, text)
			}
			if p := e.Undo.Undo(e.Buf); p == nil || strings.Join(e.Buf.Lines, "\n") != want {
				t.Fatal("multicursor pair deletion did not undo as one step")
			}
		})
	}
}

func TestAutoClosingTabSwitchAndSyntaxDisabled(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")
	for _, path := range []string{a, b} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g := NewEditorGroupWidget(nil, 4, true, "compact")
	g.Editor.AutoClosing.Brackets = true
	g.OpenFile(a)
	g.CommitActiveTab()
	typeAutoClosing(g.Editor, "(")
	g.OpenFile(b)
	g.CommitActiveTab()
	typeAutoClosing(g.Editor, "[")
	g.SwitchToTabByPath(a)
	typeAutoClosing(g.Editor, ")")
	assertAutoClosing(t, g.Editor, "()", 0, 2)
	g.SwitchToTabByPath(b)
	autoClosingKey(g.Editor, tcell.KeyBackspace2)
	assertAutoClosing(t, g.Editor, "", 0, 0)
	g.CloseAllTabs()
	e := autoClosingEditor(t, "var s = \"a  b\"", "a.go", 11)
	e.Highlighter.Close()
	e.Highlighter = nil
	typeAutoClosing(e, "(")
	assertAutoClosing(t, e, "var s = \"a ( b\"", 0, 12)
	e = autoClosingEditor(t, "", "a.go", 0)
	e.Highlighter.Close()
	e.Highlighter = nil
	typeAutoClosing(e, "(")
	assertAutoClosing(t, e, "()", 0, 1)
}

func TestAutoClosingMultiCursorMixedContextsAndSettings(t *testing.T) {
	for _, pair := range []string{"()", "[]", "{}", "''", "\"\""} {
		for _, skip := range []bool{false, true} {
			for _, del := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/skip=%v/delete=%v", pair, skip, del), func(t *testing.T) {
					e := newMultiCursorEditor([]string{"", "value"}, [][2]int{{0, 0}, {1, 0}})
					e.Highlighter = highlight.New("a.go")
					t.Cleanup(e.Highlighter.Close)
					e.Undo = &undo.UndoStack{}
					e.AutoClosing = config.AutoClosingSettings{Brackets: true, Quotes: true, SkipClosing: skip, DeletePairs: del}
					typeAutoClosing(e, pair[:1])
					if e.Buf.Lines[0] != pair || e.Buf.Lines[1] != pair[:1]+"value" {
						t.Fatalf("mixed insertion = %q", e.Buf.Lines)
					}
					if del {
						autoClosingKey(e, tcell.KeyBackspace2)
						if e.Buf.Lines[0] != "" || e.Buf.Lines[1] != "value" {
							t.Fatalf("mixed deletion = %q", e.Buf.Lines)
						}
					} else {
						typeAutoClosing(e, pair[1:])
						want := pair
						if !skip {
							want += pair[1:]
						}
						if e.Buf.Lines[0] != want || e.Buf.Lines[1] != pair+"value" {
							t.Fatalf("mixed overtype = %q", e.Buf.Lines)
						}
					}
				})
			}
		}
	}
}

func TestAutoClosingSkipDoesNotDirtyBufferOrCreateUndo(t *testing.T) {
	e := newMultiCursorEditor([]string{"", ""}, [][2]int{{0, 0}, {1, 0}})
	e.Highlighter = highlight.New("a.go")
	defer e.Highlighter.Close()
	e.Undo = &undo.UndoStack{}
	e.AutoClosing.Brackets = true
	typeAutoClosing(e, "(")
	e.FlushOnChange()
	e.Buf.Dirty = false
	version := e.Buf.Version()
	changes := 0
	e.OnChange = func() { changes++ }
	typeAutoClosing(e, ")")
	e.FlushOnChange()
	if e.Buf.Version() != version || e.Buf.Dirty || changes != 0 {
		t.Fatal("skipping changed the document")
	}
	e.Undo.Undo(e.Buf)
	if strings.Join(e.Buf.Lines, "\n") != "\n" {
		t.Fatal("skipping created an extra undo step")
	}
}

func TestAutoClosingMarkersFollowEditing(t *testing.T) {
	for seed := int64(0); seed < 12; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			e := autoClosingEditor(t, "", "a.go", 0)
			e.AutoClosing.InStrings = true
			for step := 0; step < 300; step++ {
				switch rng.Intn(12) {
				case 0:
					autoClosingKey(e, tcell.KeyLeft)
				case 1:
					autoClosingKey(e, tcell.KeyRight)
				case 2:
					autoClosingKey(e, tcell.KeyBackspace2)
				case 3:
					autoClosingKey(e, tcell.KeyDelete)
				case 4:
					autoClosingKey(e, tcell.KeyEnter)
				case 5:
					e.pasteText("界\n🙂")
				default:
					chars := []rune("([{)]}'\"abc界\\")
					typeAutoClosing(e, string(chars[rng.Intn(len(chars))]))
				}
				s := e.syncAutoClosing()
				for _, p := range s.pairs {
					for _, point := range []struct {
						pos  selection.Position
						char rune
					}{{p.open, p.opener}, {p.close, p.closer}} {
						if point.pos.Line < 0 || point.pos.Line >= len(e.Buf.Lines) {
							t.Fatalf("step %d: pair line moved outside buffer", step)
						}
						runes := []rune(e.Buf.Lines[point.pos.Line])
						if point.pos.Col < 0 || point.pos.Col >= len(runes) || runes[point.pos.Col] != point.char {
							t.Fatalf("step %d: tracked %q at %+v no longer names that character", step, point.char, point.pos)
						}
					}
				}
			}
		})
	}
}
