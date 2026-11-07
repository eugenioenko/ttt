package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestQuickOpenSettingsApplyAndPersist(t *testing.T) {
	h := newTestHarness(t, 120, 50)
	path := filepath.Join(h.dir, ".quick-open-test")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	checkFile := func(want bool) {
		t.Helper()
		h.exec("file.quickOpen")
		palette := h.app.Root.TopOverlayWidget().(*ui.SelectDialogWidget)
		palette.Input.SetText("quick-open-test")
		if got := len(palette.Items) == 1 && palette.Items[0].ID == path; got != want {
			t.Fatalf("hidden file visible = %v, want %v: %v", got, want, palette.Items)
		}
		h.pressKey(tcell.KeyEscape, 0)
	}
	checkFile(false)
	h.exec("settings.openUI")
	clickRowControl(t, h, "Sidebar", "Sidebar")
	quickOpen := false
	for y, line := range strings.Split(h.screenText(), "\n") {
		if strings.Contains(line, "Quick Open") {
			quickOpen = true
		}
		if quickOpen && (strings.Contains(line, "Hidden files") || strings.Contains(line, "Git-ignored files")) {
			x := displayColumnOf(line, uncheckedBox)
			if x < 0 {
				t.Fatalf("Quick Open option should default to off: %s", line)
			}
			h.click(x+1, y)
		}
	}
	if !quickOpen {
		t.Fatalf("Quick Open section missing:\n%s", h.screenText())
	}
	if h.app.Settings.QuickOpen.ShowHidden || h.app.Settings.QuickOpen.ShowGitIgnored {
		t.Fatal("settings applied before Apply")
	}
	h.exec("settings.apply")
	want := config.QuickOpenSettings{ShowHidden: true, ShowGitIgnored: true}
	if h.app.Settings.QuickOpen != want || config.LoadSettings().QuickOpen != want {
		t.Fatal("Quick Open settings were not applied and persisted")
	}
	if h.app.Settings.Explorer != config.DefaultExplorerSettings() || h.app.Settings.Search != config.DefaultSearchSettings() {
		t.Fatal("Quick Open settings changed Explorer or search")
	}
	checkFile(true)
}
