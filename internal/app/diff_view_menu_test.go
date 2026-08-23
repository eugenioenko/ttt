package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/ui"
)

func assertActiveDiffViewMenu(t *testing.T, items []ui.ContextMenuItem, checkedCommands ...string) {
	t.Helper()
	checked := make(map[string]bool, len(checkedCommands))
	for _, command := range checkedCommands {
		checked[command] = true
	}
	wantCommands := []string{
		"diff.splitView",
		"diff.unifiedView",
		"",
		"diff.changesOnlyView",
		"diff.fullFileView",
		"",
		"diff.toggleWrap",
	}
	if len(items) != len(wantCommands) {
		t.Fatalf("menu items = %+v, want %d rows", items, len(wantCommands))
	}
	for index, wantCommand := range wantCommands {
		item := items[index]
		if wantCommand == "" {
			if !item.IsSep {
				t.Errorf("row %d = %+v, want separator", index, item)
			}
			continue
		}
		if item.Command != wantCommand {
			t.Errorf("row %d command = %q, want %q", index, item.Command, wantCommand)
		}
		wantChecked := menuChecked(checked[wantCommand])
		if item.Checked != wantChecked {
			t.Errorf("%s checked = %d, want %d", wantCommand, item.Checked, wantChecked)
		}
	}
}

func TestActiveDiffViewMenuTracksFileDiffAndCommitDetailState(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.EditorGroup.OpenDiff("file.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	fileDiff := a.EditorGroup.ActiveDiffWidget()
	fileDiff.SetMode(ui.DiffModeUnified)
	fileDiff.SetContextMode(ui.DiffContextFullFile)
	fileDiff.SetWrapMode(ui.DiffWrapOn)
	assertActiveDiffViewMenu(t, a.BuildActiveDiffViewMenu(),
		"diff.unifiedView", "diff.fullFileView", "diff.toggleWrap")

	detail := ui.NewCommitDetailWidget("/repo", "ref", "abcdef0", false)
	detail.SetDetail("message", nil, "")
	a.EditorGroup.ApplyDiffDefaults(detail)
	a.EditorGroup.OpenPluginTab("commit-detail", "Commit abcdef0", detail)
	assertActiveDiffViewMenu(t, a.BuildActiveDiffViewMenu(),
		"diff.splitView", "diff.changesOnlyView")

	for _, item := range a.BuildActiveDiffViewMenu() {
		if item.Command == "options.toggleDiffHighContrast" {
			t.Fatal("surface menu exposes global High Contrast preference")
		}
	}
}

func TestActiveDiffViewMenuSupportsFlatContentAndCompactTabLayouts(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	detail := ui.NewCommitDetailWidget("/repo", "ref", "abcdef0", false)
	detail.SetDetail("message", nil, "")
	a.EditorGroup.ApplyDiffDefaults(detail)
	a.EditorGroup.OpenPluginTab("commit-detail", "Commit abcdef0", detail)

	base := []ui.ContextMenuItem{{Label: "Base", Command: "base"}}
	flat := a.withActiveDiffViewMenu(base)
	if len(flat) < 2 || flat[len(flat)-1].Command != "diff.toggleWrap" {
		t.Fatalf("flat content menu does not expose direct controls: %+v", flat)
	}
	compact := a.withActiveDiffViewSubmenu(base)
	last := compact[len(compact)-1]
	if last.Label != "Diff View" || len(last.Submenu) == 0 {
		t.Fatalf("compact tab menu does not expose Diff View submenu: %+v", compact)
	}
	assertActiveDiffViewMenu(t, last.Submenu, "diff.splitView", "diff.changesOnlyView")
	if len(base) != 1 {
		t.Fatalf("layout builders mutated their base menu: %+v", base)
	}
}
