package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

// A right-docked panel has no bottom edge: dragging up from the bottom of
// the content area opened it from the right, sized by the drag's x position
// (#1068).
func TestContentSplitBottomEdgeDoesNotOpenARightDockedPanel(t *testing.T) {
	split := NewContentSplitWidget()
	split.Position = SplitRight
	split.ShowBottom = false
	split.SetRect(Rect{X: 0, Y: 0, W: 100, H: 20})

	// A fresh click on the bottom edge of the content area.
	split.HandleEvent(tcell.NewEventMouse(60, 20, tcell.Button1, tcell.ModNone))

	if split.dragging {
		t.Error("the bottom-edge gesture started a drag on a right-docked panel")
	}
}

// The bottom-docked panel still opens from the bottom edge.
func TestContentSplitBottomEdgeOpensABottomDockedPanel(t *testing.T) {
	split := NewContentSplitWidget()
	split.Position = SplitBottom
	split.ShowBottom = false
	split.SetRect(Rect{X: 0, Y: 0, W: 100, H: 20})

	split.HandleEvent(tcell.NewEventMouse(60, 20, tcell.Button1, tcell.ModNone))

	if !split.dragging {
		t.Error("the bottom-edge gesture no longer opens a bottom-docked panel")
	}
}
