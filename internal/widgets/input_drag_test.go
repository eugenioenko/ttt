package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestInputDragSelection(t *testing.T) {
	for _, cfg := range []InputConfig{{}, {Bordered: true}, {Wrap: true}, {Mask: true}} {
		for _, reverse := range []bool{false, true} {
			input := NewInputWidget(cfg)
			input.SetText("a日bcdef")
			input.SetFocused(true)
			rect := Rect{X: 5, Y: 3, W: 30, H: 4}
			input.SetRect(rect)
			input.Render(newTestSurface(40, 10).Sub(rect))
			x, y := input.textRect.X, input.textRect.Y
			from, to := x+3, x+6
			if cfg.Mask {
				from, to = x+2, x+5
			}
			if reverse {
				from, to = to, from
			}
			if result := input.HandleEvent(mouseClick(from, y)); result != EventCaptured {
				t.Fatal("input did not capture press")
			}
			input.HandleEvent(mouseClick(to, y))
			input.HandleEvent(mouseRelease(to, y))
			if lo, hi := input.selRange(); lo != 2 || hi != 5 {
				t.Fatalf("config=%+v reverse=%v: selection=%d,%d", cfg, reverse, lo, hi)
			}
			if input.OwnsPointerCapture() {
				t.Fatal("release left pointer captured")
			}
			input.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "X", tcell.ModNone))
			if input.Text() != "a日Xef" {
				t.Fatalf("replacement = %q", input.Text())
			}
		}
	}
}

func TestInputDragWrapAndCancellation(t *testing.T) {
	input := NewInputWidget(InputConfig{Wrap: true, Prefix: "> ", MaxRows: 3})
	input.SetText("one two\nthree four")
	input.SetFocused(true)
	renderWidget(input, 0, 0, 20, 3)
	x, y := input.textRect.X, input.textRect.Y
	input.HandleEvent(mouseClick(x+4, y))
	input.HandleEvent(mouseClick(x+5, y+1))
	input.HandleEvent(mouseRelease(x+5, y+1))
	if lo, hi := input.selRange(); lo != 4 || hi != 13 {
		t.Fatalf("wrapped selection=%d,%d", lo, hi)
	}
	input.HandleEvent(mouseClick(x, y))
	input.SetFocused(false)
	if input.OwnsPointerCapture() {
		t.Fatal("blur kept drag active")
	}
	input.HandleEvent(mouseRelease(x+2, y))
	if input.HasSelection() {
		t.Fatal("release after cancellation selected text")
	}
}
