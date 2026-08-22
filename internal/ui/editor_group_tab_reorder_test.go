package ui

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/view"
)

func reorderTestTab(path string, preview bool) editorTab {
	return editorTab{
		FilePath: path,
		Buf:      &buffer.Buffer{Lines: []string{""}},
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     &undo.UndoStack{},
		Sel:      &selection.Selection{},
		Preview:  preview,
	}
}

func tabPaths(g *EditorGroupWidget) []string {
	paths := make([]string, len(g.tabs))
	for i := range g.tabs {
		paths[i] = g.tabs[i].FilePath
	}
	return paths
}

func TestEditorGroupMoveTabPreservesActiveDocumentAndCommitsPreview(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.tabs = []editorTab{
		reorderTestTab("one.go", true),
		reorderTestTab("two.go", false),
		reorderTestTab("three.go", false),
	}
	g.active = 1

	if !g.MoveTab(0, 2) {
		t.Fatal("MoveTab should reorder distinct unpinned positions")
	}
	want := []string{"two.go", "three.go", "one.go"}
	if got := tabPaths(g); !slices.Equal(got, want) {
		t.Fatalf("tab order = %v, want %v", got, want)
	}
	if g.active != 0 || g.tabs[g.active].FilePath != "two.go" {
		t.Fatalf("active tab = %d %q, want two.go at 0", g.active, g.tabs[g.active].FilePath)
	}
	if g.tabs[2].Preview {
		t.Fatal("dragged preview should become a regular tab")
	}
}

func TestEditorGroupMoveTabKeepsPinnedBoundary(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.tabs = []editorTab{
		reorderTestTab("pin-one.go", false),
		reorderTestTab("pin-two.go", false),
		reorderTestTab("one.go", false),
		reorderTestTab("two.go", false),
	}
	g.pinnedCount = 2
	g.active = 2

	if g.MoveTab(2, 0) {
		t.Fatal("unpinned tab should not cross into the pinned block")
	}
	if g.MoveTab(1, 3) {
		t.Fatal("pinned tab should not cross into the unpinned block")
	}
	if !g.MoveTab(0, 1) {
		t.Fatal("pinned tabs should reorder within their block")
	}
	want := []string{"pin-two.go", "pin-one.go", "one.go", "two.go"}
	if got := tabPaths(g); !slices.Equal(got, want) {
		t.Fatalf("tab order = %v, want %v", got, want)
	}
	if g.active != 2 || !g.CanMoveActiveTab(1) || g.CanMoveActiveTab(-1) {
		t.Fatalf("active movement availability is wrong: active=%d left=%v right=%v",
			g.active, g.CanMoveActiveTab(-1), g.CanMoveActiveTab(1))
	}
}

func TestEditorGroupEffectivePinnedTargetControlsPreviewCommit(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.tabs = []editorTab{
		reorderTestTab("pin-one.go", false),
		reorderTestTab("pin-two.go", false),
		reorderTestTab("preview.go", true),
		reorderTestTab("later.go", true),
	}
	g.pinnedCount = 2
	g.active = 2

	if g.MoveTab(2, 0) {
		t.Fatal("first unpinned tab should be an effective no-op at the boundary")
	}
	if !g.tabs[2].Preview {
		t.Fatal("an effective no-op should not commit a preview tab")
	}
	if got := g.NormalizeTabMoveTarget(3, 0); got != 2 {
		t.Fatalf("effective target = %d, want pinned boundary 2", got)
	}
	if !g.MoveTab(3, 0) {
		t.Fatal("a later unpinned tab should move to the pinned boundary")
	}
	if g.tabs[2].FilePath != "later.go" || g.tabs[2].Preview {
		t.Fatal("effective move should insert at the boundary and commit its preview")
	}
}
