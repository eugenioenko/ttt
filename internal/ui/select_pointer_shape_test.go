package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestWidgetAdapterSelectPopupTakesPrecedenceOverWidgetsUnderneath(t *testing.T) {
	// Select control at row 0, height 1.
	sw := widgets.NewSelectWidget(widgets.SelectConfig{
		Items: []widgets.SelectItem{
			{ID: "1", Label: "One"},
			{ID: "2", Label: "Two"},
		},
		Collapsible: true,
	})
	sw.SetRect(widgets.Rect{X: 0, Y: 0, W: 20, H: 1})

	// Underneath widget at row 1..5 that always reports term.PointerText.
	underneath := &shapeWidget{shape: term.PointerText}
	underneath.SetRect(widgets.Rect{X: 0, Y: 1, W: 20, H: 5})

	stack := widgets.NewVStackWidget(sw, underneath)
	adapter := NewWidgetAdapter(stack)
	adapter.SetRect(widgets.Rect{X: 0, Y: 0, W: 20, H: 6})

	render := func() {
		adapter.Render(NewRenderSurface(makeGrid(20, 10), widgets.Rect{X: 0, Y: 0, W: 20, H: 10}))
	}
	render()

	// 1. When popup is closed:
	// Select control returns PointerHand
	if got := adapter.PointerShapeAt(5, 0); got != term.PointerHand {
		t.Fatalf("closed select control = %q, want %q", got, term.PointerHand)
	}
	// Underneath widget at row 1 returns PointerText
	if got := adapter.PointerShapeAt(5, 1); got != term.PointerText {
		t.Fatalf("underneath widget before open = %q, want %q", got, term.PointerText)
	}

	// 2. Open the select popup.
	// Click row 0 to open
	adapter.HandleEvent(tcell.NewEventMouse(5, 0, tcell.Button1, 0))
	render()
	if !sw.HasPopup() {
		t.Fatal("expected select popup to be open after clicking control")
	}

	pr := sw.PopupRect()
	// Verify popup coordinates cover the underlying widget (Y=1..4)
	if pr.Y != 1 || pr.H < 4 {
		t.Fatalf("unexpected popup rect: %+v", pr)
	}

	// Option rows inside popup overlay the underlying widget:
	// Row Y=2 is "One": should be PointerHand, NOT PointerText
	if got := adapter.PointerShapeAt(5, 2); got != term.PointerHand {
		t.Fatalf("popup option over underlying widget = %q, want %q", got, term.PointerHand)
	}
	// Row Y=3 is "Two": should be PointerHand, NOT PointerText
	if got := adapter.PointerShapeAt(5, 3); got != term.PointerHand {
		t.Fatalf("popup option over underlying widget = %q, want %q", got, term.PointerHand)
	}

	// Neutral popup border at Y=1:
	// Must return PointerDefault, NOT PointerText from underlying widget!
	if got := adapter.PointerShapeAt(5, 1); got != term.PointerDefault {
		t.Fatalf("popup border over underlying widget = %q, want %q", got, term.PointerDefault)
	}
	// Neutral popup bottom border at Y=pr.Y+pr.H-1:
	if got := adapter.PointerShapeAt(5, pr.Y+pr.H-1); got != term.PointerDefault {
		t.Fatalf("popup bottom border over underlying widget = %q, want %q", got, term.PointerDefault)
	}

	// Row 5 is below the popup, still on the underlying widget:
	if got := adapter.PointerShapeAt(5, 5); got != term.PointerText {
		t.Fatalf("underneath widget below popup = %q, want %q", got, term.PointerText)
	}

	// 3. Close the popup
	sw.ClosePopup()
	render()
	if sw.HasPopup() {
		t.Fatal("expected popup to be closed")
	}

	// Once closed, row Y=2 and Y=1 must restore PointerText from the underlying widget
	if got := adapter.PointerShapeAt(5, 1); got != term.PointerText {
		t.Fatalf("underneath widget at Y=1 after close = %q, want %q", got, term.PointerText)
	}
	if got := adapter.PointerShapeAt(5, 2); got != term.PointerText {
		t.Fatalf("underneath widget at Y=2 after close = %q, want %q", got, term.PointerText)
	}
}
