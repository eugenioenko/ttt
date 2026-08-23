package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/ui"
)

func focusedContextMenu(t *testing.T, h *testHarness) *ui.ContextMenuWidget {
	t.Helper()
	menu, ok := h.app.Root.Focused.(*ui.ContextMenuWidget)
	if !ok {
		t.Fatalf("focused widget = %T, want context menu", h.app.Root.Focused)
	}
	return menu
}

func assertSurfaceMenuState(t *testing.T, menu *ui.ContextMenuWidget, checkedCommands ...string) {
	t.Helper()
	checked := make(map[string]bool, len(checkedCommands))
	for _, command := range checkedCommands {
		checked[command] = true
	}
	for _, command := range []string{"diff.splitView", "diff.unifiedView", "diff.changesOnlyView", "diff.fullFileView", "diff.toggleWrap"} {
		item, ok := findMenuCommand(menu.Items, command)
		if !ok {
			t.Errorf("menu missing %s: %+v", command, menu.Items)
			continue
		}
		want := ui.MenuUnchecked
		if checked[command] {
			want = ui.MenuChecked
		}
		if item.Checked != want {
			t.Errorf("%s checked = %d, want %d", command, item.Checked, want)
		}
	}
	if _, ok := findMenuCommand(menu.Items, "options.toggleDiffHighContrast"); ok {
		t.Error("active-surface menu includes global High Contrast command")
	}
}

func TestCommitDetailContentAndTabMenusControlOnlyActiveSurface(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	h.app.EditorGroup.OpenDiff("background.txt", diff.FileDiff{}, []string{"old"}, []string{"new"}, true)
	background := h.app.EditorGroup.ActiveDiffWidget()
	detail := ui.NewCommitDetailWidget(h.dir, "ref", "abcdef0", false)
	file := ui.CommitDetailFileWithContent(ui.CommitDetailFile{
		Path: "detail.txt",
		Diff: diff.Parse("--- a/detail.txt\n+++ b/detail.txt\n@@ -1 +1 @@\n-old\n+new\n"),
	}, []string{"old"}, []string{"new"})
	detail.SetDetail("message", []ui.CommitDetailFile{file}, "")
	h.app.EditorGroup.ApplyDiffDefaults(detail)
	h.app.EditorGroup.OpenPluginTab("commit-detail", "Commit abcdef0", detail)
	h.redraw()

	r := h.app.EditorGroup.GetRect()
	openContentMenu := func() *ui.ContextMenuWidget {
		h.rightClick(r.X+r.W/2, r.Y+4)
		return focusedContextMenu(t, h)
	}
	executeContentCommand := func(command string) {
		menu := openContentMenu()
		if _, ok := findMenuCommand(menu.Items, command); !ok {
			t.Fatalf("content menu missing %s", command)
		}
		menu.OnExec(command)
		h.redraw()
	}

	assertSurfaceMenuState(t, openContentMenu(), "diff.splitView", "diff.changesOnlyView")
	focusedContextMenu(t, h).OnDismiss()
	executeContentCommand("diff.unifiedView")
	executeContentCommand("diff.fullFileView")
	executeContentCommand("diff.toggleWrap")
	if detail.Mode() != ui.DiffModeUnified || detail.ContextMode() != ui.DiffContextFullFile || detail.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("detail state = mode %v context %v wrap %v", detail.Mode(), detail.ContextMode(), detail.WrapMode())
	}
	if background.Mode() != ui.DiffModeSplit || background.ContextMode() != ui.DiffContextFullFile || background.WrapMode() != ui.DiffWrapOff {
		t.Fatalf("inactive diff changed: mode %v context %v wrap %v", background.Mode(), background.ContextMode(), background.WrapMode())
	}
	if h.app.Settings.Editor.DiffMode != config.DiffModeSplit || h.app.Settings.Editor.DiffContext != config.DiffContextChanges || h.app.Settings.Editor.DiffWordWrap {
		t.Fatalf("surface actions persisted defaults: mode %q context %q wrap %v", h.app.Settings.Editor.DiffMode, h.app.Settings.Editor.DiffContext, h.app.Settings.Editor.DiffWordWrap)
	}

	tabRect := h.app.EditorGroup.TabBar.GetRect()
	h.app.EditorGroup.TabBar.OnTabRightClick(h.app.EditorGroup.ActiveTabIndex(), tabRect.X+1, tabRect.Y)
	assertSurfaceMenuState(t, focusedContextMenu(t, h), "diff.unifiedView", "diff.fullFileView", "diff.toggleWrap")
}
