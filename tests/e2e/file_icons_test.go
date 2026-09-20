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
	h.exec("sidebar.explorer")
	h.assertContains(txt + " alpha.txt")
	h.assertContains("▶ subdir")

	if err := os.WriteFile(filepath.Join(h.dir, "alpha.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	h.exec("sidebar.changes")
	h.assertContains("M " + txt + " alpha.txt")
}

func TestSettingsFileIconsTurnOnAfterApply(t *testing.T) {
	h := openSettings(t)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	clickRowControl(t, h, "Advanced", "Advanced")
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
