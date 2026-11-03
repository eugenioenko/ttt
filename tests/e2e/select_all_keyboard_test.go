package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

func TestSelectAllKeyboardDocument(t *testing.T) {
	for _, tc := range []struct{ binding, sequence string }{
		{"ctrl+a", "\x01"},
		{"super+a", "\x1b[97;9u"},
		{"super+left", "\x1b[1;9D"},
		{"ctrl+super+a", "\x1b[97;13u"},
	} {
		t.Run(tc.binding, func(t *testing.T) {
			h := newTestHarness(t, 80, 30)
			defer h.stop()
			if tc.binding != "ctrl+a" && tc.binding != "super+a" {
				bindings := []config.KeyBinding{{Key: tc.binding, Command: "editor.selectAll"}}
				if err := config.ParseKeyBindings(bindings); err != nil {
					t.Fatal(err)
				}
				app.BindKeys(h.app.Root, h.reg, bindings)
			}
			text := strings.Repeat("body { color: red; }\n", 7999) + "/* 界é */\n"
			file := filepath.Join(h.dir, "document.txt")
			if err := os.WriteFile(file, []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
			h.app.EditorGroup.OpenFile(file)
			h.redraw()

			terminal := vt.NewMockTerm()
			screen, err := tcell.NewTerminfoScreenFromTty(terminal, tcell.OptNegotiation(false))
			if err != nil {
				t.Fatal(err)
			}
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			defer screen.Fini()
			terminal.SendRaw([]byte(tc.sequence))
			timer := time.NewTimer(2 * time.Second)
			defer timer.Stop()
			for {
				select {
				case event := <-screen.EventQ():
					if _, ok := event.(*tcell.EventKey); !ok {
						continue
					}
					h.app.Root.HandleEvent(event)
					editor := h.app.EditorGroup.Editor
					if !editor.Selection.Active {
						t.Fatal("select-all key did not activate editor selection")
					}
					if got := editor.Selection.Text(editor.Buf.Lines, editor.Cursor.Line, editor.Cursor.Col); got != text {
						t.Fatalf("selected %d bytes, want entire %d-byte document", len(got), len(text))
					}
					h.pressRune('x')
					if got := strings.Join(editor.Buf.Lines, "\n"); got != "x" {
						t.Fatalf("typing did not replace the document: %d bytes remain", len(got))
					}
					return
				case <-timer.C:
					t.Fatal("no key event decoded")
				}
			}
		})
	}
}
