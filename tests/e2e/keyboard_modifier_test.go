package e2e

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

func TestKeyboardModifierClipboard(t *testing.T) {
	clipboard.DisableSystem()
	for _, mode := range []string{"control", "control+super", "super"} {
		for _, super := range []bool{false, true} {
			name := mode + "/ctrl"
			if super {
				name = mode + "/super"
			}
			t.Run(name, func(t *testing.T) {
				h := newTestHarness(t, 80, 24)
				defer h.stop()
				h.exec("file.new")
				h.typeText("hello 界")
				s := *h.app.Settings
				s.Keyboard.ModifierKey = mode
				h.app.ApplySettings(s)

				tty := vt.NewMockTerm()
				screen, err := tcell.NewTerminfoScreenFromTty(tty, tcell.OptNegotiation(false))
				if err != nil {
					t.Fatal(err)
				}
				if err := screen.Init(); err != nil {
					t.Fatal(err)
				}
				defer screen.Fini()
				press := func(ch byte) {
					t.Helper()
					sequence := []byte{ch - 'a' + 1}
					if super {
						sequence = []byte("\x1b[" + strconv.Itoa(int(ch)) + ";9u")
					}
					tty.SendRaw(sequence)
					timer := time.NewTimer(2 * time.Second)
					defer timer.Stop()
					for {
						select {
						case ev := <-screen.EventQ():
							if _, ok := ev.(*tcell.EventKey); ok {
								h.app.Root.HandleEvent(ev)
								h.redraw()
								return
							}
						case <-timer.C:
							t.Fatal("no key decoded")
						}
					}
				}
				press('a')
				ed := h.app.EditorGroup.Editor
				want := mode == "control+super" || (mode == "super") == super
				if ed.Selection.Active != want {
					t.Fatalf("select all active=%v, want %v", ed.Selection.Active, want)
				}
				if !want {
					return
				}
				press('c')
				h.pressRune('x')
				press('v')
				if got := strings.Join(ed.Buf.Lines, "\n"); got != "xhello 界" {
					t.Fatalf("select/copy/replace/paste = %q", got)
				}
			})
		}
	}
}
