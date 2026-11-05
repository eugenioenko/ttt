package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestInputDragPreservesActionsAndUnicode(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		input := NewInputWidget()
		input.SetText("a日bcdef")
		actionCalls := 0
		input.Actions = []InputAction{{Label: "Action", OnClick: func() { actionCalls++ }}}
		render := func() { input.Render(NewRenderSurface(makeGrid(50, 10), Rect{X: 5, Y: 2, W: 40, H: 5}), 1, 1, 30) }
		render()
		start, end := input.renderX+3, input.renderX+6
		if reverse {
			start, end = end, start
		}
		mouse := func(x int, button tcell.ButtonMask) {
			input.HandleMouse(tcell.NewEventMouse(x, input.renderY, button, 0))
			render()
		}
		mouse(start, tcell.Button1)
		mouse(end, tcell.Button1)
		mouse(end, tcell.ButtonNone)
		if lo, hi := input.selRange(); lo != 2 || hi != 5 {
			t.Fatalf("reverse=%v: selection=%d,%d", reverse, lo, hi)
		}
		mouse(input.renderX, tcell.Button1)
		hit := input.ActionHits[0]
		mouse(hit.X, tcell.Button1)
		mouse(hit.X, tcell.ButtonNone)
		if actionCalls != 0 {
			t.Fatal("drag fired an input action")
		}
		mouse(hit.X, tcell.Button1)
		mouse(hit.X, tcell.ButtonNone)
		if actionCalls != 1 {
			t.Fatal("ordinary action click stopped working")
		}
		mouse(input.renderX, tcell.Button1)
		input.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModNone))
		if input.OwnsPointerCapture() {
			t.Fatal("keyboard interaction kept drag active")
		}
		mouse(input.renderX, tcell.Button1)
		input.PasteText("X")
		if input.OwnsPointerCapture() {
			t.Fatal("paste kept drag active")
		}
	}
}
