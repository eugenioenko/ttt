package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/gdamore/tcell/v3"
)

func awaitGitGutter(t *testing.T, h *testHarness) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-h.screen.EventQ():
			if intr, ok := ev.(*tcell.EventInterrupt); ok {
				if res, ok := intr.Data().(*app.GitGutterResult); ok {
					h.app.ApplyGitGutterResult(res)
					h.redraw()
					return
				}
			}
		case <-timeout:
			t.Fatal("git gutter result never arrived")
		}
	}
}

func TestOpenChangesEditsInlineDiff(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "code.txt")
	os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	os.WriteFile(path, []byte("one\nTWO\nthree\n"), 0644)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "code.txt", Status: "M"}, false)
	h.redraw()

	if h.app.EditorGroup.ActiveDiffWidget() != nil {
		t.Fatal("working-tree change opened the read-only diff view")
	}
	if !h.app.EditorGroup.IsInlineDiffActive() {
		t.Fatal("inline diff not active")
	}
	h.assertContains("code.txt (diff)")
	h.assertContains("two")
	h.assertContains("TWO")

	h.app.EditorGroup.GoToLine(3)
	h.exec("editor.focus")
	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	h.pressRune('!')
	h.flushOnChange()
	h.app.RequestGitGutterForActiveFile()
	awaitGitGutter(t, h)

	screen := h.screenText()
	if !strings.Contains(screen, "three!") {
		t.Fatalf("edit not rendered:\n%s", screen)
	}
	if strings.Count(screen, "three") != 2 {
		t.Fatalf("expected the old line as a deleted row above the edit:\n%s", screen)
	}
	if got := h.app.EditorGroup.ActiveBuffer().Lines[2]; got != "three!" {
		t.Fatalf("buffer line 3 = %q", got)
	}

	h.exec("file.save")
	data, _ := os.ReadFile(path)
	if string(data) != "one\nTWO\nthree!\n" {
		t.Fatalf("saved content = %q", data)
	}

	h.exec("editor.toggleDiff")
	h.redraw()
	if h.app.EditorGroup.IsInlineDiffActive() {
		t.Fatal("toggle did not leave inline diff")
	}
	if strings.Contains(h.screenText(), "two") {
		t.Fatal("deleted row still shown after leaving inline diff")
	}
}
