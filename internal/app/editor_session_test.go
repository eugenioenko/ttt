package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func sessionTestApp(t *testing.T, dir string, enabled bool, files ...FileTarget) *App {
	t.Helper()
	cfg := config.AppConfig{Settings: config.DefaultSettings(), Theme: config.DefaultTheme(), Keybindings: config.DefaultKeybindings()}
	cfg.Settings.Editor.RestoreTabs = enabled
	borders := BuildBorderSet(cfg.Theme.Borders)
	a := BuildAppFromConfig(&cfg, &borders, workspace.New([]string{dir}), files)
	a.Reg = command.NewRegistry()
	running := true
	a.Running = &running
	RegisterCommands(a)
	a.Root.SetSize(120, 32)
	var renderer render.Renderer
	a.Root.Render(renderer.NextFrame(120, 32))
	t.Cleanup(a.CloseTasks)
	return a
}

func TestEditorSessionRestoresTabsPositionsAndPins(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	dir := t.TempDir()
	first, second := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte(strings.Repeat("hello world\n", 100)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a := sessionTestApp(t, dir, true)
	if err := a.EditorGroup.OpenFile(first); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.TogglePinTab()
	a.EditorGroup.GoToLineCol(50, 4)
	a.EditorGroup.Editor.Viewport.TopLine = 45
	if err := a.EditorGroup.OpenFile(second); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.SwitchToTabByPath(first)
	a.forceQuit()
	restored := sessionTestApp(t, dir, true)
	g := restored.EditorGroup
	if g.TabCount() != 2 || g.ActiveFilePath() != first || !g.IsActiveTabPinned() {
		t.Fatal("tabs, active file, or pin were not restored")
	}
	if line, col := g.ActiveCursor(); line != 49 || col != 3 || g.Editor.Viewport.TopLine != 45 {
		t.Fatalf("restored cursor %d:%d, scroll %d", line, col, g.Editor.Viewport.TopLine)
	}
	if disabled := sessionTestApp(t, dir, false); disabled.EditorGroup.TabCount() != 1 || disabled.EditorGroup.ActiveFilePath() != "untitled" {
		t.Fatal("disabled setting restored tabs")
	}
	if explicit := sessionTestApp(t, dir, true, FileTarget{Path: second}); explicit.EditorGroup.TabCount() != 1 || explicit.EditorGroup.ActiveFilePath() != second {
		t.Fatal("saved session overrode explicit file arguments")
	}
	if other := sessionTestApp(t, t.TempDir(), true); other.EditorGroup.TabCount() != 1 || other.EditorGroup.ActiveFilePath() != "untitled" {
		t.Fatal("another workspace inherited saved tabs")
	}
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	cleaned := sessionTestApp(t, dir, true)
	if tabs, err := config.LoadEditorSession(cleaned.sessionWorkspaceKey); err != nil || len(tabs) != 1 {
		t.Fatalf("missing file was not cleaned: %+v, %v", tabs, err)
	}
	cleaned.ClearEditorSessions()
	s := *cleaned.Settings
	s.Editor.WordWrap = true
	cleaned.ApplySettings(s)
	cleaned.forceQuit()
	if tabs, err := config.LoadEditorSession(cleaned.sessionWorkspaceKey); err != nil || len(tabs) != 0 || cleaned.EditorGroup.ActiveFilePath() != first {
		t.Fatal("clear closed open tabs or recreated the session on exit")
	}
}
