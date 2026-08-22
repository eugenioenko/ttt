package e2e

import (
	"path/filepath"
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
