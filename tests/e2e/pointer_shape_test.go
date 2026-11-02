package e2e

import (
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestPointerShapesAcrossComposedApp(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.ShowBottomPanel()
	h.redraw()

	root := h.app.Root
	pane := h.app.EditorGroup.Editor.GetRect()
	tabsY := h.app.EditorGroup.TabBar.GetRect().Y + 1
	tabX := displayColumnOf(h.screenRow(tabsY), "alpha.txt")
	divY := h.app.ContentSplit.DividerScreenY()
	if tabX < 0 {
		t.Fatalf("tab label not visible in %q", h.screenRow(tabsY))
	}

	for _, tt := range []struct {
		name string
		x, y int
		want string
	}{
		{"editor text", pane.X + h.app.EditorGroup.Editor.GutterWidth() + 2, pane.Y, term.PointerText},
		{"editor gutter", pane.X, pane.Y, term.PointerDefault},
		{"tab", tabX + 1, tabsY, term.PointerHand},
		{"panel divider", pane.X + 5, divY, term.PointerNSResize},
		{"sidebar divider", h.app.SplitPanel.DividerScreenX(), pane.Y, term.PointerEWResize},
	} {
		if got := root.PointerShapeAt(tt.x, tt.y); got != tt.want {
			t.Errorf("%s (%d,%d) = %q, want %q", tt.name, tt.x, tt.y, got, tt.want)
		}
	}
}

func TestPointerShapeFollowsTabDragAndEndsOnRelease(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	for _, name := range []string{"alpha.txt", "beta.txt"} {
		h.app.EditorGroup.OpenFile(filepath.Join(h.dir, name))
		h.app.EditorGroup.CommitActiveTab()
	}
	h.redraw()

	root := h.app.Root
	pane := h.app.EditorGroup.Editor.GetRect()
	textX := pane.X + h.app.EditorGroup.Editor.GutterWidth() + 2
	tabsY := h.app.EditorGroup.TabBar.GetRect().Y + 1
	betaX := displayColumnOf(h.screenRow(tabsY), "beta.txt")
	if betaX < 0 {
		t.Fatalf("tab label not visible in %q", h.screenRow(tabsY))
	}

	root.HandleEvent(tcell.NewEventMouse(betaX+1, tabsY, tcell.Button1, 0))
	root.HandleEvent(tcell.NewEventMouse(betaX+6, tabsY, tcell.Button1, 0))
	h.redraw()
	if got := root.PointerShapeAt(textX, pane.Y+3); got != term.PointerGrabbing {
		t.Fatalf("dragging over the editor = %q, want %q", got, term.PointerGrabbing)
	}
	root.HandleEvent(tcell.NewEventMouse(textX, pane.Y+3, tcell.ButtonNone, 0))
	h.redraw()
	if got := root.PointerShapeAt(textX, pane.Y+3); got != term.PointerText {
		t.Fatalf("after release = %q, want %q", got, term.PointerText)
	}
}

func TestPointerShapeIgnoresEditorBeneathOverlay(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.redraw()

	pane := h.app.EditorGroup.Editor.GetRect()
	x, y := pane.X+pane.W-3, pane.Y+pane.H-2
	if got := h.app.Root.PointerShapeAt(x, y); got != term.PointerText {
		t.Fatalf("before the overlay = %q, want %q", got, term.PointerText)
	}
	h.exec("command.palette")
	h.redraw()
	if !h.app.Root.HasOverlay() {
		t.Fatal("test setup: command palette did not open")
	}
	if got := h.app.Root.PointerShapeAt(x, y); got != term.PointerDefault {
		t.Fatalf("beneath the overlay = %q, want %q", got, term.PointerDefault)
	}
}
