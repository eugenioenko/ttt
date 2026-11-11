package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func (h *testHarness) mouseMove(x, y int) {
	h.t.Helper()
	ev := tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone)
	h.app.TrackMouseHover(ev)
	h.app.Root.HandleEvent(ev)
	h.redraw()
}

func TestLongTabNameIsCutAndShownOnHover(t *testing.T) {
	h := newTestHarness(t, 150, 30)
	defer h.stop()

	name := "notes_*draft*_with_a_really_long_name_for_the_tab_bar.txt"
	path := filepath.Join(h.dir, name)
	if err := os.WriteFile(path, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	if err := h.app.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	h.redraw()

	tabRect := h.app.EditorGroup.TabBar.GetRect()
	tabY := tabRect.Y + 1
	cut := "notes_*draft*_with_a_really_lon…"
	row := h.screenRow(tabY)
	if !strings.Contains(row, cut) {
		t.Fatalf("tab row %q does not contain cut name %q", row, cut)
	}
	h.assertNotContains(name)

	cutX := displayColumnOf(row, cut)
	h.mouseMove(cutX+2, tabY)
	h.assertContains(name)

	h.mouseMove(cutX+4, tabY)
	h.assertContains(name)

	h.mouseMove(displayColumnOf(row, "alpha.txt")+1, tabY)
	h.assertNotContains(name)

	h.mouseMove(cutX+2, tabY)
	h.assertContains(name)

	h.mouseMove(tabRect.X+tabRect.W/2, tabRect.Y+tabRect.H+8)
	h.assertNotContains(name)
}
