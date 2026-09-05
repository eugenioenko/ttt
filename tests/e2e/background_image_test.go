package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

// The SimScreen used by newTestHarness always reports Tty() as unavailable
// (internal/term/sim_screen.go), so these tests exercise ApplySettings'
// settings-diff and style-map plumbing, not the actual Kitty-protocol byte
// output (covered by internal/term/kittygfx's unit tests instead).

func TestApplySettingsPersistsBackgroundImage(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	s := *h.app.Settings
	s.Editor.BackgroundImage = "/tmp/wallpaper.png"
	s.Editor.BackgroundImageDim = 40
	h.app.ApplySettings(s)

	if got := h.app.Settings.Editor.BackgroundImage; got != "/tmp/wallpaper.png" {
		t.Errorf("BackgroundImage = %q, want %q", got, "/tmp/wallpaper.png")
	}
	if got := h.app.Settings.Editor.BackgroundImageDim; got != 40 {
		t.Errorf("BackgroundImageDim = %d, want 40", got)
	}
}

func TestApplySettingsBackgroundImageForcesTransparentStyle(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	s := *h.app.Settings
	s.Editor.TransparentBackground = false
	s.Editor.BackgroundImage = "/tmp/wallpaper.png"
	h.app.ApplySettings(s)

	base := h.app.Screen.GetStyleMap()[term.StyleDefault]
	if bg := base.GetBackground(); bg != tcell.ColorDefault {
		t.Errorf("expected unset background when BackgroundImage is set without TransparentBackground, got %v", bg)
	}
}

func TestApplySettingsNoBackgroundImageKeepsThemeBackground(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	s := *h.app.Settings
	s.Editor.TransparentBackground = false
	s.Editor.BackgroundImage = ""
	h.app.ApplySettings(s)

	base := h.app.Screen.GetStyleMap()[term.StyleDefault]
	if bg := base.GetBackground(); bg == tcell.ColorDefault {
		t.Errorf("expected theme background to be set when no background image is configured")
	}
}
