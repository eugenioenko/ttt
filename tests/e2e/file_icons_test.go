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

func TestFileIconsShowByDefaultInExplorerAndChanges(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)

	txt := fileicons.ForFile("alpha.txt").Glyph
	h.exec("sidebar.explorer")
	h.assertContains(txt + " alpha.txt")
	h.assertContains(fileicons.ForFolder(false).Glyph + " subdir")

	if err := os.WriteFile(filepath.Join(h.dir, "alpha.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	h.exec("sidebar.changes")
	h.assertContains("M " + txt + " alpha.txt")
}

func TestSettingsFileIconsTurnOffAfterApply(t *testing.T) {
	h := openSettings(t)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	clickRowControl(t, h, "Advanced", "Advanced")
	for _, label := range []string{"Explorer: file icons", "Git: file icons"} {
		if !rowHas(h, label, "Nerd Font") {
			t.Fatalf("%s should default to Nerd Font:\n%s", label, h.screenText())
		}
		clickRowControl(t, h, label, "Nerd Font")
		clickRowControl(t, h, "None", "None")
		if !rowHas(h, label, "None") {
			t.Fatalf("%s did not switch to None:\n%s", label, h.screenText())
		}
	}
	if h.app.Settings.Explorer.Icons != config.IconsNerdFont || h.app.Settings.Git.Icons != config.IconsNerdFont {
		t.Fatal("icon settings applied before Apply")
	}

	h.exec("settings.apply")
	if h.app.Settings.Explorer.Icons != config.IconsNone || h.app.Settings.Git.Icons != config.IconsNone {
		t.Fatalf("icon settings did not apply: explorer=%q git=%q", h.app.Settings.Explorer.Icons, h.app.Settings.Git.Icons)
	}

	h.exec("sidebar.explorer")
	h.assertContains("alpha.txt")
	if strings.Contains(h.screenText(), fileicons.ForFile("alpha.txt").Glyph) {
		t.Fatalf("explorer still shows file icons after None:\n%s", h.screenText())
	}
	if err := os.WriteFile(filepath.Join(h.dir, "alpha.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	h.exec("sidebar.changes")
	h.assertContains("M alpha.txt")
}

func TestOptionsMenuTogglesFileIcons(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("options.toggleExplorerIcons")
	if h.app.Settings.Explorer.Icons != config.IconsNone {
		t.Fatalf("explorer icons after toggle = %q, want none", h.app.Settings.Explorer.Icons)
	}
	h.exec("options.toggleExplorerIcons")
	if h.app.Settings.Explorer.Icons != config.IconsNerdFont {
		t.Fatalf("explorer icons after second toggle = %q, want nerd-font", h.app.Settings.Explorer.Icons)
	}

	for _, menu := range [][]ui.ContextMenuItem{h.app.BuildOptionsMenu(), h.app.BuildChangesPanelMenu(), h.app.BuildChangesContextMenu()} {
		if item, ok := findMenuCommand(menu, "options.toggleGitIcons"); !ok || item.Checked != ui.MenuChecked {
			t.Fatalf("Git Files menus should carry a checked File Icons entry: item=%+v found=%v", item, ok)
		}
	}
	h.exec("options.toggleGitIcons")
	if item, _ := findMenuCommand(h.app.BuildOptionsMenu(), "options.toggleGitIcons"); h.app.Settings.Git.Icons != config.IconsNone || item.Checked != ui.MenuUnchecked {
		t.Fatalf("git icons after toggle = %q, menu checked %v", h.app.Settings.Git.Icons, item.Checked)
	}
}
