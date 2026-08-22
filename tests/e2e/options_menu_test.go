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

func findMenuCommand(items []ui.ContextMenuItem, command string) (ui.ContextMenuItem, bool) {
	for _, item := range items {
		if item.Command == command {
			return item, true
		}
		if found, ok := findMenuCommand(item.Submenu, command); ok {
			return found, true
		}
	}
	return ui.ContextMenuItem{}, false
}

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

func TestOptionsAndChangesShareCheckedPresentationSubmenus(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.app.EditorGroup.OpenDiff("existing.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil {
		t.Fatal("expected active diff")
	}

	options := h.app.BuildOptionsMenu()
	for _, command := range []string{"options.useSplitDiff", "options.useUnifiedDiff", "options.useChangesOnlyDiff", "options.useFullFileDiff", "options.toggleDiffWordWrap", "options.toggleDiffHighContrast", "options.useGitFileTree", "options.useGitFileList", "changes.expandAll", "changes.collapseAll"} {
		if _, ok := findMenuCommand(options, command); !ok {
			t.Errorf("Options menu missing %s", command)
		}
	}
	changes := h.app.BuildChangesPanelMenu()
	for _, command := range []string{"options.useSplitDiff", "options.useUnifiedDiff", "options.useChangesOnlyDiff", "options.useFullFileDiff", "options.toggleDiffWordWrap", "options.toggleDiffHighContrast", "options.useGitFileTree", "options.useGitFileList", "changes.expandAll", "changes.collapseAll"} {
		if _, ok := findMenuCommand(changes, command); !ok {
			t.Errorf("Changes menu missing contextual command %s", command)
		}
	}
	if item, ok := findMenuCommand(options, "options.useGitFileList"); !ok || item.Checked != ui.MenuChecked {
		t.Fatalf("List should be the checked default: item=%+v found=%v", item, ok)
	}
	h.exec("options.useGitFileTree")
	if item, ok := findMenuCommand(h.app.BuildChangesContextMenu(), "options.useGitFileTree"); !ok || item.Checked != ui.MenuChecked {
		t.Fatalf("Changes context did not share checked Tree state: item=%+v found=%v", item, ok)
	}

	h.exec("options.useUnifiedDiff")
	h.exec("options.useFullFileDiff")
	h.exec("options.toggleDiffWordWrap")
	h.exec("options.toggleDiffHighContrast")
	if dv.Mode() != ui.DiffModeUnified || dv.ContextMode() != ui.DiffContextFullFile || dv.WrapMode() != ui.DiffWrapOn || !dv.DiffHighContrast() {
		t.Fatalf("live inherited diff = mode %v context %v wrap %v contrast %v", dv.Mode(), dv.ContextMode(), dv.WrapMode(), dv.DiffHighContrast())
	}
	if h.app.Settings.Editor.DiffMode != config.DiffModeUnified || h.app.Settings.Editor.DiffContext != config.DiffContextFull || !h.app.Settings.Editor.DiffWordWrap || !h.app.Settings.Editor.DiffHighContrast {
		t.Fatalf("in-memory settings = %+v", h.app.Settings.Editor)
	}

	data, err := os.ReadFile(filepath.Join(h.dir, "config", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved config.Settings
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Editor.DiffMode != config.DiffModeUnified || saved.Editor.DiffContext != config.DiffContextFull || !saved.Editor.DiffWordWrap || !saved.Editor.DiffHighContrast {
		t.Fatalf("saved diff settings = %+v", saved.Editor)
	}
}

func TestOptionsDiffDefaultsRespectPerPropertyOverrides(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.app.EditorGroup.OpenDiff("first.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	first := h.app.EditorGroup.ActiveDiffWidget()
	first.SetMode(ui.DiffModeUnified)
	h.app.EditorGroup.OpenDiff("second.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	second := h.app.EditorGroup.ActiveDiffWidget()
	second.SetWrapMode(ui.DiffWrapOn)

	h.exec("options.useUnifiedDiff")
	h.exec("options.toggleDiffWordWrap")
	h.exec("options.useSplitDiff")
	h.exec("options.toggleDiffWordWrap")
	if first.Mode() != ui.DiffModeUnified || first.WrapMode() != ui.DiffWrapOff {
		t.Fatalf("first override/inheritance = mode %v wrap %v", first.Mode(), first.WrapMode())
	}
	if second.Mode() != ui.DiffModeSplit || second.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("second override/inheritance = mode %v wrap %v", second.Mode(), second.WrapMode())
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
