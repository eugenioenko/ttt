package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
)

func numberedLines(n int, prefix string) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("%s%d", prefix, i+1)
	}
	return lines
}

func TestCtrlHomeEndInFileTab(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "long.txt")
	content := ""
	for _, l := range numberedLines(300, "line ") {
		content += l + "\n"
	}
	os.WriteFile(path, []byte(content), 0644)
	h.app.EditorGroup.OpenFile(path)
	h.exec("editor.focus")
	h.redraw()

	h.pressKey(tcell.KeyEnd, tcell.ModCtrl)
	ed := h.app.EditorGroup.Editor
	if last := len(ed.Buf.Lines) - 1; ed.Cursor.Line != last || ed.Cursor.Col != len([]rune(ed.Buf.Lines[last])) {
		t.Fatalf("ctrl+end cursor = %d:%d, want end of line %d", ed.Cursor.Line, ed.Cursor.Col, last)
	}
	h.assertContains("line 300")

	h.pressKey(tcell.KeyHome, tcell.ModCtrl|tcell.ModShift)
	if ed.Cursor.Line != 0 || ed.Cursor.Col != 0 || !ed.Selection.Active {
		t.Fatalf("ctrl+shift+home cursor = %d:%d selection %v", ed.Cursor.Line, ed.Cursor.Col, ed.Selection.Active)
	}
	h.assertContains("line 1")
}

func TestCtrlEndReachesTheBottomOfReadOnlyDiffs(t *testing.T) {
	old := numberedLines(400, "old ")
	updated := append([]string(nil), old...)
	updated[5] = "changed near the top"
	updated[395] = "changed near the bottom"
	fd := diff.Parse(diff.Generate(old, updated, "f.txt"))
	for _, mode := range []string{config.DiffModeSplit, config.DiffModeUnified} {
		t.Run(mode, func(t *testing.T) {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			setDiffView(h, mode, config.DiffContextFull, false)
			h.app.EditorGroup.OpenDiff("f.txt", fd, old, updated, true)
			h.app.Root.SetFocus(h.app.EditorGroup)
			h.redraw()
			h.assertNotContains("old 400")

			h.pressKey(tcell.KeyEnd, tcell.ModCtrl)
			h.assertContains("old 400")
			h.assertContains("changed near the bottom")

			h.pressKey(tcell.KeyHome, tcell.ModCtrl)
			h.assertContains("old 1")
			h.assertNotContains("old 400")
		})
	}
}

func TestCtrlEndInEditableDiff(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "code.txt")
	lines := numberedLines(300, "row ")
	write := func(ls []string) {
		content := ""
		for _, l := range ls {
			content += l + "\n"
		}
		os.WriteFile(path, []byte(content), 0644)
	}
	write(lines)
	initializeHarnessRepository(t, h.dir)
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	edited := append([]string(nil), lines...)
	edited[299] = "last row edited"
	write(edited)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "code.txt", Status: "M"}, false, false)
	h.exec("editor.focus")
	h.redraw()
	h.pressKey(tcell.KeyEnd, tcell.ModCtrl)
	ed := h.app.EditorGroup.Editor
	if last := len(ed.Buf.Lines) - 1; ed.Cursor.Line != last {
		t.Fatalf("ctrl+end in an editable diff left the cursor on line %d, want %d", ed.Cursor.Line, last)
	}
	h.assertContains("last row edited")
}
