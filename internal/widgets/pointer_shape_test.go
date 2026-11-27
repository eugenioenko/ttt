package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestPointerShapeAtPrefersLaterChildWhereChildrenOverlap(t *testing.T) {
	button := NewButtonWidget(ButtonConfig{Label: "Run"})
	button.SetRect(Rect{X: 0, Y: 0, W: 10, H: 1})
	input := NewInputWidget(InputConfig{})
	input.SetRect(Rect{X: 5, Y: 0, W: 10, H: 1})
	stack := NewVStackWidget(button, input)

	for _, tt := range []struct {
		x    int
		want string
	}{
		{2, term.PointerHand},
		{7, term.PointerText},
		{20, ""},
	} {
		if got := PointerShapeAt(stack, tt.x, 0); got != tt.want {
			t.Errorf("PointerShapeAt(%d, 0) = %q, want %q", tt.x, got, tt.want)
		}
	}
}

func TestPointerShapeAtFollowsCaptureOutsideTheOwner(t *testing.T) {
	items := make([]*TreeNode, 10)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('a' + i)), Label: "item"}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	tree.SetRect(Rect{X: 0, Y: 0, W: 5, H: 3})
	tree.Render(newVirtualSurface(5, 3))
	button := NewButtonWidget(ButtonConfig{Label: "Run"})
	button.SetRect(Rect{X: 0, Y: 5, W: 10, H: 1})
	stack := NewVStackWidget(tree, button)

	if got := PointerShapeAt(stack, 2, 5); got != term.PointerHand {
		t.Fatalf("over button before the drag = %q, want %q", got, term.PointerHand)
	}
	tree.HandleEvent(tcell.NewEventMouse(4, 0, tcell.Button1, 0))
	if got := PointerShapeAt(stack, 2, 5); got != term.PointerDefault {
		t.Fatalf("over button during a scrollbar drag = %q, want %q", got, term.PointerDefault)
	}
}

func TestDisabledButtonHasNoPointerShape(t *testing.T) {
	button := NewButtonWidget(ButtonConfig{Label: "Run"})
	button.SetRect(Rect{W: 10, H: 1})
	button.Disabled = true
	if got := button.PointerShapeAt(1, 0); got != "" {
		t.Fatalf("disabled button shape = %q, want none", got)
	}
}

func TestTreePointerShapeSkipsHeadingsScrollbarAndEmptyRows(t *testing.T) {
	items := []*TreeNode{{ID: "h", Label: "Heading", Heading: true}}
	for i := range 9 {
		items = append(items, &TreeNode{ID: string(rune('a' + i)), Label: "item"})
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	tree.SetRect(Rect{W: 8, H: 4})
	tree.Render(newVirtualSurface(8, 4))

	if got := tree.PointerShapeAt(1, 0); got != "" {
		t.Errorf("heading shape = %q, want none", got)
	}
	if got := tree.PointerShapeAt(1, 1); got != term.PointerHand {
		t.Errorf("row shape = %q, want %q", got, term.PointerHand)
	}
	if got := tree.PointerShapeAt(7, 1); got != "" {
		t.Errorf("scrollbar shape = %q, want none", got)
	}

	short := NewTreeWidget(TreeConfig{Items: items[1:2]})
	short.SetRect(Rect{W: 8, H: 4})
	short.Render(newVirtualSurface(8, 4))
	if got := short.PointerShapeAt(1, 3); got != "" {
		t.Errorf("empty row shape = %q, want none", got)
	}
}

func TestTabsPointerShape(t *testing.T) {
	tabs := NewTabsWidget(TabsConfig{
		Items:       []TabItem{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}},
		Reorderable: true,
		OnReorder:   func(from, to int) {},
	})
	tabs.SetRect(Rect{X: 2, Y: 1, W: 40, H: 1})
	tabs.Render(newVirtualSurface(40, 1))

	tabX := tabs.GetRect().X + tabs.tabSpans[0][0]
	if got := tabs.PointerShapeAt(tabX, 1); got != term.PointerHand {
		t.Fatalf("over a tab = %q, want %q", got, term.PointerHand)
	}
	if got := tabs.PointerShapeAt(41, 1); got != "" {
		t.Fatalf("past the tabs = %q, want none", got)
	}

	tabs.HandleEvent(tcell.NewEventMouse(tabX, 1, tcell.Button1, 0))
	if got := tabs.PointerShapeAt(tabX, 1); got != term.PointerHand {
		t.Fatalf("pressed but not yet dragging = %q, want %q", got, term.PointerHand)
	}
	tabs.HandleEvent(tcell.NewEventMouse(tabX+5, 1, tcell.Button1, 0))
	if got := tabs.PointerShapeAt(tabX+5, 9); got != term.PointerGrabbing {
		t.Fatalf("dragging outside the bar = %q, want %q", got, term.PointerGrabbing)
	}
	tabs.HandleEvent(tcell.NewEventMouse(tabX+5, 1, tcell.ButtonNone, 0))
	if got := tabs.PointerShapeAt(41, 1); got != "" {
		t.Fatalf("after release = %q, want none", got)
	}
}
