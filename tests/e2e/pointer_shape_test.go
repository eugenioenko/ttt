package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
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
	if got := root.PointerShapeAt(1, pane.Y+3); got != term.PointerGrabbing {
		t.Fatalf("dragging over the sidebar = %q, want %q", got, term.PointerGrabbing)
	}
	if got := root.PointerShapeAt(10, 0); got != term.PointerGrabbing {
		t.Fatalf("dragging over the menu bar = %q, want %q", got, term.PointerGrabbing)
	}
	if got := root.PointerShapeAt(10, 29); got != term.PointerGrabbing {
		t.Fatalf("dragging over the status bar = %q, want %q", got, term.PointerGrabbing)
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

func TestMouseInteractionsWorkWithPointerShapesDisabled(t *testing.T) {
	disabled := false

	t.Run("selection", func(t *testing.T) {
		h := newTestHarness(t, 120, 30)
		defer h.stop()

		s := *h.app.Settings
		s.Appearance.PointerShapes = &disabled
		h.app.ApplySettings(s)
		if h.app.Screen.PointerShapesEnabled() {
			t.Fatal("expected Screen.PointerShapesEnabled() to be false")
		}

		f := filepath.Join(h.dir, "test.txt")
		_ = os.WriteFile(f, []byte("hello world\nline 2\nline 3\n"), 0644)
		h.app.EditorGroup.OpenFile(f)
		h.app.EditorGroup.CommitActiveTab()
		h.redraw()

		e := h.app.EditorGroup.Editor
		r := e.GetRect()
		textX := r.X + e.GutterWidth() + 2
		textY := r.Y + 2
		h.app.Root.HandleEvent(tcell.NewEventMouse(textX, textY, tcell.Button1, 0))
		h.app.Root.HandleEvent(tcell.NewEventMouse(textX+5, textY, tcell.Button1, 0))
		h.app.Root.HandleEvent(tcell.NewEventMouse(textX+5, textY, tcell.ButtonNone, 0))
		if !e.Selection.Active {
			t.Error("expected text selection to work with pointer shapes disabled")
		}
	})

	t.Run("tab_drag", func(t *testing.T) {
		h := newTestHarness(t, 120, 30)
		defer h.stop()

		s := *h.app.Settings
		s.Appearance.PointerShapes = &disabled
		h.app.ApplySettings(s)

		for _, name := range []string{"alpha.txt", "beta.txt", "gamma.txt"} {
			h.app.EditorGroup.OpenFile(filepath.Join(h.dir, name))
			h.app.EditorGroup.CommitActiveTab()
		}
		h.redraw()

		tabsY := h.app.EditorGroup.TabBar.GetRect().Y + 1
		gammaX := displayColumnOf(h.screenRow(tabsY), "gamma.txt")
		dropX := h.app.EditorGroup.TabBar.GetRect().X
		h.app.Root.HandleEvent(tcell.NewEventMouse(gammaX+2, tabsY, tcell.Button1, 0))
		h.app.Root.HandleEvent(tcell.NewEventMouse(dropX, tabsY, tcell.Button1, 0))
		h.app.Root.HandleEvent(tcell.NewEventMouse(dropX, tabsY, tcell.ButtonNone, 0))
		h.redraw()

		path0, _ := h.app.EditorGroup.TabInfo(0)
		if filepath.Base(path0) != "gamma.txt" {
			t.Fatalf("first tab = %q, want gamma.txt after drag", path0)
		}
	})

	t.Run("divider_resize", func(t *testing.T) {
		h := newTestHarness(t, 120, 30)
		defer h.stop()

		s := *h.app.Settings
		s.Appearance.PointerShapes = &disabled
		h.app.ApplySettings(s)

		h.app.ShowBottomPanel()
		h.redraw()

		cs := h.app.ContentSplit
		divY := cs.DividerScreenY()
		startH := cs.BottomH
		h.app.Root.HandleEvent(tcell.NewEventMouse(60, divY, tcell.Button1, tcell.ModNone))
		h.app.Root.HandleEvent(tcell.NewEventMouse(60, divY-4, tcell.Button1, tcell.ModNone))
		h.app.Root.HandleEvent(tcell.NewEventMouse(60, divY-4, tcell.ButtonNone, tcell.ModNone))
		h.redraw()

		if cs.BottomH != startH+4 {
			t.Errorf("panel height = %d, want %d after divider drag", cs.BottomH, startH+4)
		}
	})
}

func TestPointerShapeProblemsPanel(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()

	h.app.Problems.SetItems([]ui.ProblemItem{
		{File: "main.go", Line: 10, Col: 5, Severity: ui.DiagError, Message: "syntax error"},
	})
	h.app.ShowBottomPanel()
	h.app.BottomPanel.SetActivePanel("problems")
	h.redraw()

	panelRect := h.app.Problems.GetRect()
	if panelRect.H <= 0 {
		t.Fatal("problems panel not rendered")
	}

	// Diagnostic row inside the Problems panel receives PointerHand through Root
	if got := h.app.Root.PointerShapeAt(panelRect.X+5, panelRect.Y); got != term.PointerHand {
		t.Errorf("problems diagnostic row = %q, want %q", got, term.PointerHand)
	}

	// An overlay suppresses pointer feedback from the Problems panel underneath
	h.exec("command.palette")
	h.redraw()
	if got := h.app.Root.PointerShapeAt(panelRect.X+5, panelRect.Y); got != term.PointerDefault {
		t.Errorf("problems beneath overlay = %q, want %q", got, term.PointerDefault)
	}
}
