package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func sidebarFocusTestApp(t *testing.T) *App {
	t.Helper()
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	cfg := config.AppConfig{
		Keybindings: config.DefaultKeybindings(),
		Settings:    config.DefaultSettings(),
		Theme:       config.DefaultTheme(),
	}
	borders := BuildBorderSet(cfg.Theme.Borders)
	return BuildAppFromConfig(&cfg, &borders, workspace.New([]string{t.TempDir()}), nil)
}

func TestHideSidebarMovesFocusToEditor(t *testing.T) {
	a := sidebarFocusTestApp(t)
	a.FocusSidebar()
	if a.focusedRegion() != "sidebar" {
		t.Fatal("setup: sidebar not focused")
	}
	a.ToggleSidebar()
	if a.Root.Focused != a.EditorGroup {
		t.Fatalf("focus = %T, want editor", a.Root.Focused)
	}
}

func TestHideSidebarKeepsFocusOutsideSidebar(t *testing.T) {
	a := sidebarFocusTestApp(t)
	a.FocusPanel()
	want := a.Root.Focused
	if a.focusedRegion() != "bottom" {
		t.Fatal("setup: bottom panel not focused")
	}
	a.ToggleSidebar()
	if a.Root.Focused != want {
		t.Fatalf("focus = %T, want unchanged %T", a.Root.Focused, want)
	}
}
