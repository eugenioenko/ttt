package ui

import "testing"

func TestContentSplitResizePanelStepsEffectiveSize(t *testing.T) {
	split := NewContentSplitWidget()
	split.ShowBottom = true
	split.Position = SplitRight
	split.SetRect(Rect{W: 50, H: 20})

	split.ResizePanel(-1)
	if split.RightW != 28 {
		t.Fatalf("shorter from oversized RightW = %d, want 28", split.RightW)
	}

	split.RightW = 60
	split.ResizePanel(1)
	if split.RightW != 29 {
		t.Fatalf("taller from oversized RightW = %d, want 29", split.RightW)
	}

	split.Position = SplitBottom
	split.MinTopH = 5
	split.BottomH = 40
	split.ResizePanel(-1)
	if split.BottomH != 13 {
		t.Fatalf("shorter from oversized BottomH = %d, want 13", split.BottomH)
	}
}
