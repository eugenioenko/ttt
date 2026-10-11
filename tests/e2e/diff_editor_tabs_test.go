package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

func numberedLines(n int, change int, changed string) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	if change > 0 {
		lines[change-1] = changed
	}
	return lines
}

func awaitDiffOpen(t *testing.T, h *testHarness) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case event := <-h.screen.EventQ():
			interrupt, ok := event.(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			if result, ok := interrupt.Data().(*app.DiffOpenResult); ok {
				h.app.ApplyDiffOpen(result)
				h.redraw()
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the diff to open")
		}
	}
}

func TestCommitDiffOpensOnTheDiffEditor(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "code.txt")
	write := func(lines []string) {
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(numberedLines(30, 0, ""))
	initializeHarnessRepository(t, h.dir)
	write(numberedLines(30, 15, "CHANGED 15"))
	harnessGit(t, h.dir, "commit", "-qam", "change line 15")

	h.app.OpenCommitDiff(h.dir, "HEAD", "HEAD", git.FileStatus{Path: "code.txt", Status: "M"}, false)
	awaitDiffOpen(t, h)
	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil {
		t.Fatal("a commit diff did not open a DiffEditorWidget tab")
	}
	h.assertContains("code.txt @ HEAD")
	h.assertContains("CHANGED 15")
	h.assertContains("⋯ 11 lines ⋯")
	h.assertContains("⋯ 12 lines ⋯")
}

func TestHunkOnlyPRDiffShowsHunkNumbersAndFailsTheFetchOffline(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	old := numberedLines(30, 0, "")
	cur := numberedLines(30, 15, "CHANGED 15")
	group := &ui.ChangesGroup{IsPR: true, PRDiffs: map[string]string{"code.txt": diff.Generate(old, cur, "code.txt")}}

	h.app.OpenPRDiff(group, git.FileStatus{Path: "code.txt", Status: "M"}, false)
	h.redraw()
	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil {
		t.Fatal("a PR diff did not open a DiffEditorWidget tab")
	}
	h.assertContains("⋯ 11 lines ⋯")
	changed := ""
	for y := 0; y < 30; y++ {
		if row := h.screenRow(y); strings.Contains(row, "CHANGED 15") {
			changed = row
		}
	}
	if !strings.Contains(changed, "15  CHANGED 15") {
		t.Fatalf("changed row %q, want the hunk's line number 15 in the gutter", changed)
	}

	dv.SetContextMode(ui.DiffContextFullFile)
	h.redraw()
	if dv.Loading || dv.IsExtended() {
		t.Fatalf("an offline fetch left loading %v, extended %v", dv.Loading, dv.IsExtended())
	}
	h.assertContains("CHANGED 15")
	h.assertNotContains("Loading...")
}

func TestFindBarSearchesBothSidesOfADiffTab(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.app.EditorGroup.OpenDiff("code.txt", diff.FileDiff{}, []string{"keep", "old needle"}, []string{"keep", "new needle"}, true)
	h.redraw()
	dv := h.app.EditorGroup.ActiveDiffWidget()
	h.app.OpenFind()
	for _, r := range "needle" {
		h.pressRune(r)
	}
	h.redraw()
	if len(dv.SearchMatchesLeft) != 1 || len(dv.SearchMatchesRight) != 1 {
		t.Fatalf("find matches left %v right %v, want one per side", dv.SearchMatchesLeft, dv.SearchMatchesRight)
	}
	h.assertContains("1/2")
}

func TestGlobalSearchNavigationScrollsADiffTab(t *testing.T) {
	h := newTestHarness(t, 120, 20)
	defer h.stop()
	old := numberedLines(200, 0, "")
	cur := numberedLines(200, 150, "CHANGED 150")
	h.app.EditorGroup.OpenDiff("code.txt", diff.FileDiff{}, old, cur, true)
	h.redraw()
	h.assertNotContains("CHANGED 150")
	h.app.NavigateToSearchMatch("code.txt (diff)", 150, 0)
	h.redraw()
	h.assertContains("CHANGED 150")
}

func TestCtrlEndReachesTheEndOfADiffTab(t *testing.T) {
	h := newTestHarness(t, 120, 20)
	defer h.stop()
	old := numberedLines(200, 0, "")
	cur := numberedLines(200, 2, "CHANGED 02")
	h.app.EditorGroup.OpenDiff("code.txt", diff.FileDiff{}, old, cur, true)
	h.app.Root.SetFocus(h.app.EditorGroup)
	h.redraw()
	h.assertNotContains("line 200")
	h.pressKey(tcell.KeyEnd, tcell.ModCtrl)
	h.assertContains("line 200")
	h.pressKey(tcell.KeyHome, tcell.ModCtrl)
	h.assertContains("CHANGED 02")
}

func TestDiffGutterOptionsReachAnOpenDiffTab(t *testing.T) {
	h := newTestHarness(t, 100, 12)
	defer h.stop()
	h.app.EditorGroup.OpenDiff("code.txt", diff.FileDiff{}, []string{"same", "old"}, []string{"same", "new"}, true)
	h.app.EditorGroup.SetDiffDefaults(ui.DiffModeUnified, ui.DiffContextFullFile, false)
	h.redraw()
	signs := func() bool {
		for y := 0; y < 12; y++ {
			row := h.screenRow(y)
			if strings.Contains(row, "+new") || strings.Contains(row, "−old") {
				return true
			}
		}
		return false
	}
	if signs() {
		t.Fatalf("diff gutter signs drawn by default:\n%s", h.screenText())
	}
	h.exec("options.toggleDiffSigns")
	if !h.app.Settings.Editor.DiffSigns || !signs() {
		t.Fatalf("toggling diff gutter signs did not draw them:\n%s", h.screenText())
	}
	h.exec("options.toggleDiffGutterHighlight")
	if h.app.Settings.Editor.DiffGutterHighlight {
		t.Fatal("toggling diff gutter highlight did not turn it off")
	}
}
