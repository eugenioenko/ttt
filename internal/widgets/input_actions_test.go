package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

func TestInputDragPreservesActionsAndUnicode(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		input := NewInputWidget(InputConfig{})
		input.SetFocused(true)
		input.SetText("a日bcdef")
		actionCalls := 0
		input.Actions = []InputAction{{Label: "Action", OnClick: func() { actionCalls++ }}}
		render := func() {
			input.SetRect(Rect{X: 6, Y: 3, W: 30, H: 1})
			input.Render(newTestSurface(50, 10).Sub(input.GetRect()))
		}
		render()
		start, end := input.textRect.X+3, input.textRect.X+6
		if reverse {
			start, end = end, start
		}
		mouse := func(x int, button tcell.ButtonMask) {
			input.HandleEvent(tcell.NewEventMouse(x, input.textRect.Y, button, 0))
			render()
		}
		mouse(start, tcell.Button1)
		mouse(end, tcell.Button1)
		mouse(end, tcell.ButtonNone)
		if lo, hi := input.selRange(); lo != 2 || hi != 5 {
			t.Fatalf("reverse=%v: selection=%d,%d", reverse, lo, hi)
		}
		mouse(input.textRect.X, tcell.Button1)
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
		mouse(input.textRect.X, tcell.Button1)
		input.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModNone))
		if input.OwnsPointerCapture() {
			t.Fatal("keyboard interaction kept drag active")
		}
		mouse(input.textRect.X, tcell.Button1)
		input.PasteText("X")
		if input.OwnsPointerCapture() {
			t.Fatal("paste kept drag active")
		}
	}
}

func TestInputActionsStayWithinRenderedContent(t *testing.T) {
	for _, cfg := range []InputConfig{{}, {Bordered: true}, {Wrap: true}, {Mask: true}} {
		for _, width := range []int{1, 4, 8, 16} {
			input := NewInputWidget(cfg)
			input.SetFocused(true)
			input.SetText("日abcdef")
			calls := 0
			input.Actions = []InputAction{{Label: "界A", Active: true, OnClick: func() { calls++ }}, {Label: "B"}}
			rect := Rect{X: 5, Y: 3, W: width, H: 4}
			input.SetRect(rect)
			surface := newTestSurface(30, 10)
			input.Render(surface.Sub(rect))
			for x := rect.X + rect.W; x < 30; x++ {
				if surface.cells[rect.Y][x].Ch != 0 {
					t.Fatalf("width=%d config=%+v: action escaped into column %d", width, cfg, x)
				}
			}
			for i, hit := range input.ActionHits {
				if hit.W == 0 {
					continue
				}
				if hit.X < rect.X || hit.X+hit.W > rect.X+rect.W || hit.Y < rect.Y || hit.Y >= rect.Y+rect.H {
					t.Fatalf("width=%d config=%+v: action escaped bounds: %+v", width, cfg, hit)
				}
				if input.PointerShapeAt(hit.X, hit.Y) != term.PointerHand {
					t.Fatal("action did not advertise a clickable target")
				}
				if input.HandleEvent(mouseClick(hit.X, hit.Y)) != EventCaptured || input.drag.Active() {
					t.Fatal("action click started text selection")
				}
				input.HandleEvent(mouseRelease(hit.X, hit.Y))
				if i == 0 && calls != 1 {
					t.Fatal("visible action did not fire")
				}
			}
			if input.Text() != "日abcdef" {
				t.Fatal("action click changed input text")
			}
		}
	}
}

func TestInputActionFiresOncePerPress(t *testing.T) {
	input := NewInputWidget(InputConfig{})
	input.SetFocused(true)
	calls := 0
	input.Actions = []InputAction{{Label: "Action", OnClick: func() { calls++ }}}
	renderWidget(input, 0, 0, 30, 1)
	hit := input.ActionHits[0]
	for _, x := range []int{hit.X, hit.X + 1, hit.X + 2} {
		input.HandleEvent(mouseClick(x, hit.Y))
	}
	if calls != 1 || input.HasSelection() {
		t.Fatalf("held action press: calls=%d selection=%v", calls, input.HasSelection())
	}
	input.HandleEvent(mouseClick(100, 100))
	input.HandleEvent(mouseRelease(100, 100))
	if input.OwnsPointerCapture() {
		t.Fatal("release outside input kept action captured")
	}
	input.HandleEvent(mouseClick(hit.X, hit.Y))
	input.HandleEvent(mouseRelease(hit.X, hit.Y))
	if calls != 2 {
		t.Fatalf("next action press: calls=%d, want 2", calls)
	}
}

func TestInputIgnoresRemovedActionBeforeRedraw(t *testing.T) {
	for _, remaining := range []int{0, 1} {
		input := NewInputWidget(InputConfig{})
		calls := 0
		input.Actions = []InputAction{{Label: "First", OnClick: func() { calls++ }}, {Label: "Last", OnClick: func() { calls++ }}}
		renderWidget(input, 0, 0, 30, 1)
		hit := input.ActionHits[1]
		input.Actions = input.Actions[:remaining]
		if result := input.HandleEvent(mouseClick(hit.X, hit.Y)); result != EventConsumed {
			t.Fatalf("remaining=%d: removed action click was not consumed: %v", remaining, result)
		}
		if calls != 0 || input.OwnsPointerCapture() {
			t.Fatal("removed action fired a callback or captured the press")
		}
	}
}
