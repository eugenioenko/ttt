package app

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func buildTestApp(t *testing.T, settings config.Settings) *App {
	t.Helper()
	cfg := config.AppConfig{
		Keybindings: config.DefaultKeybindings(),
		Settings:    settings,
		Theme:       config.DefaultTheme(),
	}
	borders := BuildBorderSet(cfg.Theme.Borders)
	return BuildAppFromConfig(&cfg, &borders, workspace.New(nil), nil)
}

// The hidden state has to survive a restart, so it is applied while the widget
// tree is being built, not only on a live toggle.
func TestMenuBarHiddenAtStartup(t *testing.T) {
	settings := config.DefaultSettings()
	hidden := false
	settings.Editor.MenuBar = &hidden

	a := buildTestApp(t, settings)

	if a.MenuBar.Visible {
		t.Error("menu bar should be hidden at startup when the setting says so")
	}
	if len(a.RootBox.Children) != 2 {
		t.Errorf("hidden menu bar should leave the root stack: got %d children, want 2", len(a.RootBox.Children))
	}
	for _, child := range a.RootBox.Children {
		if child == a.MenuBar {
			t.Error("hidden menu bar is still in the root stack")
		}
	}
}

func TestMenuBarVisibleByDefault(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())

	if !a.MenuBar.Visible {
		t.Error("menu bar should be visible by default")
	}
	if len(a.RootBox.Children) != 3 || a.RootBox.Children[0] != a.MenuBar {
		t.Error("menu bar should be the first child of the root stack")
	}
}

// Anything remaining focused after being dropped from the tree would swallow
// key events with nothing on screen to explain it.
func TestHidingMenuBarMovesFocusAway(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Root.SetFocus(a.MenuBar)

	a.applyMenuBarVisibility(false)

	if a.Root.Focused == a.MenuBar {
		t.Error("focus should move off the menu bar when it is hidden")
	}
}

// Verifies the layout actually reflows: with the bar hidden, the row it used to
// own is drawn by whatever comes next.
func TestMenuBarHiddenReclaimsTopRow(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Root.SetSize(80, 24)

	rowText := func() string {
		cells := make([][]term.Cell, 24)
		for y := range cells {
			cells[y] = make([]term.Cell, 80)
		}
		a.Root.Render(cells)
		row := make([]rune, 0, 80)
		for _, c := range cells[0] {
			row = append(row, c.Ch)
		}
		return string(row)
	}

	visible := rowText()
	if !strings.Contains(visible, "File") {
		t.Fatalf("menu bar should be drawn on row 0, got %q", visible)
	}

	a.applyMenuBarVisibility(false)

	if hidden := rowText(); strings.Contains(hidden, "File") {
		t.Errorf("menu bar still drawn on row 0 after hiding: %q", hidden)
	}
}
