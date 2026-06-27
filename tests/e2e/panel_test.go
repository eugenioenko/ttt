package e2e

import "testing"

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

	h.click(panelX+49, panelY)
	if h.app.BottomPanel.ActivePanel != "test-b" {
		t.Errorf("expected active panel 'test-b' after click, got %q", h.app.BottomPanel.ActivePanel)
	}

	h.click(panelX+42, panelY)
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
		t.Fatalf("expected 4 panels (terminal+problems+references+comments), got %d", h.app.BottomPanel.PanelCount())
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
