package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestMultiCursorEditSelectionContainingCursor(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		key        tcell.Key
		text       string
	}{
		{"type", "head\ntail~", tcell.KeyRune, "~"},
		{"tab", "head\ntail\t", tcell.KeyTab, ""},
		{"enter", "head\ntail\n", tcell.KeyEnter, ""},
		{"backspace", "head\ntail", tcell.KeyBackspace2, ""},
		{"delete", "head\ntail", tcell.KeyDelete, ""},
	} {
		for _, reversed := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/forward", true: "/reversed"}[reversed], func(t *testing.T) {
				const original = "head\ntail\ninside\n"
				e := autoClosingEditor(t, original, "plain.txt", 0)
				e.AutoIndent = false
				e.UseTabs = true
				e.Cursor.Line, e.Cursor.Col = 3, 0
				e.Selection.Start(1, 4)
				if reversed {
					e.Cursor.Line, e.Cursor.Col = 1, 4
					e.Selection.Start(3, 0)
				}
				e.ensureMulti()
				e.Multi.Add(2, 0)
				e.HandleEvent(tcell.NewEventKey(tc.key, tc.text, 0))
				if got := strings.Join(e.Buf.Lines, "\n"); got != tc.want {
					t.Fatalf("text = %q, want %q", got, tc.want)
				}
				if len(e.Multi.Cursors) != 1 {
					t.Fatalf("overlapping cursors survived edit: %+v", e.Multi.Cursors)
				}
				cs := e.Multi.PrimaryCursor()
				if cs.Line != e.Cursor.Line || cs.Col != e.Cursor.Col || cs.Sel.Active {
					t.Fatalf("cursor state out of sync: %+v, %+v", cs, e.Cursor)
				}
				e.Undo.Undo(e.Buf)
				if got := strings.Join(e.Buf.Lines, "\n"); got != original {
					t.Fatalf("undo = %q, want %q", got, original)
				}
				e.Undo.Redo(e.Buf)
				if got := strings.Join(e.Buf.Lines, "\n"); got != tc.want {
					t.Fatalf("redo = %q, want %q", got, tc.want)
				}
				typeAutoClosing(e, "\"")
			})
		}
	}
}
