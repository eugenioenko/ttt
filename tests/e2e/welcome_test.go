package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWelcomeShowsOnFreshStart(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	screen := h.screenText()
	for _, want := range []string{"ttt", "Command palette", "Go to file", "New file", "Quit"} {
		if !strings.Contains(screen, want) {
			t.Errorf("welcome screen missing %q:\n%s", want, screen)
		}
	}
}

func TestWelcomeYieldsToTyping(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	if !strings.Contains(h.screenText(), "Command palette") {
		t.Fatal("welcome screen not showing before typing")
	}
	h.pressRune('x')
	screen := h.screenText()
	if strings.Contains(screen, "Command palette") {
		t.Errorf("welcome screen still showing after typing:\n%s", screen)
	}
	if !strings.Contains(screen, "x") {
		t.Errorf("typed rune missing from screen:\n%s", screen)
	}
}

func TestWelcomeDismissedByNewFile(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	if !strings.Contains(h.screenText(), "Command palette") {
		t.Fatal("welcome screen not showing before new file")
	}
	h.exec("file.new")
	screen := h.screenText()
	if strings.Contains(screen, "Command palette") {
		t.Errorf("welcome screen still showing over the new file tab:\n%s", screen)
	}
	if !strings.Contains(screen, "untitled-2") {
		t.Errorf("new file tab missing:\n%s", screen)
	}
}

func TestWelcomeReturnsWhenAllTabsClosed(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	fp := filepath.Join(h.dir, "w.txt")
	if err := os.WriteFile(fp, []byte("hi\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(fp)
	h.redraw()
	if strings.Contains(h.screenText(), "Command palette") {
		t.Fatal("welcome screen showing over an open file")
	}
	if n := h.app.EditorGroup.TabCount(); n != 1 {
		t.Errorf("opening a file left %d tabs, want 1 (pristine consumed)", n)
	}
	h.app.EditorGroup.CloseTab()
	h.redraw()
	if !strings.Contains(h.screenText(), "Command palette") {
		t.Errorf("welcome screen missing after closing all tabs:\n%s", h.screenText())
	}
}

func TestWelcomeKeepsDirtiedScratchOnOpen(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	h.pressRune('x')
	fp := filepath.Join(h.dir, "w.txt")
	if err := os.WriteFile(fp, []byte("hi\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(fp)
	h.redraw()
	if n := h.app.EditorGroup.TabCount(); n != 2 {
		t.Errorf("opening a file over dirtied scratch left %d tabs, want 2", n)
	}
	if path, _ := h.app.EditorGroup.TabInfo(0); path != "untitled" {
		t.Errorf("scratch tab lost: tab 0 = %q", path)
	}
}
