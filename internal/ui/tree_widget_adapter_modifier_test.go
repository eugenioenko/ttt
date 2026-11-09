package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type modifierTreeProbe struct {
	BaseWidget
	children []widgets.Widget
	visits   int
}

func (p *modifierTreeProbe) WidgetChildren() []widgets.Widget {
	p.visits++
	return p.children
}

func TestAdapterModifiersDoNotWalkOnEvents(t *testing.T) {
	markdown := widgets.NewMarkdownWidget()
	probe := &modifierTreeProbe{children: []widgets.Widget{markdown}}
	adapter := NewWidgetAdapter(probe)
	mask := tcell.ModCtrl
	adapter.SetControlModifiers(&mask)
	if markdown.ControlModifiers != &mask {
		t.Fatal("control did not receive the shared setting")
	}
	probe.visits = 0
	mask = tcell.ModMeta
	for range 100 {
		adapter.SetControlModifiers(&mask)
		adapter.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "k", 0))
		adapter.HandleEvent(tcell.NewEventMouse(1, 1, tcell.ButtonNone, 0))
	}
	if probe.visits != 0 {
		t.Fatalf("events or unchanged settings walked the widget tree %d times", probe.visits)
	}
	if *markdown.ControlModifiers != tcell.ModMeta {
		t.Fatal("control did not observe the changed setting")
	}
	input := widgets.NewInputWidget(widgets.InputConfig{})
	probe.children = append(probe.children, input)
	adapter.RebuildFocus()
	if input.ControlModifiers != &mask {
		t.Fatal("rebuilt control did not inherit the setting")
	}
}
