package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

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
	gammaX := displayColumnOf(row, "gamma.txt")
	if displayColumnOf(row, "alpha.txt") < 0 || gammaX < 0 {
		t.Fatalf("tab labels not visible in %q", row)
	}

	if got := h.app.Root.HandleEvent(tcell.NewEventMouse(gammaX+2, tabsY, tcell.Button1, 0)); got != ui.EventConsumed {
		t.Fatalf("mouse down result = %v, want EventConsumed", got)
	}
	if got := filepath.Base(h.app.EditorGroup.ActiveFilePath()); got != "gamma.txt" {
		t.Fatalf("pressed source = %q, want gamma.txt", got)
	}
	dropX := h.app.EditorGroup.TabBar.GetRect().X
	h.app.Root.HandleEvent(tcell.NewEventMouse(dropX, tabsY, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(dropX, tabsY, tcell.ButtonNone, 0))
	h.redraw()

	path0, _ := h.app.EditorGroup.TabInfo(0)
	if filepath.Base(path0) != "gamma.txt" {
		t.Fatalf("first tab = %q, want gamma.txt", path0)
	}
	if got := h.app.EditorGroup.ActiveFilePath(); filepath.Base(got) != "gamma.txt" {
		t.Fatalf("active file = %q, want gamma.txt", got)
	}

	h.exec("tab.moveRight")
	path1, _ := h.app.EditorGroup.TabInfo(1)
	if filepath.Base(path1) != "gamma.txt" {
		t.Fatalf("command fallback moved tab to %q, want gamma.txt at index 1", path1)
	}
}

func TestEditorRemovedDragSourceDoesNotRetargetSameIndex(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	for _, name := range []string{"alpha.txt", "beta.txt", "gamma.txt"} {
		h.app.EditorGroup.OpenFile(filepath.Join(h.dir, name))
		h.app.EditorGroup.CommitActiveTab()
	}
	h.redraw()

	tabsY := h.app.EditorGroup.TabBar.GetRect().Y + 1
	betaX := displayColumnOf(h.screenRow(tabsY), "beta.txt")
	if betaX < 0 {
		t.Fatal("beta tab is not visible")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(betaX+2, tabsY, tcell.Button1, 0))
	if got := filepath.Base(h.app.EditorGroup.ActiveFilePath()); got != "beta.txt" {
		t.Fatalf("pressed source = %q, want beta.txt", got)
	}

	h.app.EditorGroup.CloseTab()
	postRemoval := editorTabPaths(h)
	if slices.ContainsFunc(postRemoval, func(path string) bool { return filepath.Base(path) == "beta.txt" }) {
		t.Fatal("test setup did not remove beta")
	}
	if h.app.Root.PointerCaptureActive() || h.app.EditorGroup.TabBar.OwnsPointerCapture() {
		t.Fatal("removing beta retained pointer capture")
	}
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "beta.txt"))
	h.app.EditorGroup.CommitActiveTab()
	postReopen := editorTabPaths(h)
	h.app.Root.HandleEvent(tcell.NewEventMouse(betaX+20, tabsY, tcell.ButtonNone, 0))
	if got := editorTabPaths(h); !slices.Equal(got, postReopen) {
		t.Fatalf("release after rapid beta reopen reordered tabs: got %v, want %v", got, postReopen)
	}
}

func TestEditorFullscreenHideCancelsQueuedAutoScrollGeneration(t *testing.T) {
	h := newTestHarness(t, 70, 24)
	defer h.stop()
	for i := range 12 {
		name := fmt.Sprintf("overflow-%02d.txt", i)
		path := filepath.Join(h.dir, name)
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		h.app.EditorGroup.OpenFile(path)
		h.app.EditorGroup.CommitActiveTab()
	}
	first := filepath.Join(h.dir, "overflow-00.txt")
	if !h.app.EditorGroup.SwitchToTabByPath(first) {
		t.Fatal("could not activate first overflow tab")
	}
	h.redraw()

	ticks := make(chan uint64, 2)
	h.app.EditorGroup.TabBar.PostDragAutoScrollTick = func(generation uint64) { ticks <- generation }
	tabsRect := h.app.EditorGroup.TabBar.GetRect()
	tabsY := tabsRect.Y + 1
	firstX := displayColumnOf(h.screenRow(tabsY), "overflow-00.txt")
	if firstX < 0 {
		t.Fatal("first overflow tab is not visible")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(firstX+2, tabsY, tcell.Button1, 0))
	if got := filepath.Base(h.app.EditorGroup.ActiveFilePath()); got != "overflow-00.txt" {
		t.Fatalf("pressed source = %q, want overflow-00.txt", got)
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(tabsRect.X+tabsRect.W-6, tabsY, tcell.Button1, 0))

	var generation uint64
	select {
	case generation = <-ticks:
	case <-time.After(time.Second):
		t.Fatal("edge drag did not queue an auto-scroll tick")
	}
	h.app.ContentSplit.ShowBottom = true
	h.app.ContentSplit.BottomH = h.app.ContentSplit.GetRect().H - 1
	if h.app.EditorGroup.TabBar.HandleDragAutoScrollTick(generation) {
		t.Fatal("fullscreen-hidden editor accepted an old auto-scroll generation")
	}
	if h.app.Root.PointerCaptureActive() || h.app.EditorGroup.TabBar.OwnsPointerCapture() {
		t.Fatal("fullscreen-hidden editor retained pointer capture")
	}
	h.redraw()
	h.app.Root.HandleEvent(tcell.NewEventMouse(firstX+20, tabsY, tcell.ButtonNone, 0))
	select {
	case next := <-ticks:
		t.Fatalf("old auto-scroll generation rearmed tick %d", next)
	case <-time.After(150 * time.Millisecond):
	}
}

func editorTabPaths(h *testHarness) []string {
	paths := make([]string, h.app.EditorGroup.TabCount())
	for i := range paths {
		paths[i], _ = h.app.EditorGroup.TabInfo(i)
	}
	return paths
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
	gammaX := displayColumnOf(row, "gamma.txt")
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
	alphaX := displayColumnOf(row, "alpha.txt")
	if alphaX < 0 {
		t.Fatalf("alpha tab not visible after release in %q", row)
	}
	h.click(alphaX+2, tabsY)

	if got := h.app.EditorGroup.ActiveFilePath(); filepath.Base(got) != "alpha.txt" {
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
