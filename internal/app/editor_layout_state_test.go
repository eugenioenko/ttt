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

func buildLayoutStateApp(t *testing.T, dir string, files []FileTarget) *App {
	t.Helper()
	cfg := config.AppConfig{Settings: config.DefaultSettings(), Theme: config.DefaultTheme(), Keybindings: config.DefaultKeybindings()}
	borders := BuildBorderSet(cfg.Theme.Borders)
	a := BuildAppFromConfig(&cfg, &borders, workspace.New([]string{dir}), files)
	a.Reg = command.NewRegistry()
	running := true
	a.Running = &running
	RegisterCommands(a)
	a.Root.SetSize(160, 42)
	var renderer render.Renderer
	a.Root.Render(renderer.NextFrame(160, 42))
	t.Cleanup(func() {
		a.restoringEditorLayout = true
		a.CloseTasks()
		for _, group := range a.editorGroups() {
			group.OnEmpty = nil
			group.CloseAllTabs()
		}
	})
	return a
}

func TestEditorLayoutRestoresWorkspaceViewsAndRatios(t *testing.T) {
	previous := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = previous })
	dir := t.TempDir()
	path := filepath.Join(dir, "alpha.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("hello world\n", 100)), 0644); err != nil {
		t.Fatal(err)
	}
	a := buildLayoutStateApp(t, dir, nil)
	left := a.EditorGroup
	left.OpenFile(path)
	left.TogglePinTab()
	a.SplitEditor()
	right := a.EditorGroup
	a.SplitEditorDown()
	bottom := a.EditorGroup
	left.GoToLineCol(50, 4)
	left.Editor.Viewport.TopLine = 45
	right.GoToLineCol(8, 2)
	bottom.GoToLineCol(80, 7)
	a.EditorSplit.Split.BottomRatio = 0.37
	a.forceQuit()

	restored := buildLayoutStateApp(t, dir, nil)
	groups := restored.editorGroups()
	if len(groups) != 3 || restored.EditorSplit.Split.BottomRatio != 0.37 || restored.EditorSplit.Second.Split == nil {
		t.Fatal("workspace layout shape or divider ratio was not restored")
	}
	if groups[0].ActiveBuffer() != groups[1].ActiveBuffer() || groups[1].ActiveBuffer() != groups[2].ActiveBuffer() {
		t.Fatal("restored views do not share their document")
	}
	if line, col := groups[0].ActiveCursor(); line != 49 || col != 3 || groups[0].Editor.Viewport.TopLine != 45 || !groups[0].IsActiveTabPinned() {
		t.Fatalf("first view was not restored: cursor %d:%d top %d", line, col, groups[0].Editor.Viewport.TopLine)
	}
	if restored.EditorGroup != groups[2] {
		t.Fatal("active editor group was not restored")
	}
	if line, col := groups[2].ActiveCursor(); line != 79 || col != 6 {
		t.Fatalf("last view cursor = %d:%d", line, col)
	}

	explicit := buildLayoutStateApp(t, dir, []FileTarget{{Path: path, Line: 3, Col: 2}})
	if len(explicit.editorGroups()) != 1 {
		t.Fatal("explicit file arguments unexpectedly restored a previous session")
	}
	other := buildLayoutStateApp(t, t.TempDir(), nil)
	if len(other.editorGroups()) != 1 {
		t.Fatal("another workspace inherited the saved layout")
	}
}

func TestEditorLayoutRejectsInvalidStateAndSkipsMissingFiles(t *testing.T) {
	previous := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = previous })
	dir := t.TempDir()
	state := config.State{EditorLayouts: map[string]*config.EditorLayoutState{dir: {
		Direction: "right", Ratio: 4,
		First:  &config.EditorLayoutState{Views: []config.EditorViewState{{Path: filepath.Join(dir, "deleted.txt")}}},
		Second: &config.EditorLayoutState{Focused: true},
	}}}
	if err := config.SaveState(state); err != nil {
		t.Fatal(err)
	}
	a := buildLayoutStateApp(t, dir, nil)
	if len(a.editorGroups()) != 2 || a.EditorSplit.Split.BottomRatio != 0.5 {
		t.Fatal("missing files or invalid ratio broke layout restoration")
	}
	if a.editorGroups()[0].TabCount() != 1 {
		t.Fatal("missing file produced an extra tab")
	}
	if validEditorLayout(&config.EditorLayoutState{Direction: "right"}, 0) != 0 || validEditorLayout(state.EditorLayouts[dir], 5) != 0 {
		t.Fatal("incomplete or excessively nested layout was accepted")
	}
}
