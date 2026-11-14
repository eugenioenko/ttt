package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestEditorSessionRestoresSplitTreeAndIndependentViews(t *testing.T) {
	for _, focused := range []int{0, 2} {
		t.Run([]string{"first", "last"}[focused/2], func(t *testing.T) {
			config.OverrideConfigDir = t.TempDir()
			t.Cleanup(func() { config.OverrideConfigDir = "" })
			dir := t.TempDir()
			paths := []string{filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go"), filepath.Join(dir, "c.go")}
			for _, path := range paths {
				if err := os.WriteFile(path, []byte(strings.Repeat("hello world\n", 100)), 0600); err != nil {
					t.Fatal(err)
				}
			}
			a := sessionTestApp(t, dir, true)
			first := a.EditorGroup
			if err := first.OpenFile(paths[0]); err != nil {
				t.Fatal(err)
			}
			first.CommitActiveTab()
			first.TogglePinTab()
			first.GoToLineCol(12, 3)
			first.Editor.Viewport.TopLine = 8
			a.SplitEditor()
			if err := a.EditorGroup.OpenFile(paths[1]); err != nil {
				t.Fatal(err)
			}
			a.EditorGroup.CommitActiveTab()
			a.SplitEditorDown()
			last := a.EditorGroup
			if err := last.OpenFile(paths[2]); err != nil {
				t.Fatal(err)
			}
			last.CommitActiveTab()
			last.TogglePinTab()
			if err := last.OpenFile(paths[0]); err != nil {
				t.Fatal(err)
			}
			last.CommitActiveTab()
			last.GoToLineCol(70, 6)
			last.Editor.Viewport.TopLine = 66
			a.EditorSplit.Split.BottomRatio = 0.35
			a.EditorSplit.Second.Split.BottomRatio = 0.6
			a.Root.SetFocus(a.editorGroups()[focused])
			a.forceQuit()
			restored := sessionTestApp(t, dir, true)
			groups := restored.editorGroups()
			if len(groups) != 3 || restored.EditorSplit.Split.Position != ui.SplitRight || restored.EditorSplit.Second.Split.Position != ui.SplitBottom {
				t.Fatal("split tree was not restored")
			}
			if restored.EditorSplit.Split.BottomRatio != 0.35 || restored.EditorSplit.Second.Split.BottomRatio != 0.6 {
				t.Fatal("divider proportions were not restored")
			}
			if restored.EditorGroup != groups[focused] || restored.Root.Focused != groups[focused] {
				t.Fatal("focused split was not restored")
			}
			if groups[0].ActiveFilePath() != paths[0] || groups[1].ActiveFilePath() != paths[1] || groups[2].ActiveFilePath() != paths[0] || groups[2].TabCount() != 2 {
				t.Fatal("tabs were not restored into their original groups")
			}
			if !groups[0].IsActiveTabPinned() || groups[2].IsActiveTabPinned() {
				t.Fatal("pin state leaked between shared views")
			}
			if groups[0].ActiveBuffer() != groups[2].ActiveBuffer() || groups[0].Editor.Undo != groups[2].Editor.Undo {
				t.Fatal("restored views of the same file do not share their document")
			}
			for _, view := range []struct{ group, line, col, top int }{{0, 11, 2, 8}, {2, 69, 5, 66}} {
				g := groups[view.group]
				line, col := g.ActiveCursor()
				if line != view.line || col != view.col || g.Editor.Viewport.TopLine != view.top {
					t.Fatalf("group %d: cursor %d:%d, scroll %d", view.group, line, col, g.Editor.Viewport.TopLine)
				}
			}
			if disabled := sessionTestApp(t, dir, false); len(disabled.editorGroups()) != 1 {
				t.Fatal("disabled restoration recreated splits")
			}
			if explicit := sessionTestApp(t, dir, true, FileTarget{Path: paths[1]}); len(explicit.editorGroups()) != 1 || explicit.EditorGroup.ActiveFilePath() != paths[1] {
				t.Fatal("saved layout overrode explicit file arguments")
			}
			if err := os.Remove(paths[2]); err != nil {
				t.Fatal(err)
			}
			cleaned := sessionTestApp(t, dir, true)
			if len(cleaned.editorGroups()) != 3 || cleaned.editorGroups()[2].TabCount() != 1 {
				t.Fatal("missing files were not skipped within the restored layout")
			}
		})
	}
}

func TestEditorSessionRestoresEmptySplitsWithoutUntitledContents(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	dir := t.TempDir()
	a := sessionTestApp(t, dir, true)
	a.SplitEditor()
	a.SplitEditorDown()
	a.SplitEditorDown()
	a.EditorGroup.NewFileFromText("unsaved text")
	a.forceQuit()
	restored := sessionTestApp(t, dir, true)
	groups := restored.editorGroups()
	if len(groups) != 4 {
		t.Fatal("empty split layout was discarded")
	}
	for i, group := range groups {
		if group.TabCount() != 1 || group.ActiveBuffer().Lines[0] != "" {
			t.Fatal("untitled text was persisted")
		}
		for _, other := range groups[:i] {
			if group.ActiveBuffer() == other.ActiveBuffer() || group.ActiveFilePath() == other.ActiveFilePath() {
				t.Fatal("restored empty groups shared an untitled buffer or name")
			}
		}
	}
}
