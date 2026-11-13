package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/gdamore/tcell/v3"
)

func newGoalGroup(t *testing.T, text string) *EditorGroupWidget {
	t.Helper()
	path := filepath.Join(t.TempDir(), "goal.txt")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	if err := g.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	g.Editor.Cursor.Col = 4
	pressWrapKey(g.Editor, tcell.KeyDown, 0)
	assertCursor(t, g.Editor, 1, 2)
	return g
}

func TestVerticalGoalResetsAfterSelectAll(t *testing.T) {
	g := newGoalGroup(t, "abcdefghij\nxy")
	g.SelectAll()
	assertCursor(t, g.Editor, 1, 2)
	pressWrapKey(g.Editor, tcell.KeyUp, 0)
	assertCursor(t, g.Editor, 0, 2)
}

func TestVerticalGoalResetsAfterFind(t *testing.T) {
	for _, tc := range []struct {
		name   string
		active int
		find   func(*EditorGroupWidget)
	}{
		{"next", 0, (*EditorGroupWidget).FindNext},
		{"previous", 2, (*EditorGroupWidget).FindPrev},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newGoalGroup(t, "abcdefghij\nxy\nabcdefghij")
			opts := SearchOptions{UseRegex: true}
			matches, err := FindInLines(g.Editor.Buf.Lines, "$", opts)
			if err != nil {
				t.Fatal(err)
			}
			g.SetFindSearch("$", matches, opts)
			g.SetSearchActive(tc.active)
			tc.find(g)
			assertCursor(t, g.Editor, 1, 2)
			pressWrapKey(g.Editor, tcell.KeyUp, 0)
			assertCursor(t, g.Editor, 0, 2)
		})
	}
}

func TestVerticalGoalResetsAfterBracketRoundtrip(t *testing.T) {
	e := newWrapEditor(5, "aaaaa", "abc(中文)", "zzzzz")
	e.Cursor.Col = 4
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 3)
	e.GoToMatchingBracket()
	assertCursor(t, e, 1, 6)
	e.GoToMatchingBracket()
	assertCursor(t, e, 1, 3)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 3)
}

func TestVerticalGoalSurvivesUnsuccessfulNavigation(t *testing.T) {
	for name, navigate := range map[string]func(*EditorGroupWidget){
		"bracket":  (*EditorGroupWidget).GoToMatchingBracket,
		"next":     (*EditorGroupWidget).FindNext,
		"previous": (*EditorGroupWidget).FindPrev,
	} {
		t.Run(name, func(t *testing.T) {
			g := newGoalGroup(t, "abcdefghij\nxy\nabcdefghij")
			g.SetFindSearch("missing", nil, SearchOptions{})
			navigate(g)
			assertCursor(t, g.Editor, 1, 2)
			pressWrapKey(g.Editor, tcell.KeyUp, 0)
			assertCursor(t, g.Editor, 0, 4)
		})
	}
}

func TestVerticalGoalAcrossShortLines(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyDown, tcell.KeyUp, tcell.KeyPgDn, tcell.KeyPgUp} {
		for _, mods := range []tcell.ModMask{0, tcell.ModShift} {
			t.Run(tcell.NewEventKey(key, "", mods).Name(), func(t *testing.T) {
				e := newEditorWithLines("aaaaaaaaaaaaaaaaaaaa", "short", "bbbbbbbbbbbbbbbbbbbb")
				e.Viewport.Height = 1
				e.Selection = &selection.Selection{}
				e.Cursor.Col = 12
				up := key == tcell.KeyUp || key == tcell.KeyPgUp
				if up {
					e.Cursor.Line = 2
				}
				pressWrapKey(e, key, mods)
				assertCursor(t, e, 1, 5)
				pressWrapKey(e, key, mods)
				line := 2
				if up {
					line = 0
				}
				assertCursor(t, e, line, 12)
				if mods == tcell.ModShift {
					anchor := e.Selection.Anchor
					if !e.Selection.Active || anchor.Col != 12 || anchor.Line != 2-line {
						t.Fatalf("selection anchor changed: %+v", e.Selection)
					}
				}
			})
		}
	}
}

func TestVerticalGoalResets(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		for name, reset := range map[string]func(*EditorPaneWidget){
			"horizontal roundtrip": func(e *EditorPaneWidget) {
				pressWrapKey(e, tcell.KeyLeft, 0)
				pressWrapKey(e, tcell.KeyRight, 0)
			},
			"click at same position": func(e *EditorPaneWidget) {
				renderRows(e)
				clickAt(e, e.CursorX, e.CursorY)
			},
			"edit roundtrip": func(e *EditorPaneWidget) {
				e.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "z", 0))
				pressWrapKey(e, tcell.KeyBackspace2, 0)
			},
			"buffer edit": func(e *EditorPaneWidget) { e.Buf.SetLine(0, "other text") },
			"buffer switch": func(e *EditorPaneWidget) {
				e.Buf = newEditorWithLines("abcdefghij", "xy", "abcdefghij").Buf
			},
			"end at same position": func(e *EditorPaneWidget) { e.MoveToLineEdge(true, false) },
			"word wrap change":     func(e *EditorPaneWidget) { e.WordWrap = !e.WordWrap },
			"tab width change":     func(e *EditorPaneWidget) { e.TabSize = 8 },
		} {
			t.Run(name+map[bool]string{false: "/nowrap", true: "/wrap"}[wrap], func(t *testing.T) {
				e := newRowsEditor([]string{"abcdefghij", "xy", "abcdefghij"}, 30, 8)
				e.WordWrap = wrap
				renderRows(e)
				e.Cursor.Col = 4
				pressWrapKey(e, tcell.KeyDown, 0)
				assertCursor(t, e, 1, 2)
				reset(e)
				assertCursor(t, e, 1, 2)
				pressWrapKey(e, tcell.KeyDown, 0)
				assertCursor(t, e, 2, 2)
			})
		}
	}
}

