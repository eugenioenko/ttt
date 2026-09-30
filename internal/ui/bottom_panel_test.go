package ui

import "testing"

type bottomPanelDividerProbe struct {
	BaseWidget
	x int
}

func (p *bottomPanelDividerProbe) DividerScreenX() int { return p.x }

func TestBottomPanelDividerUnavailableWhenTooShort(t *testing.T) {
	bp := NewBottomPanelWidget(nil)
	bp.AddPanel("probe", "Probe", &bottomPanelDividerProbe{x: 17})

	bp.SetRect(Rect{W: 40, H: 3})
	if got := bp.DividerScreenX(); got != 17 {
		t.Fatalf("renderable panel divider = %d, want 17", got)
	}

	bp.SetRect(Rect{W: 40, H: 2})
	if got := bp.DividerScreenX(); got != -1 {
		t.Fatalf("short panel divider = %d, want -1", got)
	}
}
