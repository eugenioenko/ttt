package app

import (
	"slices"
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

func TestOpenFileMenuItemImmediatelyPrecedesOpenFolder(t *testing.T) {
	fileMenu := menuBarMenus[0]
	for i, item := range fileMenu {
		if item.Command != "file.open" {
			continue
		}
		if item.Label != "Open File" || i+1 >= len(fileMenu) ||
			fileMenu[i+1].Label != "Open Folder" || fileMenu[i+1].Command != "workspace.openFolder" {
			t.Fatalf("Open File menu item = %+v, next item = %+v; want it immediately before Open Folder", item, fileMenu[min(i+1, len(fileMenu)-1)])
		}
		return
	}
	t.Fatal("File menu has no Open File command")
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

func TestOptionsMenuGroupsControls(t *testing.T) {
	items := buildTestApp(t, config.DefaultSettings()).BuildOptionsMenu()
	wantLabels := []string{"Editor", "Code Assist", "Appearance", "Diff Views", "Git Files", "", "Theme", "Settings"}
	var labels []string
	for _, item := range items {
		labels = append(labels, item.Label)
	}
	if !slices.Equal(labels, wantLabels) {
		t.Fatalf("Options labels = %v, want %v", labels, wantLabels)
	}
	for _, item := range items[:5] {
		if len(item.Submenu) == 0 || item.Command != "" || item.Checked != 0 {
			t.Errorf("category should only open a submenu: %+v", item)
		}
	}
	if !items[5].IsSep || items[6].Command != "theme.switch" || items[7].Command != "settings.openUI" {
		t.Fatal("Theme and Settings should follow the final separator")
	}
	wantCommands := [][]string{
		{"options.toggleWordWrap", "options.toggleLineNumbers", "options.toggleSyntaxHighlight", "options.toggleBracketColors", "", "options.toggleAutoIndent", "options.toggleAutoDedent", "", "options.indentation", "", "options.toggleGitGutter", "options.toggleScrollbarGitMarks"},
		{"options.toggleLSP", "options.toggleLSPHover"},
		{"options.toggleFontIcons", menuBarToggleCommand, "options.toggleTransparentBackground", "", "options.gutterStyle", "options.borderStyle"},
	}
	for i, want := range wantCommands {
		var commands []string
		for _, item := range items[i].Submenu {
			commands = append(commands, item.Command)
			if len(item.Submenu) != 0 || (item.Command == "" && !item.IsSep) {
				t.Errorf("unexpected nested or inactive control: %+v", item)
			}
			if strings.HasPrefix(item.Command, "options.toggle") && item.Checked == 0 {
				t.Errorf("toggle %q has no checked indicator", item.Label)
			}
		}
		if !slices.Equal(commands, want) {
			t.Errorf("%s commands = %v, want %v", items[i].Label, commands, want)
		}
	}
	for _, item := range menuBarMenus[3] {
		if item.Command == "theme.switch" {
			t.Error("Theme should move from View to Options")
		}
	}
	assist := items[1].Submenu
	if assist[0].Label != "Enable Code Assist" || assist[1].Label != "Hover Information" {
		t.Fatalf("Code Assist labels = %+v", assist)
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
