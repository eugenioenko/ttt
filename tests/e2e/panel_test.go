package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestBottomPanelOverflowMenu(t *testing.T) {
	for _, tc := range []struct {
		name      string
		width     int
		dockRight bool
	}{
		{name: "bottom dock", width: 56},
		{name: "right dock", width: 80, dockRight: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t, tc.width, 30)
			defer h.stop()

			target := newEmptyWidget()
			h.app.BottomPanel.AddPanel("overflow-target", "Hidden Target", target)
			if tc.dockRight {
				h.exec("panel.dockRight")
			}
			h.app.ContentSplit.ShowBottom = true
			h.app.BottomPanel.SetActivePanel("terminal")
			h.redraw()

			hiddenIDs, _ := h.app.BottomPanel.HiddenTabs()
			hidden := false
			for _, id := range hiddenIDs {
				if id == "overflow-target" {
					hidden = true
					break
				}
			}
			if !hidden {
				t.Fatalf("test target is not hidden at width %d: %v", tc.width, hiddenIDs)
			}

			tabsRect := h.app.BottomPanel.Tabs.GetRect()
			cells, screenWidth, _ := h.screen.GetContents()
			overflowX := -1
			for x := tabsRect.X; x < tabsRect.X+tabsRect.W; x++ {
				if cells[tabsRect.Y*screenWidth+x].Str == "»" {
					overflowX = x
					break
				}
			}
			if overflowX < 0 {
				t.Fatalf("no bottom-panel overflow chevron in %s layout:\n%s", tc.name, h.screenText())
			}
			h.click(overflowX, tabsRect.Y)
			menu, ok := h.app.Root.Focused.(*ui.ContextMenuWidget)
			if !ok {
				t.Fatalf("focused widget = %T, want context menu", h.app.Root.Focused)
			}

			targetIndex := -1
			for i, item := range menu.Items {
				if item.Label == "Hidden Target" {
					targetIndex = i
					break
				}
			}
			if targetIndex < 0 {
				t.Fatalf("overflow menu missing hidden target: %+v", menu.Items)
			}
			for menu.Selected != targetIndex {
				h.pressKey(tcell.KeyDown, tcell.ModNone)
			}
			h.pressKey(tcell.KeyEnter, tcell.ModNone)

			if got := h.app.BottomPanel.ActivePanel; got != "overflow-target" {
				t.Errorf("active panel = %q, want overflow-target", got)
			}
			if h.app.Root.Focused != target {
				t.Errorf("focused widget = %T, want selected panel content", h.app.Root.Focused)
			}
		})
	}
}

func TestBottomPanelTabClick(t *testing.T) {
	h := newTestHarness(t, 120, 24)
	defer h.stop()

	h.app.BottomPanel.AddPanel("test-a", "Alpha", newEmptyWidget())
	h.app.BottomPanel.AddPanel("test-b", "Beta", newEmptyWidget())
	h.app.ContentSplit.ShowBottom = true
	h.app.ContentSplit.BottomH = 10
	h.redraw()

	h.app.BottomPanel.SetActivePanel("test-a")
	if h.app.BottomPanel.ActivePanel != "test-a" {
		t.Fatalf("expected active panel 'test-a', got %q", h.app.BottomPanel.ActivePanel)
	}

	panelY := h.app.BottomPanel.GetRect().Y
	panelX := h.app.BottomPanel.GetRect().X

	h.click(panelX+50, panelY)
	if h.app.BottomPanel.ActivePanel != "test-b" {
		t.Errorf("expected active panel 'test-b' after click, got %q", h.app.BottomPanel.ActivePanel)
	}

	h.click(panelX+43, panelY)
	if h.app.BottomPanel.ActivePanel != "test-a" {
		t.Errorf("expected active panel 'test-a' after click, got %q", h.app.BottomPanel.ActivePanel)
	}
}

func TestTabbedPanelRemovePanel(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.app.BottomPanel.AddPanel("p1", "One", newEmptyWidget())
	h.app.BottomPanel.AddPanel("p2", "Two", newEmptyWidget())
	h.app.BottomPanel.AddPanel("p3", "Three", newEmptyWidget())
	h.app.BottomPanel.SetActivePanel("p2")

	if h.app.BottomPanel.PanelCount() != 7 {
		t.Fatalf("expected 7 panels, got %d", h.app.BottomPanel.PanelCount())
	}

	h.app.BottomPanel.RemovePanel("p2")
	if h.app.BottomPanel.PanelCount() != 6 {
		t.Fatalf("expected 6 panels, got %d", h.app.BottomPanel.PanelCount())
	}
	if h.app.BottomPanel.ActivePanel == "p2" {
		t.Error("active panel should have changed after removing it")
	}

	h.app.BottomPanel.RemovePanel("p1")
	h.app.BottomPanel.RemovePanel("p3")
	if h.app.BottomPanel.PanelCount() != 4 {
		t.Fatalf("expected 4 panels (terminal+problems+references+output), got %d", h.app.BottomPanel.PanelCount())
	}
}

func TestTogglePanel(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	if h.app.ContentSplit.ShowBottom {
		t.Error("bottom panel should start hidden")
	}

	h.exec("panel.toggle")
	if !h.app.ContentSplit.ShowBottom {
		t.Error("bottom panel should be visible after toggle")
	}

	h.exec("panel.toggle")
	if h.app.ContentSplit.ShowBottom {
		t.Error("bottom panel should be hidden after second toggle")
	}
}

// Guards against sizing the PTY from full editor+panel height (issue #465).
func TestSpawnTerminalSizedToBottomPanel(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()

	h.app.Settings.Terminal.Shell = "/bin/cat"
	h.exec("terminal.toggle")
	if len(h.app.Terminals) != 1 {
		t.Fatalf("expected 1 terminal after toggle, got %d", len(h.app.Terminals))
	}
	term := h.app.Terminals[0].Term
	defer term.Close()

	fullH := h.app.ContentSplit.GetRect().H
	if h.app.ContentSplit.BottomH >= fullH-3 {
		t.Fatalf("test setup invalid: bottom panel height %d not smaller than full content split height %d", h.app.ContentSplit.BottomH, fullH)
	}

	wantRows := h.app.ContentSplit.BottomH - 2
	_, gotRows := term.Size()
	if gotRows != wantRows {
		t.Errorf("terminal rows = %d, want %d (bottom panel height %d, full content split height %d)", gotRows, wantRows, h.app.ContentSplit.BottomH, fullH)
	}
}

func TestPanelDockedRightKeepsEditorAndCorner(t *testing.T) {
	h := newTestHarness(t, 80, 14)
	defer h.stop()

	h.app.Settings.Terminal.Shell = "/bin/cat"
	h.exec("terminal.toggle")
	h.exec("panel.dockRight")
	h.redraw()
	for _, tt := range h.app.Terminals {
		defer tt.Term.Close()
	}

	h.assertContains("untitled")
	if row := []rune(h.screenRow(1)); row[len(row)-1] != '╮' {
		t.Errorf("right edge of row 1 = %q, want '╮'\n%s", row[len(row)-1], h.screenText())
	}
}
