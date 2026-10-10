package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/undo"
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

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "code.txt", Status: "M"}, false, false)
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

func TestSplitInlineDiffScrollsInStep(t *testing.T) {
	h := newTestHarness(t, 120, 20)
	defer h.stop()
	path := filepath.Join(h.dir, "long.txt")
	var old, cur []string
	for i := 1; i <= 100; i++ {
		line := fmt.Sprintf("line %03d", i)
		old = append(old, line)
		switch i {
		case 10:
			cur = append(cur, "changed 010")
		case 20:
			cur = append(cur, line, "added a", "added b", "added c")
		case 30:
		default:
			cur = append(cur, line)
		}
	}
	os.WriteFile(path, []byte(strings.Join(old, "\n")+"\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	os.WriteFile(path, []byte(strings.Join(cur, "\n")+"\n"), 0644)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "long.txt", Status: "M"}, false, false)
	h.exec("diff.splitView")
	h.app.EditorGroup.GoToLine(80)
	h.redraw()

	aligned := 0
	for y := 0; y < 20; y++ {
		row := h.screenRow(y)
		idx := strings.Index(row, "line ")
		if idx < 0 {
			continue
		}
		label := row[idx : idx+len("line 000")]
		if strings.Count(row, label) != 2 {
			t.Fatalf("row %d not aligned across panes: %q", y, row)
		}
		aligned++
	}
	if aligned == 0 {
		t.Fatalf("no context rows visible after scrolling:\n%s", h.screenText())
	}
	h.assertContains("line 078")
}

func TestSplitInlineDiffWrapsInStepAndCopiesFromHead(t *testing.T) {
	clipboard.DisableSystem()
	h := newTestHarness(t, 100, 20)
	defer h.stop()
	path := filepath.Join(h.dir, "wrap.txt")
	os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	os.WriteFile(path, []byte("alpha\nbeta "+strings.Repeat("wrapped ", 12)+"\ngamma\n"), 0644)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "wrap.txt", Status: "M"}, false, false)
	h.exec("diff.splitView")
	if !h.app.Settings.Editor.WordWrap {
		h.exec("options.toggleWordWrap")
	}
	h.redraw()

	gammaRows := 0
	for y := 0; y < 20; y++ {
		if strings.Count(h.screenRow(y), "gamma") == 2 {
			gammaRows++
		}
	}
	if gammaRows != 1 {
		t.Fatalf("wrapped split rows are not aligned:\n%s", h.screenText())
	}

	alphaY := -1
	for y := 0; y < 20; y++ {
		if strings.Contains(h.screenRow(y), "alpha") {
			alphaY = y
			break
		}
	}
	x := strings.Index(h.screenRow(alphaY), "alpha")
	x = len([]rune(h.screenRow(alphaY)[:x]))
	h.click(x+1, alphaY)
	h.click(x+1, alphaY)
	h.exec("editor.copy")
	if got := clipboard.Get(); got != "alpha" {
		t.Fatalf("copy from HEAD pane = %q", got)
	}
}

func TestInlineDiffSuspendsFoldsAndRestoresThem(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "fold.txt")
	content := "top\nblock:\n    inner one\n    inner two\nend\n"
	os.WriteFile(path, []byte(content), 0644)
	initializeHarnessRepository(t, h.dir)
	os.WriteFile(path, []byte(strings.Replace(content, "top", "TOP", 1)), 0644)

	h.app.EditorGroup.OpenFile(path)
	h.app.EditorGroup.GoToLine(2)
	h.exec("fold.toggle")
	h.redraw()
	if strings.Contains(h.screenText(), "inner one") {
		t.Fatal("fold did not collapse before opening the diff")
	}

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "fold.txt", Status: "M"}, false, false)
	h.redraw()
	h.assertContains("inner one")
	h.exec("fold.collapseAll")
	h.redraw()
	h.assertContains("inner one")

	h.exec("editor.toggleDiff")
	h.redraw()
	if strings.Contains(h.screenText(), "inner one") {
		t.Fatal("file folds not restored after leaving the diff")
	}
}

func TestInlineDiffGapRowExpands(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "gaps.txt")
	var old []string
	for i := 1; i <= 40; i++ {
		old = append(old, fmt.Sprintf("row %02d", i))
	}
	cur := append([]string(nil), old...)
	cur[19] = "row 20 edited"
	os.WriteFile(path, []byte(strings.Join(old, "\n")+"\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	os.WriteFile(path, []byte(strings.Join(cur, "\n")+"\n"), 0644)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "gaps.txt", Status: "M"}, false, false)
	h.redraw()
	h.assertContains("⋯ 16 lines ⋯")
	if strings.Contains(h.screenText(), "row 05") {
		t.Fatalf("unchanged rows not collapsed:\n%s", h.screenText())
	}
	for y := 0; y < 30; y++ {
		row := h.screenRow(y)
		if i := strings.Index(row, "⋯ 16 lines ⋯"); i >= 0 {
			h.click(len([]rune(row[:i]))+2, y)
			break
		}
	}
	h.redraw()
	h.assertContains("row 05")
	if strings.Contains(h.screenText(), "⋯ 16 lines ⋯") {
		t.Fatalf("gap still shown after click:\n%s", h.screenText())
	}
}

func TestOpenChangesFocusFollowsFocusOnOpen(t *testing.T) {
	for _, focusOnOpen := range []bool{false, true} {
		h := newTestHarness(t, 100, 30)
		path := filepath.Join(h.dir, "focus.txt")
		os.WriteFile(path, []byte("one\ntwo\n"), 0644)
		initializeHarnessRepository(t, h.dir)
		os.WriteFile(path, []byte("one\nTWO\n"), 0644)
		h.app.Settings.Editor.FocusOnOpen = focusOnOpen
		h.app.FocusSidebar()
		before := h.app.Root.Focused

		h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "focus.txt", Status: "M"}, false, false)
		h.redraw()
		if !h.app.EditorGroup.IsInlineDiffActive() {
			t.Fatal("inline diff not active")
		}
		focusedEditor := h.app.Root.Focused == h.app.EditorGroup
		if focusOnOpen && !focusedEditor {
			t.Fatal("focusOnOpen did not focus the editable diff")
		}
		if !focusOnOpen && h.app.Root.Focused != before {
			t.Fatal("opening changes moved focus although focusOnOpen is off")
		}
		if focusOnOpen {
			h.pressRune('!')
			if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; !strings.Contains(got, "!") {
				t.Fatalf("typing after open did not reach the diff buffer: %q", got)
			}
		}
		h.stop()
	}
}

func TestInlineDiffFindSurvivesRecompute(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "code.txt")
	os.WriteFile(path, []byte("foo\nbar\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	os.WriteFile(path, []byte("foo\nbar\nbaz\n"), 0644)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "code.txt", Status: "M"}, false, false)
	h.redraw()
	h.exec("editor.focus")
	h.app.OpenFind()
	h.redraw()
	for _, r := range "ba" {
		h.pressRune(r)
	}
	h.assertContains(" 1/3")

	h.app.EditorGroup.Editor.ExecCommand(&undo.InsertLineCommand{Idx: 3, Text: "bat"})
	h.flushOnChange()
	h.app.RequestGitGutterForActiveFile()
	awaitGitGutter(t, h)

	h.assertContains(" 1/4")
	if dv := h.app.EditorGroup.ActiveInlineDiff(); dv == nil || len(dv.SearchMatchesRight) != 3 {
		t.Fatal("recomputed diff lost its search highlights")
	}
}
