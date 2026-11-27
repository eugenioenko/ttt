package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestCheckboxPointerShapeInsideAndOutsideHitTests(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Enable Option"})
	cb.SetRect(Rect{X: 10, Y: 5, W: 25, H: 1})

	tests := []struct {
		name string
		x    int
		y    int
		want string
	}{
		{"top-left edge inside", 10, 5, term.PointerHand},
		{"bracket prefix inside", 11, 5, term.PointerHand},
		{"mark cell inside", 12, 5, term.PointerHand},
		{"label text inside", 15, 5, term.PointerHand},
		{"trailing row space inside", 25, 5, term.PointerHand},
		{"bottom-right edge inside", 34, 5, term.PointerHand},
		{"left outside", 9, 5, ""},
		{"right outside", 35, 5, ""},
		{"top outside", 10, 4, ""},
		{"bottom outside", 10, 6, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cb.PointerShapeAt(tt.x, tt.y); got != tt.want {
				t.Errorf("cb.PointerShapeAt(%d, %d) = %q, want %q", tt.x, tt.y, got, tt.want)
			}
			if got := PointerShapeAt(cb, tt.x, tt.y); got != tt.want {
				t.Errorf("widgets.PointerShapeAt(cb, %d, %d) = %q, want %q", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestCheckboxPointerShapeCheckedAndUncheckedStates(t *testing.T) {
	for _, checked := range []bool{false, true} {
		name := "unchecked"
		if checked {
			name = "checked"
		}
		t.Run(name, func(t *testing.T) {
			cb := NewCheckboxWidget(CheckboxConfig{Label: "Toggle", Checked: checked})
			cb.SetRect(Rect{X: 2, Y: 1, W: 20, H: 1})

			if got := cb.PointerShapeAt(5, 1); got != term.PointerHand {
				t.Errorf("inside %s checkbox = %q, want %q", name, got, term.PointerHand)
			}
			if got := cb.PointerShapeAt(0, 1); got != "" {
				t.Errorf("outside %s checkbox = %q, want empty", name, got)
			}
		})
	}
}

func TestCheckboxPointerShapeFocusedAndUnfocusedStates(t *testing.T) {
	for _, focused := range []bool{false, true} {
		name := "unfocused"
		if focused {
			name = "focused"
		}
		t.Run(name, func(t *testing.T) {
			cb := NewCheckboxWidget(CheckboxConfig{Label: "Focus Test"})
			cb.SetFocused(focused)
			cb.SetRect(Rect{X: 4, Y: 2, W: 18, H: 1})

			if got := cb.PointerShapeAt(6, 2); got != term.PointerHand {
				t.Errorf("inside %s checkbox = %q, want %q", name, got, term.PointerHand)
			}
			if got := cb.PointerShapeAt(3, 2); got != "" {
				t.Errorf("outside %s checkbox = %q, want empty", name, got)
			}
		})
	}
}

func TestCheckboxPointerShapeLeavesStateAndCallbacksUnchanged(t *testing.T) {
	callbacks := 0
	cb := NewCheckboxWidget(CheckboxConfig{
		Label:   "Callback Test",
		Checked: false,
		OnChange: func(bool) {
			callbacks++
		},
	})
	cb.SetRect(Rect{X: 0, Y: 0, W: 20, H: 1})

	for x := -5; x < 30; x++ {
		for y := -2; y < 5; y++ {
			_ = cb.PointerShapeAt(x, y)
		}
	}

	if cb.Config.Checked != false {
		t.Fatalf("cb.Config.Checked was modified: got %v, want false", cb.Config.Checked)
	}
	if callbacks != 0 {
		t.Fatalf("cb.Config.OnChange was called %d times, want 0", callbacks)
	}
}

func TestCheckboxPointerShapeEmptyRectHasNoOpinion(t *testing.T) {
	cb := NewCheckboxWidget(CheckboxConfig{Label: "Unrendered"})
	// default empty Rect{0, 0, 0, 0}
	if got := cb.PointerShapeAt(0, 0); got != "" {
		t.Errorf("unrendered checkbox shape = %q, want empty", got)
	}
}
