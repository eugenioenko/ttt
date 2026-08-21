package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestOptionsMenuToggleLineNumbers(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	if !h.app.Settings.Editor.LineNumbers {
		t.Fatal("line numbers should be enabled by default")
	}

	h.exec("options.toggleLineNumbers")

	if h.app.Settings.Editor.LineNumbers {
		t.Error("line numbers should be disabled after toggle")
	}
	if h.app.EditorGroup.LineNumbers {
		t.Error("editor group line numbers should be disabled after toggle")
	}
	if h.app.EditorGroup.Editor.LineNumbers {
		t.Error("editor pane line numbers should be disabled after toggle")
	}

	h.exec("options.toggleLineNumbers")

	if !h.app.Settings.Editor.LineNumbers {
		t.Error("line numbers should be re-enabled after second toggle")
	}
	if !h.app.EditorGroup.LineNumbers {
		t.Error("editor group line numbers should be re-enabled after second toggle")
	}
	if !h.app.EditorGroup.Editor.LineNumbers {
		t.Error("editor pane line numbers should be re-enabled after second toggle")
	}
}

func TestOptionsMenuToggleWordWrap(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	if h.app.Settings.Editor.WordWrap {
		t.Fatal("word wrap should be disabled by default")
	}

	h.exec("options.toggleWordWrap")

	if !h.app.Settings.Editor.WordWrap {
		t.Error("word wrap should be enabled after toggle")
	}

	h.exec("options.toggleWordWrap")

	if h.app.Settings.Editor.WordWrap {
		t.Error("word wrap should be disabled after second toggle")
	}
}

func TestOptionsMenuDiffDefaultsPersistAndUpdateInheritedSurface(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.app.EditorGroup.OpenDiff("existing.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil {
		t.Fatal("expected active diff")
	}

	checked := func(command string) int {
		t.Helper()
		for _, item := range h.app.BuildOptionsMenu() {
			if item.Command == command {
				return item.Checked
			}
		}
		t.Fatalf("Options menu missing %s", command)
		return 0
	}
	if checked("options.useSplitDiff") != ui.MenuChecked ||
		checked("options.useUnifiedDiff") != ui.MenuUnchecked ||
		checked("options.toggleDiffWordWrap") != ui.MenuUnchecked {
		t.Fatalf("default diff options state is wrong: %+v", h.app.BuildOptionsMenu())
	}

	h.exec("options.useUnifiedDiff")
	h.exec("options.toggleDiffWordWrap")

	if h.app.Settings.Editor.DiffMode != config.DiffModeUnified || !h.app.Settings.Editor.DiffWordWrap {
		t.Fatalf("settings defaults = mode %q wrap %v, want unified/true",
			h.app.Settings.Editor.DiffMode, h.app.Settings.Editor.DiffWordWrap)
	}
	if h.app.EditorGroup.DiffMode != ui.DiffModeUnified || !h.app.EditorGroup.DiffWordWrap {
		t.Fatalf("live defaults = mode %v wrap %v, want unified/true",
			h.app.EditorGroup.DiffMode, h.app.EditorGroup.DiffWordWrap)
	}
	if dv.Mode() != ui.DiffModeUnified || dv.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("open inherited surface = mode %v wrap %v, want unified/on", dv.Mode(), dv.WrapMode())
	}
	if checked("options.useSplitDiff") != ui.MenuUnchecked ||
		checked("options.useUnifiedDiff") != ui.MenuChecked ||
		checked("options.toggleDiffWordWrap") != ui.MenuChecked {
		t.Fatalf("updated diff options state is wrong: %+v", h.app.BuildOptionsMenu())
	}

	data, err := os.ReadFile(filepath.Join(h.dir, "config", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved config.Settings
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Editor.DiffMode != config.DiffModeUnified || !saved.Editor.DiffWordWrap {
		t.Fatalf("saved defaults = mode %q wrap %v, want unified/true",
			saved.Editor.DiffMode, saved.Editor.DiffWordWrap)
	}
}

func TestOptionsDiffDefaultsPreservePerPropertyViewOverridesAcrossOpenDiffs(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.app.EditorGroup.OpenDiff("first.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	first := h.app.EditorGroup.ActiveDiffWidget()
	if first == nil {
		t.Fatal("expected first diff")
	}
	first.SetMode(ui.DiffModeUnified)

	h.app.EditorGroup.OpenDiff("second.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	second := h.app.EditorGroup.ActiveDiffWidget()
	if second == nil {
		t.Fatal("expected second diff")
	}
	second.SetWrapMode(ui.DiffWrapOn)

	h.exec("options.useUnifiedDiff")
	h.exec("options.toggleDiffWordWrap")
	if first.Mode() != ui.DiffModeUnified || first.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("first surface did not preserve mode and inherit wrap: mode %v wrap %v", first.Mode(), first.WrapMode())
	}
	if second.Mode() != ui.DiffModeUnified || second.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("second surface did not inherit mode and preserve wrap: mode %v wrap %v", second.Mode(), second.WrapMode())
	}

	h.exec("options.useSplitDiff")
	h.exec("options.toggleDiffWordWrap")
	if first.Mode() != ui.DiffModeUnified || first.WrapMode() != ui.DiffWrapOff {
		t.Fatalf("first surface after second default = mode %v wrap %v, want unified/off", first.Mode(), first.WrapMode())
	}
	if second.Mode() != ui.DiffModeSplit || second.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("second surface after second default = mode %v wrap %v, want split/on", second.Mode(), second.WrapMode())
	}
}