func TestVerticalGoalAcrossFold(t *testing.T) {
	e := newEditorWithLines("abcdefghij", "x", "hidden", "abcdefghij")
	e.Folds = fold.NewState()
	e.Folds.SetRanges([]fold.Range{{StartLine: 1, EndLine: 2}})
	e.Folds.Toggle(1)
	e.Cursor.Col = 4
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 1, 1)
	pressWrapKey(e, tcell.KeyDown, 0)
	assertCursor(t, e, 3, 4)
	pressWrapKey(e, tcell.KeyUp, 0)
	pressWrapKey(e, tcell.KeyUp, 0)
	assertCursor(t, e, 0, 4)
}

func TestVerticalGoalPageKeepsBufferLineLanding(t *testing.T) {
	e := newEditorWithLines("abcdefghij", "x", "abcdefghij")
	e.Viewport.Height = 1
	e.setPhantoms(map[int]int{1: 3})
	e.Cursor.Col = 4
	pressWrapKey(e, tcell.KeyPgDn, 0)
	assertCursor(t, e, 1, 1)
	pressWrapKey(e, tcell.KeyPgDn, 0)
	assertCursor(t, e, 2, 4)
	pressWrapKey(e, tcell.KeyPgUp, 0)
	pressWrapKey(e, tcell.KeyPgUp, 0)
	assertCursor(t, e, 0, 4)
}

func TestVerticalGoalAcrossMixedMoves(t *testing.T) {
	e := newEditorWithLines("abcdefghij", "xy", "", "abcdefghij")
	e.Viewport.Height = 1
	e.Cursor.Col = 4
	pressWrapKey(e, tcell.KeyDown, 0)
	pressWrapKey(e, tcell.KeyPgDn, 0)
	e.ScrollRows(1)
	assertCursor(t, e, 3, 4)
	pressWrapKey(e, tcell.KeyDown, 0)
	pressWrapKey(e, tcell.KeyPgUp, 0)
	pressWrapKey(e, tcell.KeyUp, 0)
	e.ScrollRows(-1)
	assertCursor(t, e, 0, 4)
}

func TestMultiCursorVerticalGoals(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "nowrap", true: "wrap"}[wrap], func(t *testing.T) {
			e := newEditorWithLines("abcdefghij", "xy", "abcdefghij", "abcdefghij", "xy", "abcdefghij")
			e.WordWrap, e.Viewport.Width = wrap, 20
			e.Cursor.Col = 4
			e.Multi = &multicursor.MultiCursor{Cursors: []multicursor.CursorState{{Line: 0, Col: 4}, {Line: 3, Col: 7}}}
			pressWrapKey(e, tcell.KeyDown, 0)
			pressWrapKey(e, tcell.KeyDown, 0)
			assertAddedCursors(t, e, [2]int{2, 4}, [2]int{5, 7})
			pressWrapKey(e, tcell.KeyUp, 0)
			pressWrapKey(e, tcell.KeyLeft, 0)
			pressWrapKey(e, tcell.KeyRight, 0)
			pressWrapKey(e, tcell.KeyUp, 0)
			assertAddedCursors(t, e, [2]int{0, 2}, [2]int{3, 2})
		})
	}
}

func TestVerticalGoalUsesVisualColumns(t *testing.T) {
	for _, tc := range []struct {
		name, middle string
		goal, col    int
	}{
		{"fullwidth boundary", "中文abcdef", 4, 2},
		{"inside fullwidth rune", "中文abcdef", 3, 1},
		{"tab boundary", "\tabcdef", 4, 1},
		{"inside tab", "\tabcdef", 3, 0},
		{"empty line", "", 4, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEditorWithLines("abcdefghij", tc.middle, "abcdefghij")
			e.TabSize = 4
			e.Cursor.Col = tc.goal
			pressWrapKey(e, tcell.KeyDown, 0)
			assertCursor(t, e, 1, tc.col)
			pressWrapKey(e, tcell.KeyDown, 0)
			assertCursor(t, e, 2, tc.goal)
			pressWrapKey(e, tcell.KeyUp, 0)
			pressWrapKey(e, tcell.KeyUp, 0)
			assertCursor(t, e, 0, tc.goal)
		})
	}
}
