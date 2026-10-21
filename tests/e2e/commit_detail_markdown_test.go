package e2e

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestCommitDetailMarkdownScrollAndCopyAcrossPages(t *testing.T) {
	clipboard.DisableSystem()
	clipboard.Set("sentinel")
	h := newTestHarness(t, 76, 18)
	defer h.stop()
	visible := "A界B語C " + strings.Repeat("alpha beta gamma delta ", 35) + "COPY-END"
	d := ui.NewCommitDetailWidget(h.dir, "ref", "abcdef0", false)
	d.SetDetail("# Heading\n\n## Subheading\n\n**A界B語C** "+strings.Repeat("alpha beta gamma delta ", 35)+"COPY-END", nil, "")
	h.app.EditorGroup.OpenPluginTab("markdown-scroll", "Markdown review", d)
	h.app.Root.SetFocus(h.app.EditorGroup)
	h.redraw()
	find := func(label string) (int, int) {
		t.Helper()
		for y, row := range strings.Split(h.screenText(), "\n") {
			if x := displayColumnOf(row, label); x >= 0 {
				return x, y
			}
		}
		t.Fatalf("screen missing %q:\n%s", label, h.screenText())
		return 0, 0
	}
	h.assertContains("# Heading")
	h.assertContains("## Subheading")
	h.assertNotContains("COPY-END")
	h.pressKey(tcell.KeyPgDn, 0)
	if d.TopLine <= 0 {
		t.Fatal("PageDown did not scroll the Markdown document")
	}
	h.pressKey(tcell.KeyPgUp, 0)
	if d.TopLine != 0 {
		t.Fatal("PageUp did not return to the first page")
	}
	x, y := find("A界B語C")
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.WheelDown, 0))
	h.redraw()
	if d.TopLine <= 0 {
		t.Fatal("wheel did not scroll Markdown")
	}
	h.pressKey(tcell.KeyHome, 0)
	r := d.GetRect()
	h.app.Root.HandleEvent(tcell.NewEventMouse(r.X+r.W-1, r.Y, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(r.X+r.W-1, r.Y+r.H-1, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(r.X+r.W-1, r.Y+r.H-1, tcell.ButtonNone, 0))
	h.redraw()
	h.assertContains("COPY-END")
	if d.OwnsPointerCapture() {
		t.Fatal("vertical scrollbar did not release pointer capture")
	}
	h.pressKey(tcell.KeyHome, 0)
	x, y = find("A界B語C")
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1|tcell.WheelDown, 0))
	h.redraw()
	h.pressKey(tcell.KeyEnd, 0)
	x, y = find("COPY-END")
	x += len("COPY-END")
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, 0))
	h.pressCtrl(tcell.KeyCtrlC)
	if got := clipboard.Get(); got != visible {
		t.Fatalf("Ctrl+C across scrolled soft wraps copied %q, want %q", got, visible)
	}
	h.pressCtrl(tcell.KeyCtrlN)
	h.pressCtrl(tcell.KeyCtrlV)
	if got := strings.Join(h.app.EditorGroup.Editor.Buf.Lines, "\n"); got != visible {
		t.Fatalf("Ctrl+V did not restore the selected Markdown text: %q", got)
	}
}
