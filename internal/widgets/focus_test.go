package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func tabEvent() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone)
}

func shiftTabEvent() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyBacktab, "", tcell.ModShift)
}

func TestFocusManagerCollect(t *testing.T) {
	b1 := NewButtonWidget(ButtonConfig{Label: "&Run"})
	b2 := NewButtonWidget(ButtonConfig{Label: "&Stop"})
	tree := NewTreeWidget(TreeConfig{})
	label := NewTitleWidget(TitleConfig{Title: "Header"})

	vs := NewVStackWidget(tree, label, NewHStackWidget(b1, b2))

	fm := NewFocusManager()
	fm.SetActive(true)
	fm.Collect(vs)

	if len(fm.items) != 3 {
		t.Fatalf("expected 3 focusable items, got %d", len(fm.items))
	}
	if fm.items[0] != Widget(tree) {
		t.Error("first focusable should be tree")
	}
	if fm.items[1] != Widget(b1) {
		t.Error("second focusable should be b1")
	}
	if fm.items[2] != Widget(b2) {
		t.Error("third focusable should be b2")
	}
	if !tree.focused {
		t.Error("tree should be focused initially")
	}
}

func TestFocusManagerTabCycle(t *testing.T) {
	b1 := NewButtonWidget(ButtonConfig{Label: "&Run"})
	b2 := NewButtonWidget(ButtonConfig{Label: "&Stop"})
	tree := NewTreeWidget(TreeConfig{})

	vs := NewVStackWidget(tree, NewHStackWidget(b1, b2))

	fm := NewFocusManager()
	fm.SetActive(true)
	fm.Collect(vs)

	if fm.Focused() != Widget(tree) {
		t.Fatal("initial focus should be tree")
	}

	fm.HandleEvent(tabEvent())
	if fm.Focused() != Widget(b1) {
		t.Error("after tab, focus should be b1")
	}
	if tree.focused {
		t.Error("tree should not be focused after tab")
	}
	if !b1.focused {
		t.Error("b1 should be focused")
	}

	fm.HandleEvent(tabEvent())
	if fm.Focused() != Widget(b2) {
		t.Error("after second tab, focus should be b2")
	}

	fm.HandleEvent(tabEvent())
	if fm.Focused() != Widget(tree) {
		t.Error("after third tab, focus should wrap to tree")
	}
}

func TestFocusManagerSkipsFocusableChildWithNoRenderedGeometry(t *testing.T) {
	first := NewButtonWidget(ButtonConfig{Label: "First"})
	second := NewButtonWidget(ButtonConfig{Label: "Second"})
	clipped := NewButtonWidget(ButtonConfig{Label: "Clipped"})
	stack := NewVStackWidget(first, second, clipped)

	fm := NewFocusManager()
	fm.SetActive(true)
	fm.Collect(stack)
	renderWidget(stack, 0, 0, 20, 3)
	renderWidget(stack, 0, 0, 20, 2)
	if r := clipped.GetRect(); r.W <= 0 || r.H <= 0 || r.Y < stack.GetRect().H {
		t.Fatalf("clipped child rect = %+v, want stale positive geometry outside shrunken stack %+v", r, stack.GetRect())
	}

	fm.HandleEvent(tabEvent())
	if fm.Focused() != Widget(second) {
		t.Fatalf("first Tab focus = %T, want second visible button", fm.Focused())
	}
	fm.HandleEvent(tabEvent())
	if fm.Focused() != Widget(first) {
		t.Fatalf("second Tab focus = %T, want wrap past clipped button", fm.Focused())
	}
}

