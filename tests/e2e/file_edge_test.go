package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/git"

	"github.com/gdamore/tcell/v3"
)

func openFileEdgeFixture(t *testing.T, h *testHarness, name, content string) {
	t.Helper()
	path := filepath.Join(h.dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.exec("editor.focus")
	h.redraw()
}

func numberedFile(n int) string {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	return sb.String()
}

func TestCtrlHomeEndMoveToTheFileEdges(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	openFileEdgeFixture(t, h, "long.txt", numberedFile(300))
	ed := h.app.EditorGroup.Editor

	h.pressKey(tcell.KeyEnd, tcell.ModCtrl)
	last := len(ed.Buf.Lines) - 1
	if ed.Cursor.Line != last || ed.Cursor.Col != len([]rune(ed.Buf.Lines[last])) || ed.Selection.Active {
		t.Fatalf("ctrl+end cursor = %d:%d selection %v, want the end of line %d without selection", ed.Cursor.Line, ed.Cursor.Col, ed.Selection.Active, last)
	}
	h.assertContains("line 300")
	h.assertNotContains("line 1 ")

	h.pressKey(tcell.KeyHome, tcell.ModCtrl)
	if ed.Cursor.Line != 0 || ed.Cursor.Col != 0 || ed.Selection.Active {
		t.Fatalf("ctrl+home cursor = %d:%d selection %v, want 0:0 without selection", ed.Cursor.Line, ed.Cursor.Col, ed.Selection.Active)
	}
	h.assertContains("line 1")
	h.assertNotContains("line 300")
}

func TestCtrlShiftHomeEndSelectToTheFileEdges(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	openFileEdgeFixture(t, h, "long.txt", numberedFile(300))
	ed := h.app.EditorGroup.Editor
	ed.Cursor.Line, ed.Cursor.Col = 150, 2

	h.pressKey(tcell.KeyEnd, tcell.ModCtrl|tcell.ModShift)
	last := len(ed.Buf.Lines) - 1
	start, end := ed.Selection.Range(ed.Cursor.Line, ed.Cursor.Col)
	if !ed.Selection.Active || start.Line != 150 || start.Col != 2 || end.Line != last || end.Col != len([]rune(ed.Buf.Lines[last])) {
		t.Fatalf("ctrl+shift+end selected %v..%v active=%v, want 150:2 to the end of line %d", start, end, ed.Selection.Active, last)
	}

	h.pressKey(tcell.KeyHome, tcell.ModCtrl|tcell.ModShift)
	start, end = ed.Selection.Range(ed.Cursor.Line, ed.Cursor.Col)
	if !ed.Selection.Active || start.Line != 0 || start.Col != 0 || end.Line != 150 || end.Col != 2 {
		t.Fatalf("ctrl+shift+home selected %v..%v active=%v, want 0:0..150:2 from the original anchor", start, end, ed.Selection.Active)
	}
}

func TestCtrlEndStopsOnACollapsedFinalFold(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.exec("options.toggleShowTrailingNewline")
	openFileEdgeFixture(t, h, "fold.py", "first = 1\ndef tail():\n    a = 1\n    b = 2")
	ed := h.app.EditorGroup.Editor
	h.exec("fold.collapseAll")
	h.redraw()

	h.pressKey(tcell.KeyEnd, tcell.ModCtrl)
	if ed.Cursor.Line != 1 || ed.Cursor.Col != len("def tail():") {
		t.Fatalf("ctrl+end into a collapsed fold = %d:%d, want the end of the fold line 1:%d", ed.Cursor.Line, ed.Cursor.Col, len("def tail():"))
	}
	h.pressKey(tcell.KeyHome, tcell.ModCtrl)
	if ed.Cursor.Line != 0 || ed.Cursor.Col != 0 {
		t.Fatalf("ctrl+home = %d:%d, want 0:0", ed.Cursor.Line, ed.Cursor.Col)
	}
}

func TestCtrlEndLandsOnTheOutermostCollapsedFold(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.exec("options.toggleShowTrailingNewline")
	openFileEdgeFixture(t, h, "nested.py", "first = 1\ndef outer():\n    x = 1\n    def inner():\n        a = 1\n        b = 2")
	ed := h.app.EditorGroup.Editor
	h.exec("fold.collapseAll")
	h.redraw()
	if ed.Folds == nil || ed.Folds.IsLineHidden(1) || !ed.Folds.IsLineHidden(3) {
		t.Fatal("fixture: outer fold should be collapsed and hide the inner fold's line")
	}

	h.pressKey(tcell.KeyEnd, tcell.ModCtrl|tcell.ModShift)
	if ed.Cursor.Line != 1 || ed.Cursor.Col != len("def outer():") {
		t.Fatalf("ctrl+shift+end with nested folds = %d:%d, want the outer fold line 1:%d", ed.Cursor.Line, ed.Cursor.Col, len("def outer():"))
	}
	if ed.Folds.IsLineHidden(ed.Cursor.Line) {
		t.Fatalf("cursor landed on hidden line %d", ed.Cursor.Line)
	}
}

func TestRevealingANestedHiddenLineLandsBelowTheOuterFold(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	openFileEdgeFixture(t, h, "reveal.py", "first = 1\ndef outer():\n    x = 1\n    def inner():\n        a = 1\n        b = 2\n    y = 3\nlast = 2\n")
	ed := h.app.EditorGroup.Editor
	h.exec("fold.collapseAll")
	h.redraw()
	if !ed.Folds.IsLineHidden(6) || ed.Folds.IsLineHidden(7) {
		t.Fatal("fixture: the outer fold should hide line 6 and leave line 7 visible")
	}

	ed.Cursor.Line, ed.Cursor.Col = 4, 0
	ed.EnsureCursorVisible()
	if ed.Cursor.Line != 7 {
		t.Fatalf("revealing hidden line 4 moved the cursor to line %d, want 7 below the outer fold", ed.Cursor.Line)
	}
}

func TestRevealingALineInAFoldThatEndsTheFileLandsOnTheFold(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.exec("options.toggleShowTrailingNewline")
	openFileEdgeFixture(t, h, "eof.py", "first = 1\ndef outer():\n    x = 1\n    def inner():\n        a = 1\n        b = 2")
	ed := h.app.EditorGroup.Editor
	h.exec("fold.collapseAll")
	h.redraw()
	if !ed.Folds.IsLineHidden(5) {
		t.Fatal("fixture: the outer fold should hide the file's last line")
	}

	ed.Cursor.Line, ed.Cursor.Col = 4, 0
	ed.EnsureCursorVisible()
	if ed.Cursor.Line != 1 || ed.Folds.IsLineHidden(ed.Cursor.Line) {
		t.Fatalf("revealing hidden line 4 left the cursor on line %d, want the outer fold line 1", ed.Cursor.Line)
	}
}

func TestCtrlEndRevealsAWrappedLastLine(t *testing.T) {
	h := newTestHarness(t, 60, 20)
	defer h.stop()
	last := strings.Repeat("wrap ", 40) + "TAIL"
	h.exec("options.toggleShowTrailingNewline")
	openFileEdgeFixture(t, h, "wrap.txt", numberedFile(100)+last)
	ed := h.app.EditorGroup.Editor
	ed.WordWrap = true
	h.redraw()

	h.pressKey(tcell.KeyEnd, tcell.ModCtrl)
	if ed.Cursor.Line != 100 || ed.Cursor.Col != len([]rune(last)) {
		t.Fatalf("ctrl+end with wrap = %d:%d, want 100:%d", ed.Cursor.Line, ed.Cursor.Col, len([]rune(last)))
	}
	h.assertContains("TAIL")
}

func TestCtrlHomeEndInAnEmptyFile(t *testing.T) {
	h := newTestHarness(t, 60, 20)
	defer h.stop()
	openFileEdgeFixture(t, h, "empty.txt", "")
	ed := h.app.EditorGroup.Editor

	for _, mod := range []tcell.ModMask{tcell.ModCtrl, tcell.ModCtrl | tcell.ModShift} {
		h.pressKey(tcell.KeyEnd, mod)
		h.pressKey(tcell.KeyHome, mod)
		if ed.Cursor.Line != 0 || ed.Cursor.Col != 0 {
			t.Fatalf("cursor in an empty file = %d:%d, want 0:0", ed.Cursor.Line, ed.Cursor.Col)
		}
	}
}

func TestCtrlEndInEditableDiff(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "code.txt")
	lines := prefixedLines(300, "row ")
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

func prefixedLines(n int, prefix string) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("%s%d", prefix, i+1)
	}
	return lines
}
