package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
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

func TestDiffPresentationCommands(t *testing.T) {
	h := newTestHarness(t, 80, 12)
	defer h.stop()

	h.app.EditorGroup.OpenDiff("reading.go", diff.FileDiff{}, []string{"left-prefix-LEFT-SUFFIX"}, []string{"right-prefix-RIGHT-SUFFIX"}, true)
	h.redraw()
	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil || dv.Mode() != ui.DiffModeSplit || dv.WrapMode() != ui.DiffWrapOff {
		t.Fatalf("default diff presentation = %+v", dv)
	}
	if strings.Contains(h.screenText(), "SUFFIX") {
		t.Fatalf("unwrapped suffix should be clipped:\n%s", h.screenText())
	}

	h.exec("diff.unifiedView")
	h.exec("diff.toggleWrap")
	if dv.Mode() != ui.DiffModeUnified || dv.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("explicit presentation = mode %v wrap %v", dv.Mode(), dv.WrapMode())
	}
	if !strings.Contains(h.screenText(), "SUFFIX") {
		t.Fatalf("wrapped unified diff should expose suffixes:\n%s", h.screenText())
	}
	oldRow, newRow := strings.Index(h.screenText(), "left-prefix"), strings.Index(h.screenText(), "right-prefix")
	if oldRow < 0 || newRow <= oldRow {
		t.Fatalf("unified projection should stack removal before addition:\n%s", h.screenText())
	}
}

func TestExplicitDiffOverridesDoNotPersistAndNextDiffUsesDefaults(t *testing.T) {
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
		t.Fatalf("explicit override = mode %v wrap %v", first.Mode(), first.WrapMode())
	}
	if h.app.Settings.Editor.DiffMode != config.DiffModeUnified || !h.app.Settings.Editor.DiffWordWrap {
		t.Fatalf("explicit override rewrote defaults: mode %q wrap %v", h.app.Settings.Editor.DiffMode, h.app.Settings.Editor.DiffWordWrap)
	}

	h.app.EditorGroup.OpenDiff("second.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	second := h.app.EditorGroup.ActiveDiffWidget()
	if second == nil || second.Mode() != ui.DiffModeUnified || second.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("next diff did not adopt defaults: %+v", second)
	}
}

func TestDiffTabBarDoesNotDuplicateModeControls(t *testing.T) {
	h := newTestHarness(t, 100, 14)
	defer h.stop()
	h.app.EditorGroup.OpenDiff("control.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	h.redraw()
	if strings.Contains(h.screenText(), "● Split") || strings.Contains(h.screenText(), "○ Unified") {
		t.Fatalf("diff tab bar duplicated the Options control:\n%s", h.screenText())
	}
}

func TestCompactDiffShowsQuietCollapsedDistance(t *testing.T) {
	h := newTestHarness(t, 100, 14)
	defer h.stop()
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -22,3 +22,3 @@\n line 22\n line 23\n line 24\n@@ -356,2 +356,2 @@\n line 356\n line 357\n")
	h.app.EditorGroup.OpenDiff("distance.go", fd, nil, nil, false)
	h.redraw()
	if !strings.Contains(h.screenText(), "▶⋯ 331 lines ⋯") {
		t.Fatalf("compact diff should show a gutter disclosure and collapsed distance:\n%s", h.screenText())
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
