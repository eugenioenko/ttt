package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestSelectNextOccurrenceUsesCurrentSelection(t *testing.T) {
	for _, method := range []string{"keyboard", "mouse", "another tab"} {
		t.Run(method, func(t *testing.T) {
			h := newTestHarness(t, 100, 30)
			defer h.stop()

			const matches = "const alpha = 1;\nconst beta = 2;\nconst gamma = 3;"
			path := filepath.Join(h.dir, "words.txt")
			if err := os.WriteFile(path, []byte("unique\n"+matches), 0644); err != nil {
				t.Fatal(err)
			}
			h.app.EditorGroup.OpenFile(path)
			h.app.FocusEditor()
			h.redraw()
			h.pressCtrl(tcell.KeyCtrlD)

			editor := h.app.EditorGroup.Editor
			if got := editor.Selection.Text(editor.Buf.Lines, editor.Cursor.Line, editor.Cursor.Col); got != "unique" {
				t.Fatalf("initial selection = %q, want unique", got)
			}

			firstLine := 1
			switch method {
			case "another tab":
				other := filepath.Join(h.dir, "other.txt")
				if err := os.WriteFile(other, []byte(matches), 0644); err != nil {
					t.Fatal(err)
				}
				h.app.EditorGroup.OpenFile(other)
				h.redraw()
				firstLine = 0
			case "keyboard":
				h.pressKey(tcell.KeyDown, tcell.ModNone)
				h.pressKey(tcell.KeyHome, tcell.ModNone)
			}

			if method == "mouse" {
				r := editor.GetRect()
				x, y := r.X+editor.GutterWidth(), r.Y+firstLine
				h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
				h.app.Root.HandleEvent(tcell.NewEventMouse(x+5, y, tcell.Button1, tcell.ModNone))
				h.app.Root.HandleEvent(tcell.NewEventMouse(x+5, y, tcell.ButtonNone, tcell.ModNone))
				h.redraw()
			} else {
				for range 5 {
					h.pressKey(tcell.KeyRight, tcell.ModShift)
				}
			}
			if got := editor.Selection.Text(editor.Buf.Lines, editor.Cursor.Line, editor.Cursor.Col); got != "const" {
				t.Fatalf("new selection = %q, want const", got)
			}

			for _, want := range []int{2, 3, 3} {
				h.pressCtrl(tcell.KeyCtrlD)
				if got := h.app.EditorGroup.MultiCursorCount(); got != want {
					t.Fatalf("cursor count = %d, want %d", got, want)
				}
				for i, c := range editor.Multi.Cursors {
					if !c.Sel.Active || c.Line != firstLine+i || c.Col != 5 || c.Sel.Anchor.Line != c.Line || c.Sel.Anchor.Col != 0 {
						t.Fatalf("cursor %d = %+v, want selected const on line %d", i, c, firstLine+i)
					}
				}
			}
		})
	}
}
