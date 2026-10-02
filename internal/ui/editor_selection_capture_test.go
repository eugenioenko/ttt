package ui

import (
	"strings"
	"testing"
	"time"

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

func TestEditorSelectionDragOntoScrollbarsKeepsSelecting(t *testing.T) {
	eg := NewEditorGroupWidget(nil, 4, false, "extended")
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = strings.Repeat("x", 300)
	}
	eg.Editor.Buf.Lines = lines

	root := NewRoot(eg)
	root.SetSize(80, 24)
	root.Render(makeGrid(80, 24))
	root.Render(makeGrid(80, 24))

	e := eg.Editor
	if !e.hscrollbar.Visible() || !e.scrollbar.Visible() {
		t.Fatal("test setup expects both scrollbars visible")
	}

	r := e.GetRect()
	startX := r.X + e.GutterWidth()
	root.HandleEvent(tcell.NewEventMouse(startX, r.Y, tcell.Button1, 0))
	for y := r.Y + 1; y <= e.hscrollbar.Y; y++ {
		root.HandleEvent(tcell.NewEventMouse(startX, y, tcell.Button1, 0))
		root.Render(makeGrid(80, 24))
	}
	root.HandleEvent(tcell.NewEventMouse(e.scrollbar.X, e.hscrollbar.Y, tcell.Button1, 0))
	root.HandleEvent(tcell.NewEventMouse(e.scrollbar.X, e.hscrollbar.Y, tcell.ButtonNone, 0))

	if e.hscrollbar.IsDragging() || e.scrollbar.IsDragging() {
		t.Fatal("selection drag latched onto a scrollbar")
	}
	if e.Viewport.TopLine == 0 {
		t.Fatal("dragging past the bottom did not scroll vertically")
	}
	if !e.Selection.Active {
		t.Fatal("selection was lost")
	}
}

func TestEditorSelectionDragHeldPastEdgeKeepsScrolling(t *testing.T) {
	eg := NewEditorGroupWidget(nil, 4, false, "extended")
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = strings.Repeat("x", 300)
	}
	eg.Editor.Buf.Lines = lines
	e := eg.Editor
	ticks := make(chan uint64, 8)
	e.PostDragAutoScrollTick = func(gen uint64) { ticks <- gen }

	root := NewRoot(eg)
	root.SetSize(80, 24)
	root.Render(makeGrid(80, 24))
	root.Render(makeGrid(80, 24))

	r := e.GetRect()
	x := r.X + e.GutterWidth()
	root.HandleEvent(tcell.NewEventMouse(x, r.Y+2, tcell.Button1, 0))
	root.HandleEvent(tcell.NewEventMouse(x, e.hscrollbar.Y, tcell.Button1, 0))

	top := e.Viewport.TopLine
	for i := 0; i < 3; i++ {
		e.HandleDragAutoScrollTick(<-ticks)
		if e.Viewport.TopLine != top+1 {
			t.Fatalf("tick %d: TopLine = %d, want %d", i, e.Viewport.TopLine, top+1)
		}
		top = e.Viewport.TopLine
	}

	root.HandleEvent(tcell.NewEventMouse(x, r.Y-1, tcell.Button1, 0))
	e.HandleDragAutoScrollTick(<-ticks)
	if e.Viewport.TopLine >= top {
		t.Fatalf("dragging above the editor did not scroll up: TopLine = %d", e.Viewport.TopLine)
	}

	root.HandleEvent(tcell.NewEventMouse(x, r.Y-1, tcell.ButtonNone, 0))
	select {
	case gen := <-ticks:
		if e.HandleDragAutoScrollTick(gen) {
			t.Fatal("auto-scroll continued after release")
		}
	case <-time.After(2 * editorDragAutoScrollDelay):
	}
}
