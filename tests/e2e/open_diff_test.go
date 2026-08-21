package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestOpenDiff(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	oldLines := []string{"line one", "line two", "line three"}
	newLines := []string{"line one", "line TWO", "line three", "line four"}

	h.app.EditorGroup.OpenDiff("test.go", diff.FileDiff{}, oldLines, newLines, true)
	h.redraw()

	h.assertContains("test.go (diff)")
}

func TestDiffToggleWrapCommand(t *testing.T) {
	h := newTestHarness(t, 80, 12)
	defer h.stop()

	oldLines := []string{"left-prefix-LEFT-SUFFIX"}
	newLines := []string{"right-prefix-RIGHT-SUFFIX"}
	h.app.EditorGroup.OpenDiff("wrapped.go", diff.FileDiff{}, oldLines, newLines, true)
	h.redraw()

	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil {
		t.Fatal("expected active diff widget")
	}
	if dv.IsWrapped() {
		t.Fatal("diff wrapping should be disabled by default")
	}
	if strings.Contains(h.screenText(), "FFIX") {
		t.Fatalf("long suffix should be truncated before wrapping:\n%s", h.screenText())
	}

	h.exec("diff.toggleWrap")
	if !dv.IsWrapped() {
		t.Fatal("diff wrapping should be enabled after command")
	}
	if !strings.Contains(h.screenText(), "FFIX") {
		t.Fatalf("wrapped diff should render both suffixes:\n%s", h.screenText())
	}

	h.exec("diff.toggleWrap")
	if dv.IsWrapped() {
		t.Fatal("second toggle should disable diff wrapping")
	}
}

