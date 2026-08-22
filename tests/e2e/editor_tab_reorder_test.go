package e2e

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestEditorTabDragReordersThroughRootAndKeepsActiveFile(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	for _, name := range []string{"alpha.txt", "beta.txt", "gamma.txt"} {
		h.app.EditorGroup.OpenFile(filepath.Join(h.dir, name))
		h.app.EditorGroup.CommitActiveTab()
	}
	h.redraw()

	tabsY := h.app.EditorGroup.TabBar.GetRect().Y + 1
	row := h.screenRow(tabsY)
	gammaX := strings.Index(row, "gamma.txt")
	if !strings.Contains(row, "alpha.txt") || gammaX < 0 {
		t.Fatalf("tab labels not visible in %q", row)
	}

	if got := h.app.Root.HandleEvent(tcell.NewEventMouse(gammaX+2, tabsY, tcell.Button1, 0)); got != ui.EventConsumed {
		t.Fatalf("mouse down result = %v, want EventConsumed", got)
	}
	dropX := h.app.EditorGroup.TabBar.GetRect().X
	h.app.Root.HandleEvent(tcell.NewEventMouse(dropX, tabsY, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(dropX, tabsY, tcell.ButtonNone, 0))
	h.redraw()

	path0, _ := h.app.EditorGroup.TabInfo(0)
	if !strings.HasSuffix(path0, "gamma.txt") {
		t.Fatalf("first tab = %q, want gamma.txt", path0)
	}
	if got := h.app.EditorGroup.ActiveFilePath(); !strings.HasSuffix(got, "gamma.txt") {
		t.Fatalf("active file = %q, want gamma.txt", got)
	}

	h.exec("tab.moveRight")
	path1, _ := h.app.EditorGroup.TabInfo(1)
	if !strings.HasSuffix(path1, "gamma.txt") {
		t.Fatalf("command fallback moved tab to %q, want gamma.txt at index 1", path1)
	}
}

func TestEditorPendingTabDragOwnsCrossContentSplitRelease(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	for _, name := range []string{"alpha.txt", "beta.txt", "gamma.txt"} {
		h.app.EditorGroup.OpenFile(filepath.Join(h.dir, name))
		h.app.EditorGroup.CommitActiveTab()
	}
	h.app.ShowBottomPanel()
	h.redraw()

	tabsY := h.app.EditorGroup.TabBar.GetRect().Y + 1
	row := h.screenRow(tabsY)
	gammaX := strings.Index(row, "gamma.txt")
	if gammaX < 0 {
		t.Fatalf("gamma tab not visible in %q", row)
	}

	h.app.Root.HandleEvent(tcell.NewEventMouse(gammaX+2, tabsY, tcell.Button1, 0))
	bottomY := h.app.ContentSplit.DividerScreenY() + 2
	h.app.Root.HandleEvent(tcell.NewEventMouse(gammaX+2, bottomY, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(gammaX+2, bottomY, tcell.ButtonNone, 0))
	h.redraw()

	if h.app.EditorGroup.TabBar.OwnsPointerCapture() {
		t.Fatal("cross-content release left a stale editor tab gesture")
	}
	afterRelease := make([]string, h.app.EditorGroup.TabCount())
	for i := range afterRelease {
		afterRelease[i], _ = h.app.EditorGroup.TabInfo(i)
	}
	row = h.screenRow(tabsY)
	alphaX := strings.Index(row, "alpha.txt")
	if alphaX < 0 {
		t.Fatalf("alpha tab not visible after release in %q", row)
	}
	h.click(alphaX+2, tabsY)

	if got := h.app.EditorGroup.ActiveFilePath(); !strings.HasSuffix(got, "alpha.txt") {
		t.Fatalf("active file = %q, want alpha.txt", got)
	}
	afterClick := make([]string, h.app.EditorGroup.TabCount())
	for i := range afterClick {
		afterClick[i], _ = h.app.EditorGroup.TabInfo(i)
	}
	if !slices.Equal(afterClick, afterRelease) {
		t.Fatalf("ordinary alpha click reordered tabs: got %v, want %v", afterClick, afterRelease)
	}
}
