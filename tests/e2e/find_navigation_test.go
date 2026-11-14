package e2e

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestFindNavigationKeepsCounterAndCursorTogether(t *testing.T) {
	for _, command := range []string{"search.find", "search.replace"} {
		t.Run(command, func(t *testing.T) {
			h := newTestHarness(t, 90, 20)
			defer h.stop()
			h.exec("file.new")
			pasteText(h, pasteKeys("foo Bar foo\nfoobar FOO\nhello\nline4 foo\n"))
			h.exec(command)
			for _, r := range "foo" {
				h.pressRune(r)
			}

			check := func(current, line, col int, counter string) {
				t.Helper()
				h.assertContains(counter)
				editor := h.app.EditorGroup.Editor
				if editor.SearchActive != current || editor.Cursor.Line != line || editor.Cursor.Col != col {
					t.Fatalf("active=%d cursor=%d:%d, want active=%d cursor=%d:%d", editor.SearchActive, editor.Cursor.Line, editor.Cursor.Col, current, line, col)
				}
			}
			check(0, 0, 0, "1/5")
			h.pressKey(tcell.KeyF3, tcell.ModNone)
			check(1, 0, 8, "2/5")
			h.pressKey(tcell.KeyF3, tcell.ModNone)
			check(2, 1, 0, "3/5")
			h.pressKey(tcell.KeyF3, tcell.ModShift)
			check(1, 0, 8, "2/5")
			h.pressKey(tcell.KeyEnter, tcell.ModNone)
			check(2, 1, 0, "3/5")
			h.pressKey(tcell.KeyUp, tcell.ModNone)
			check(1, 0, 8, "2/5")
			h.pressKey(tcell.KeyF3, tcell.ModShift)
			check(0, 0, 0, "1/5")
			h.pressKey(tcell.KeyF3, tcell.ModShift)
			check(4, 3, 6, "5/5")
			h.pressKey(tcell.KeyF3, tcell.ModNone)
			check(0, 0, 0, "1/5")

			if command == "search.replace" {
				h.pressKey(tcell.KeyTab, tcell.ModNone)
				h.pressKey(tcell.KeyF3, tcell.ModNone)
				check(1, 0, 8, "2/5")
				h.pressKey(tcell.KeyF3, tcell.ModShift)
				check(0, 0, 0, "1/5")
			}

			r := h.app.EditorGroup.Editor.GetRect()
			h.click(r.X+5, r.Y+2)
			h.pressKey(tcell.KeyF3, tcell.ModNone)
			check(1, 0, 8, "2/5")
			h.exec("search.findPrev")
			check(0, 0, 0, "1/5")
			if command == "search.find" {
				h.pressKey(tcell.KeyEscape, tcell.ModNone)
				h.pressKey(tcell.KeyF3, tcell.ModNone)
				editor := h.app.EditorGroup.Editor
				if h.app.Root.HasOverlay() || editor.SearchActive != 1 || editor.Cursor.Line != 0 || editor.Cursor.Col != 8 {
					t.Fatalf("find after closing bar: overlay=%v active=%d cursor=%d:%d", h.app.Root.HasOverlay(), editor.SearchActive, editor.Cursor.Line, editor.Cursor.Col)
				}
			}
		})
	}
}

func TestFindNavigationWithZeroOrOneMatch(t *testing.T) {
	for _, command := range []string{"search.find", "search.replace"} {
		for _, query := range []string{"", "missing", "foo"} {
			t.Run(command+"/"+query, func(t *testing.T) {
				h := newTestHarness(t, 90, 20)
				defer h.stop()
				h.exec("file.new")
				pasteText(h, pasteKeys("foo"))
				h.exec(command)
				for _, r := range query {
					h.pressRune(r)
				}
				cursor := *h.app.EditorGroup.Editor.Cursor
				for _, mod := range []tcell.ModMask{tcell.ModNone, tcell.ModShift} {
					h.pressKey(tcell.KeyF3, mod)
					if query == "foo" {
						h.assertContains("1/1")
					} else if query != "" {
						h.assertContains("0/0")
					} else {
						h.assertNotContains("0/0")
					}
					if got := *h.app.EditorGroup.Editor.Cursor; got != cursor {
						t.Fatalf("cursor = %+v, want %+v", got, cursor)
					}
				}
			})
		}
	}
}