func TestOptionsDiffDefaultsUpdateInheritedCommitDetailAndPreserveViewOverrides(t *testing.T) {
	h := newTestHarness(t, 100, 18)
	defer h.stop()

	fileDiff := diff.Parse("--- a/detail.go\n+++ b/detail.go\n@@ -1 +1 @@\n-old\n+new\n")
	detail := ui.NewCommitDetailWidget(h.dir, "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []ui.CommitDetailFile{{Path: "detail.go", Diff: fileDiff}}, "")
	h.app.EditorGroup.ApplyDiffDefaults(detail)
	h.app.EditorGroup.OpenPluginTab("commit:full-hash", "Commit abc1234", detail)

	h.exec("options.useUnifiedDiff")
	h.exec("options.toggleDiffWordWrap")
	if detail.Mode() != ui.DiffModeUnified || detail.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("inherited commit detail = mode %v wrap %v, want unified/on", detail.Mode(), detail.WrapMode())
	}

	h.exec("diff.splitView")
	h.exec("diff.toggleWrap")
	h.exec("options.useSplitDiff")
	h.exec("options.toggleDiffWordWrap")
	h.exec("options.useUnifiedDiff")
	h.exec("options.toggleDiffWordWrap")
	if detail.Mode() != ui.DiffModeSplit || detail.WrapMode() != ui.DiffWrapOff {
		t.Fatalf("defaults replaced commit detail View overrides: mode %v wrap %v", detail.Mode(), detail.WrapMode())
	}
}

func TestOptionsMenuSetGutterStyle(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	if h.app.Settings.Editor.GutterStyle != "compact" {
		t.Fatalf("expected default gutter style 'compact', got %q", h.app.Settings.Editor.GutterStyle)
	}

	h.app.SetGutterStyle("minimal")
	h.redraw()

	if h.app.Settings.Editor.GutterStyle != "minimal" {
		t.Errorf("expected gutter style 'minimal', got %q", h.app.Settings.Editor.GutterStyle)
	}
	if h.app.EditorGroup.GutterStyle != "minimal" {
		t.Errorf("expected editor group gutter style 'minimal', got %q", h.app.EditorGroup.GutterStyle)
	}
	if h.app.EditorGroup.Editor.GutterStyle != "minimal" {
		t.Errorf("expected editor pane gutter style 'minimal', got %q", h.app.EditorGroup.Editor.GutterStyle)
	}

	h.app.SetGutterStyle("extended")
	h.redraw()

	if h.app.Settings.Editor.GutterStyle != "extended" {
		t.Errorf("expected gutter style 'extended', got %q", h.app.Settings.Editor.GutterStyle)
	}
}

func TestOptionsMenuSetTabSize(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	if h.app.Settings.Editor.TabSize != 4 {
		t.Fatalf("expected default tab size 4, got %d", h.app.Settings.Editor.TabSize)
	}

	h.app.Settings.Editor.TabSize = 2
	h.app.EditorGroup.TabSize = 2
	h.app.EditorGroup.SetTabSize(2)
	h.redraw()

	if h.app.Settings.Editor.TabSize != 2 {
		t.Errorf("expected tab size 2, got %d", h.app.Settings.Editor.TabSize)
	}
	if h.app.EditorGroup.TabSize != 2 {
		t.Errorf("expected editor group tab size 2, got %d", h.app.EditorGroup.TabSize)
	}

	h.app.Settings.Editor.TabSize = 8
	h.app.EditorGroup.TabSize = 8
	h.app.EditorGroup.SetTabSize(8)
	h.redraw()

	if h.app.Settings.Editor.TabSize != 8 {
		t.Errorf("expected tab size 8, got %d", h.app.Settings.Editor.TabSize)
	}
}

func TestOptionsMenuBarPresent(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.assertContains("Options")
}

func TestOptionsMenuDynamicChecked(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	// Build the options menu and verify line numbers is checked
	items := h.app.BuildOptionsMenu()
	found := false
	for _, item := range items {
		if item.Command == "options.toggleLineNumbers" {
			found = true
			if item.Checked != 2 { // MenuChecked
				t.Errorf("expected line numbers checked (2), got %d", item.Checked)
			}
		}
	}
	if !found {
		t.Error("expected to find options.toggleLineNumbers in menu items")
	}

	// Toggle line numbers off
	h.exec("options.toggleLineNumbers")

	// Rebuild and verify unchecked
	items = h.app.BuildOptionsMenu()
	for _, item := range items {
		if item.Command == "options.toggleLineNumbers" {
			if item.Checked != 1 { // MenuUnchecked
				t.Errorf("expected line numbers unchecked (1), got %d", item.Checked)
			}
		}
	}
}