func TestFocusManagerKeepsOffscreenScrollContentEligible(t *testing.T) {
	first := NewButtonWidget(ButtonConfig{Label: "First"})
	second := NewButtonWidget(ButtonConfig{Label: "Second"})
	content := NewVStackWidget(first, second)
	scroll := NewScrollViewWidget(content)

	fm := NewFocusManager()
	fm.SetActive(true)
	fm.Collect(scroll)
	renderWidget(scroll, 0, 0, 20, 1)
	if VisibleRect(scroll, second) != (Rect{}) {
		t.Fatalf("second button should begin outside the scroll viewport, got %+v", VisibleRect(scroll, second))
	}

	fm.HandleEvent(tabEvent())
	fm.HandleEvent(tabEvent())
	if fm.Focused() != Widget(second) {
		t.Fatalf("focus = %T, want offscreen button retained for scroll-into-view", fm.Focused())
	}
}

func TestFocusManagerShiftTab(t *testing.T) {
	b1 := NewButtonWidget(ButtonConfig{Label: "&Run"})
	b2 := NewButtonWidget(ButtonConfig{Label: "&Stop"})

	hs := NewHStackWidget(b1, b2)

	fm := NewFocusManager()
	fm.Collect(hs)

	if fm.Focused() != Widget(b1) {
		t.Fatal("initial focus should be b1")
	}

	fm.HandleEvent(shiftTabEvent())
	if fm.Focused() != Widget(b2) {
		t.Error("shift+tab from first should wrap to last")
	}

	fm.HandleEvent(shiftTabEvent())
	if fm.Focused() != Widget(b1) {
		t.Error("shift+tab from last should wrap to first")
	}
}

func TestFocusManagerKeyRouting(t *testing.T) {
	triggered := false
	b1 := NewButtonWidget(ButtonConfig{
		Label:   "&Run",
		Command: "docker.run",
		OnCommand: func(cmd string) {
			triggered = true
		},
	})
	b2 := NewButtonWidget(ButtonConfig{Label: "&Stop"})

	hs := NewHStackWidget(b1, b2)
	fm := NewFocusManager()
	fm.SetActive(true)
	fm.Collect(hs)

	enterEv := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	fm.HandleEvent(enterEv)

	if !triggered {
		t.Error("enter on focused button should trigger its command")
	}
}

func TestFocusManagerEmpty(t *testing.T) {
	vs := NewVStackWidget()
	fm := NewFocusManager()
	fm.Collect(vs)

	if fm.Focused() != nil {
		t.Error("empty manager should have no focused widget")
	}

	fm.HandleEvent(tabEvent())
	if fm.Focused() != nil {
		t.Error("tab on empty should not crash")
	}
}

func TestFocusManagerNestedBox(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "&Go"})
	box := NewBoxWidget(BoxModel{})
	box.Child = btn

	fm := NewFocusManager()
	fm.Collect(box)

	if len(fm.items) != 1 {
		t.Fatalf("expected 1 focusable in box, got %d", len(fm.items))
	}
	if fm.Focused() != Widget(btn) {
		t.Error("button inside box should be focusable")
	}
}

func TestActiveBorderOnFocus(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{})
	box := NewBoxWidget(BoxModel{
		BorderTop: true, BorderBottom: true,
		BorderLeft: true, BorderRight: true,
	})
	box.Child = tree

	fm := NewFocusManager()
	fm.SetActive(true)
	fm.Collect(box)

	if !tree.IsFocused() {
		t.Fatal("tree should start focused")
	}
	if !hasFocusedChild(box.Child) {
		t.Error("hasFocusedChild should detect focused tree inside box")
	}
}

func TestNonFocusableSkipped(t *testing.T) {
	title := NewTitleWidget(TitleConfig{Title: "Header"})
	btn := NewButtonWidget(ButtonConfig{Label: "&Go"})

	vs := NewVStackWidget(title, btn)
	fm := NewFocusManager()
	fm.Collect(vs)

	if len(fm.items) != 1 {
		t.Fatalf("expected 1 focusable (title is not focusable), got %d", len(fm.items))
	}
	if fm.Focused() != Widget(btn) {
		t.Error("only btn should be focusable")
	}
}
