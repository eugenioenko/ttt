package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/fileicons"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestFileIconsShowWhenEnabledInExplorerAndChanges(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	h.exec("options.toggleFontIcons")

	txt := fileicons.ForFile("alpha.txt").Glyph
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.redraw()
	tabRow := h.screenRow(2)
	if !strings.Contains(tabRow, txt) || !strings.Contains(tabRow, "alpha.txt") {
		t.Fatalf("editor tab does not show file icon:\n%s", tabRow)
	}
	h.exec("sidebar.explorer")
	h.assertContains(txt + " alpha.txt")
	h.assertContains("\ueab6 subdir")

	if err := os.WriteFile(filepath.Join(h.dir, "alpha.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	h.exec("sidebar.changes")
	h.assertContains("M " + txt + " alpha.txt")
}

func TestFileIconsToggleAcrossOpenInactiveDetails(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	file := ui.CommitDetailFile{Status: "M", Path: "Dockerfile", ContentKind: ui.CommitDetailContentBinary}
	commit := ui.NewCommitDetailWidget(h.dir, "commit", "abc1234", false)
	commit.SetDetail("Commit", []ui.CommitDetailFile{file}, "")
	current := ui.NewCurrentChangesWidget(h.dir, false)
	current.SetCurrentChanges("Working tree", []ui.CommitDetailFile{file}, "")
	h.app.EditorGroup.OpenPluginTab("commit-detail", "Commit details", commit)
	h.app.EditorGroup.OpenPluginTab("current-changes", "Current Changes", current)

	glyph := fileicons.ForFile("Dockerfile").Glyph
	h.exec("options.toggleFontIcons")
	h.assertContains(glyph)

	if !h.app.EditorGroup.SwitchToTabByPath("commit-detail") {
		t.Fatal("could not switch to inactive commit detail")
	}
	h.redraw()
	h.assertContains(glyph)

	h.exec("options.toggleFontIcons")
	h.assertNotContains(glyph)
	if !h.app.EditorGroup.SwitchToTabByPath("current-changes") {
		t.Fatal("could not switch to inactive current changes")
	}
	h.redraw()
	h.assertNotContains(glyph)
}

func TestSettingsFileIconsTurnOnAfterApply(t *testing.T) {
	h := openSettings(t)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	clickRowControl(t, h, "Sidebar", "Sidebar")
	label := "Icons"
	if !rowHas(h, label, "None") {
		t.Fatalf("%s should default to None:\n%s", label, h.screenText())
	}
	clickRowControl(t, h, label, "None")
	clickRowControl(t, h, "Nerd Font", "Nerd Font")
	if !rowHas(h, label, "Nerd Font") {
		t.Fatalf("%s did not switch to Nerd Font:\n%s", label, h.screenText())
	}
	if h.app.Settings.Appearance.Icons != config.IconsNone {
		t.Fatal("icon settings applied before Apply")
	}

	h.exec("settings.apply")
	if h.app.Settings.Appearance.Icons != config.IconsNerdFont {
		t.Fatalf("icon settings did not apply: %q", h.app.Settings.Appearance.Icons)
	}

	h.exec("sidebar.explorer")
	h.assertContains("alpha.txt")
	if !strings.Contains(h.screenText(), fileicons.ForFile("alpha.txt").Glyph) {
		t.Fatalf("explorer does not show file icons after Nerd Font:\n%s", h.screenText())
	}
	if err := os.WriteFile(filepath.Join(h.dir, "alpha.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	h.exec("sidebar.changes")
	h.assertContains("M " + fileicons.ForFile("alpha.txt").Glyph + " alpha.txt")
}

func TestOptionsMenuTogglesFileIcons(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	for _, menu := range [][]ui.ContextMenuItem{h.app.BuildOptionsMenu(), h.app.BuildChangesPanelMenu(), h.app.BuildChangesContextMenu()} {
		if item, ok := findMenuCommand(menu, "options.toggleFontIcons"); !ok || item.Checked != ui.MenuUnchecked {
			t.Fatalf("icon menus should carry an unchecked entry by default: item=%+v found=%v", item, ok)
		}
	}

	h.exec("options.toggleFontIcons")
	if item, _ := findMenuCommand(h.app.BuildOptionsMenu(), "options.toggleFontIcons"); h.app.Settings.Appearance.Icons != config.IconsNerdFont || item.Checked != ui.MenuChecked {
		t.Fatalf("icons after toggle = %q, menu checked %v", h.app.Settings.Appearance.Icons, item.Checked)
	}

	h.exec("options.toggleFontIcons")
	if h.app.Settings.Appearance.Icons != config.IconsNone {
		t.Fatalf("icons after second toggle = %q, want none", h.app.Settings.Appearance.Icons)
	}
}
