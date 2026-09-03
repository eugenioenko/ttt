package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

// A text-selection drag that crosses the sidebar divider must not start a
// divider resize: the editor keeps pointer capture for the whole gesture.
func TestEditorSelectionDragDoesNotResizeSidebar(t *testing.T) {
	eg := NewEditorGroupWidget(nil, 4, false, "extended")
	eg.Editor.Buf.Lines = []string{strings.Repeat("abcde", 40)}

	cs := NewContentSplitWidget()
	cs.Top = eg

	split := NewSplitPanelWidget()
	split.Left = &mockWidget{renderChar: '.'}
	split.Right = cs
	split.DividerPos = 20
	split.ShowLeft = true

	resizeCalls := 0
	split.OnResize = func(int) { resizeCalls++ }

	root := NewRoot(split)
	root.SetSize(80, 24)
	root.Render(makeGrid(80, 24))

	drag := func(x, y int) {
		root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
	}
	drag(60, 18)
	for _, x := range []int{45, 30, 22, 21, 15, 8} {
		drag(x, 18-x/8)
	}
	root.HandleEvent(tcell.NewEventMouse(8, 12, tcell.ButtonNone, 0))

	if resizeCalls != 0 {
		t.Fatalf("divider resize fired %d times during a text selection drag", resizeCalls)
	}
	if split.dragging {
		t.Fatal("split panel latched into a divider drag")
	}
}

type capturingContentProbe struct {
	BaseWidget
	capturing bool
}

func (p *capturingContentProbe) HandleEvent(tcell.Event) EventResult { return EventIgnored }
func (p *capturingContentProbe) Render(Surface)                      {}
func (p *capturingContentProbe) OwnsPointerCapture() bool            { return p.capturing }

// A diff/commit-detail content tab that captures the pointer during a scrollbar
// drag or selection must be reported by the group, same as the editor pane.
func TestEditorGroupOwnsPointerCaptureFollowsContentTab(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	probe := &capturingContentProbe{}
	g.activeTab().Content = probe

	if g.OwnsPointerCapture() {
		t.Fatal("group owns capture while the content tab is idle")
	}
	probe.capturing = true
	if !g.OwnsPointerCapture() {
		t.Fatal("group dropped capture while the content tab held it")
	}
}