func TestDiffToggleWrapCommandTargetsCommitDetail(t *testing.T) {
	h := newTestHarness(t, 100, 16)
	defer h.stop()

	oldLine := "left-prefix-" + strings.Repeat("L", 48) + "-LEFT-SUFFIX"
	newLine := "right-prefix-" + strings.Repeat("R", 48) + "-RIGHT-SUFFIX"
	fileDiff := diff.Parse("--- a/wrapped.go\n+++ b/wrapped.go\n@@ -1 +1 @@\n-" + oldLine + "\n+" + newLine + "\n")
	detail := ui.NewCommitDetailWidget(h.dir, "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []ui.CommitDetailFile{{Path: "wrapped.go", Diff: fileDiff}}, "")
	h.app.EditorGroup.OpenPluginTab("commit:full-hash", "Commit abc1234", detail)
	h.redraw()

	if detail.IsWrapped() {
		t.Fatal("commit detail wrapping should be disabled by default")
	}
	if strings.Contains(h.screenText(), "LEFT-SUFFIX") || strings.Contains(h.screenText(), "RIGHT-SUFFIX") {
		t.Fatalf("long suffixes should be clipped before wrapping:\n%s", h.screenText())
	}

	h.exec("diff.toggleWrap")
	if !detail.IsWrapped() {
		t.Fatal("diff wrap command did not enable commit detail wrapping")
	}
	for _, suffix := range []string{"LEFT-SUFFIX", "RIGHT-SUFFIX"} {
		if !strings.Contains(h.screenText(), suffix) {
			t.Fatalf("wrapped commit detail missing %q:\n%s", suffix, h.screenText())
		}
	}

	h.exec("diff.toggleWrap")
	if detail.IsWrapped() {
		t.Fatal("second diff wrap command did not restore clipped detail rows")
	}
}

func TestDiffToggleUnifiedCommand(t *testing.T) {
	h := newTestHarness(t, 100, 14)
	defer h.stop()

	oldLines := []string{"old one", "old two"}
	newLines := []string{"new one", "new two"}
	h.app.EditorGroup.OpenDiff("unified.go", diff.FileDiff{}, oldLines, newLines, true)
	h.redraw()

	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil {
		t.Fatal("expected active diff widget")
	}
	if dv.IsUnified() {
		t.Fatal("unified mode should be disabled by default")
	}

	h.exec("diff.toggleUnified")
	if !dv.IsUnified() {
		t.Fatal("unified mode should be enabled after command")
	}
	screen := h.screenText()
	oldOne, oldTwo := strings.Index(screen, "old one"), strings.Index(screen, "old two")
	newOne, newTwo := strings.Index(screen, "new one"), strings.Index(screen, "new two")
	if oldOne < 0 || oldTwo < oldOne || newOne < oldTwo || newTwo < newOne {
		t.Fatalf("unified diff should stack all removals before additions:\n%s", screen)
	}

	h.exec("diff.toggleUnified")
	if dv.IsUnified() {
		t.Fatal("second toggle should restore split mode")
	}
}

func TestDiffExplicitModesAndViewMenuState(t *testing.T) {
	h := newTestHarness(t, 100, 14)
	defer h.stop()

	h.app.EditorGroup.OpenDiff("modes.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	h.redraw()
	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil {
		t.Fatal("expected active diff widget")
	}
	if dv.Mode() != ui.DiffModeSplit || dv.WrapMode() != ui.DiffWrapOff {
		t.Fatalf("default presentation = mode %v wrap %v, want split/off", dv.Mode(), dv.WrapMode())
	}

	checked := func(command string) int {
		t.Helper()
		for _, item := range h.app.BuildViewMenu() {
			if item.Command == command {
				return item.Checked
			}
		}
		t.Fatalf("View menu missing %s", command)
		return 0
	}
	if checked("diff.splitView") != ui.MenuChecked || checked("diff.unifiedView") != ui.MenuUnchecked || checked("diff.toggleWrap") != ui.MenuUnchecked {
		t.Fatalf("default View menu did not expose split/off state: %+v", h.app.BuildViewMenu())
	}

	h.exec("diff.unifiedView")
	h.exec("diff.toggleWrap")
	if dv.Mode() != ui.DiffModeUnified || dv.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("explicit commands left mode %v wrap %v, want unified/on", dv.Mode(), dv.WrapMode())
	}
	if checked("diff.splitView") != ui.MenuUnchecked || checked("diff.unifiedView") != ui.MenuChecked || checked("diff.toggleWrap") != ui.MenuChecked {
		t.Fatalf("updated View menu did not expose unified/on state: %+v", h.app.BuildViewMenu())
	}
}

func TestViewDiffOverridesDoNotPersistAndNextDiffUsesDefaults(t *testing.T) {
	h := newTestHarness(t, 100, 14)
	defer h.stop()

	h.exec("options.useUnifiedDiff")
	h.exec("options.toggleDiffWordWrap")
	h.app.EditorGroup.OpenDiff("first.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	first := h.app.EditorGroup.ActiveDiffWidget()
	if first == nil || first.Mode() != ui.DiffModeUnified || first.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("first diff did not adopt defaults: %+v", first)
	}

	h.exec("diff.splitView")
	h.exec("diff.toggleWrap")
	if first.Mode() != ui.DiffModeSplit || first.WrapMode() != ui.DiffWrapOff {
		t.Fatalf("View override = mode %v wrap %v, want split/off", first.Mode(), first.WrapMode())
	}
	if h.app.Settings.Editor.DiffMode != config.DiffModeUnified || !h.app.Settings.Editor.DiffWordWrap {
		t.Fatalf("View override rewrote in-memory defaults: mode %q wrap %v",
			h.app.Settings.Editor.DiffMode, h.app.Settings.Editor.DiffWordWrap)
	}
	saved := config.LoadSettings()
	if saved.Editor.DiffMode != config.DiffModeUnified || !saved.Editor.DiffWordWrap {
		t.Fatalf("View override rewrote saved defaults: mode %q wrap %v",
			saved.Editor.DiffMode, saved.Editor.DiffWordWrap)
	}

	h.app.EditorGroup.OpenDiff("second.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	second := h.app.EditorGroup.ActiveDiffWidget()
	if second == nil || second.Mode() != ui.DiffModeUnified || second.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("next diff did not adopt saved defaults: %+v", second)
	}
}

func TestDiffTabModeControlSwitchesProjection(t *testing.T) {
	h := newTestHarness(t, 100, 14)
	defer h.stop()
	h.app.EditorGroup.OpenDiff("control.go", diff.FileDiff{}, []string{"old one", "old two"}, []string{"new one", "new two"}, true)
	h.redraw()

	lines := strings.Split(h.screenText(), "\n")
	controlX, controlY := -1, -1
	for y, line := range lines {
		if byteIndex := strings.Index(line, "Unified"); byteIndex >= 0 && strings.Contains(line, "● Split") {
			controlX = textwidth.String(line[:byteIndex]) + 1
			controlY = y
			break
		}
	}
	if controlX < 0 {
		t.Fatalf("visible diff mode control not found:\n%s", h.screenText())
	}
	h.click(controlX, controlY)
	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil || dv.Mode() != ui.DiffModeUnified {
		t.Fatalf("clicking Unified left active mode at %v", dv)
	}
	if !strings.Contains(h.screenText(), "● Unified") || !strings.Contains(h.screenText(), "○ Split") {
		t.Fatalf("mode control did not update its visible current value:\n%s", h.screenText())
	}
}

func TestDiffUnifiedCommandTargetsCommitDetail(t *testing.T) {
	h := newTestHarness(t, 100, 16)
	defer h.stop()
	fileDiff := diff.Parse("--- a/detail.go\n+++ b/detail.go\n@@ -1,2 +1,2 @@\n-old one\n-old two\n+new one\n+new two\n")
	detail := ui.NewCommitDetailWidget(h.dir, "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []ui.CommitDetailFile{{Path: "detail.go", Diff: fileDiff}}, "")
	h.app.EditorGroup.OpenPluginTab("commit:full-hash", "Commit abc1234", detail)
	h.redraw()

	h.exec("diff.unifiedView")
	if detail.Mode() != ui.DiffModeUnified {
		t.Fatal("explicit unified command did not target commit detail")
	}
	screen := h.screenText()
	oldOne, oldTwo := strings.Index(screen, "old one"), strings.Index(screen, "old two")
	newOne, newTwo := strings.Index(screen, "new one"), strings.Index(screen, "new two")
	if oldOne < 0 || oldTwo < oldOne || newOne < oldTwo || newTwo < newOne {
		t.Fatalf("commit detail unified projection is not stacked:\n%s", screen)
	}

	h.exec("diff.splitView")
	if detail.Mode() != ui.DiffModeSplit {
		t.Fatal("explicit split command did not restore commit detail")
	}
}

func TestCommitDetailSelectionCopiesAndCollapseCommandsRebuildDocument(t *testing.T) {
	h := newTestHarness(t, 80, 14)
	defer h.stop()
	clipboard.DisableSystem()
	clipboard.Set("")

	fileDiff := diff.Parse("--- a/detail.go\n+++ b/detail.go\n@@ -1 +1 @@\n-abcdefghij\n+ABCDEFGHIJ\n")
	detail := ui.NewCommitDetailWidget(h.dir, "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []ui.CommitDetailFile{{Path: "first.go", Diff: fileDiff}, {Path: "second.go", Diff: fileDiff}}, "")
	h.app.EditorGroup.OpenPluginTab("commit:full-hash", "Commit abc1234", detail)
	h.redraw()

	// The first diff row follows the message header, subject, spacer, and file
	// heading. The split view's content begins after the four-column gutter.
	rect := detail.GetRect()
	selectY := rect.Y + 4
	h.app.Root.HandleEvent(tcell.NewEventMouse(rect.X+6, selectY, tcell.Button1, tcell.ModNone))
	h.app.Root.HandleEvent(tcell.NewEventMouse(rect.X+12, selectY, tcell.Button1, tcell.ModNone))
	h.app.Root.HandleEvent(tcell.NewEventMouse(rect.X+12, selectY, tcell.ButtonNone, tcell.ModNone))
	h.exec("editor.copy")
	if got := clipboard.Get(); got != "cdefgh" {
		t.Fatalf("commit detail copied %q, want original selected text", got)
	}

	h.exec("commitDetail.collapseAll")
	if strings.Contains(h.screenText(), "abcdefghij") || strings.Contains(h.screenText(), "ABCDEFGHIJ") {
		t.Fatalf("collapse-all left diff content visible:\n%s", h.screenText())
	}
	for _, heading := range []string{"first.go", "second.go", "Expand all"} {
		if !strings.Contains(h.screenText(), heading) {
			t.Fatalf("collapsed document missing %q:\n%s", heading, h.screenText())
		}
	}

	h.exec("commitDetail.expandAll")
	if !strings.Contains(h.screenText(), "abcdefghij") || !strings.Contains(h.screenText(), "Collapse all") {
		t.Fatalf("expand-all did not restore document:\n%s", h.screenText())
	}
}

func TestCommitDetailStickyControlCollapsesCurrentLongFile(t *testing.T) {
	h := newTestHarness(t, 72, 12)
	defer h.stop()
	var patch strings.Builder
	patch.WriteString("--- a/deep.go\n+++ b/deep.go\n@@ -1,12 +1,12 @@\n")
	for i := 0; i < 12; i++ {
		patch.WriteString("-old sticky line\n")
	}
	for i := 0; i < 12; i++ {
		patch.WriteString("+new sticky line\n")
	}
	detail := ui.NewCommitDetailWidget(h.dir, "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []ui.CommitDetailFile{{Path: "very/long/path/current-file.go", Diff: diff.Parse(patch.String())}}, "")
	h.app.EditorGroup.OpenPluginTab("commit:full-hash", "Commit abc1234", detail)
	detail.TopLine = 7
	h.redraw()
	rect := detail.GetRect()
	if !strings.Contains(strings.Split(h.screenText(), "\n")[rect.Y], "current-file.go") {
		t.Fatalf("sticky file heading missing before collapse:\n%s", h.screenText())
	}

	// The sticky disclosure remains at the content origin even though the real
	// heading is above the viewport.
	h.click(rect.X+1, rect.Y)
	if strings.Contains(h.screenText(), "old sticky") || strings.Contains(h.screenText(), "new sticky") {
		t.Fatalf("sticky disclosure did not collapse its file:\n%s", h.screenText())
	}
}

func TestCompactDiffShowsCollapsedLineDistance(t *testing.T) {
	h := newTestHarness(t, 100, 14)
	defer h.stop()

	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -22,3 +22,3 @@\n line 22\n line 23\n line 24\n@@ -356,2 +356,2 @@\n line 356\n line 357\n")
	h.app.EditorGroup.OpenDiff("distance.go", fd, nil, nil, false)
	h.redraw()

	if !strings.Contains(h.screenText(), "⋯ 331 lines ⋯") {
		t.Fatalf("compact diff should show the collapsed distance:\n%s", h.screenText())
	}
}

func TestCompactDiffOmitsSeparatorForAdjacentLines(t *testing.T) {
	h := newTestHarness(t, 100, 14)
	defer h.stop()

	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -24,1 +24,1 @@\n line 24\n@@ -25,1 +25,1 @@\n line 25\n")
	h.app.EditorGroup.OpenDiff("distance.go", fd, nil, nil, false)
	h.redraw()

	rows := strings.Split(h.screenText(), "\n")
	line24Row, line25Row := -1, -1
	for i, row := range rows {
		if strings.Contains(row, "line 24") {
			line24Row = i
		}
		if strings.Contains(row, "line 25") {
			line25Row = i
		}
	}
	if line24Row < 0 || line25Row != line24Row+1 {
		t.Fatalf("adjacent diff rows should render consecutively, got rows %d and %d:\n%s", line24Row, line25Row, h.screenText())
	}
	if strings.Contains(h.screenText(), "⋯") {
		t.Fatalf("adjacent diff rows should not render a collapsed separator:\n%s", h.screenText())
	}
}

func TestOpenBufferReadOnly(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	lines := []string{"hello world", "second line", "third line"}
	h.app.EditorGroup.OpenBufferReadOnly("test (abc123)", "test.go", lines)
	h.redraw()

	h.assertContains("test (abc123)")
	h.assertContains("hello world")

	if !h.app.EditorGroup.IsActiveReadOnly() {
		t.Fatal("expected tab to be readonly")
	}

	// Typing should be blocked
	h.pressRune('x')
	h.redraw()
	buf := h.app.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatal("expected buffer")
	}
	if buf.Lines[0] != "hello world" {
		t.Fatalf("expected buffer unchanged, got %q", buf.Lines[0])
	}

	// Enter should be blocked
	h.pressKey(tcell.KeyEnter, 0)
	h.redraw()
	if len(buf.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(buf.Lines))
	}

	// Backspace should be blocked
	h.pressKey(tcell.KeyBackspace2, 0)
	h.redraw()
	if buf.Lines[0] != "hello world" {
		t.Fatalf("expected buffer unchanged after backspace, got %q", buf.Lines[0])
	}

	// Save should be a no-op
	saved := h.app.EditorGroup.Save()
	if saved {
		t.Fatal("expected save to return false for readonly tab")
	}
}

func TestOpenFileReadOnly(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "readonly.txt")
	os.WriteFile(f, []byte("original content\n"), 0644)

	h.app.EditorGroup.OpenFileReadOnly(f, "readonly.txt (v1)")
	h.redraw()

	h.assertContains("readonly.txt (v1)")

	if !h.app.EditorGroup.IsActiveReadOnly() {
		t.Fatal("expected tab to be readonly")
	}

	// Typing should be blocked
	h.pressRune('z')
	h.redraw()
	buf := h.app.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatal("expected buffer")
	}
	if buf.Lines[0] != "original content" {
		t.Fatalf("expected buffer unchanged, got %q", buf.Lines[0])
	}
}

func TestOpenBufferReadOnlyReuseTab(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.app.EditorGroup.OpenBufferReadOnly("test (v1)", "test.go", []string{"first"})
	h.redraw()

	h.app.EditorGroup.OpenBufferReadOnly("test (v1)", "test.go", []string{"updated"})
	h.redraw()

	buf := h.app.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatal("expected buffer")
	}
	if buf.Lines[0] != "updated" {
		t.Fatalf("expected buffer content updated on reopen, got %q", buf.Lines[0])
	}
}
