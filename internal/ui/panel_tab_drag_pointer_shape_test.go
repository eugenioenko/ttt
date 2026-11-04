package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestBottomPanelTabDragPointerGrabbingAcrossPanels(t *testing.T) {
	bp := NewBottomPanelWidget(nil)
	bp.Tabs.Config.Reorderable = true
	var reordered bool
	bp.Tabs.Config.OnReorder = func(from, to int) {
		bp.MovePanel(from, to)
		reordered = true
	}
	bp.AddPanel("term", "Terminal", &mockWidget{})
	bp.AddPanel("probs", "Diagnostics", &mockWidget{})
	bp.AddPanel("output", "Output", &mockWidget{})

	top := &shapeWidget{shape: term.PointerText}
	top.SetRect(Rect{X: 0, Y: 0, W: 80, H: 20})

	cs := NewContentSplitWidget()
	cs.Top = top
	cs.Bottom = bp
	cs.ShowBottom = true
	cs.BottomH = 10
	cs.SetRect(Rect{X: 0, Y: 0, W: 80, H: 30})

	root := NewRoot(cs)
	root.SetSize(80, 30)

	// Render to layout widgets and calculate tab spans
	grid := makeGrid(80, 30)
	root.Render(grid)

	divY := cs.DividerScreenY()
	tabsY := bp.GetRect().Y

	// Tab 1 ("Terminal") starts around x=1
	tabX := 3

	// Initial hover over tab should be PointerHand
	if got := root.PointerShapeAt(tabX, tabsY); got != term.PointerHand {
		t.Fatalf("hover over tab = %q, want %q", got, term.PointerHand)
	}

	// 1. Mouse down on tab starts drag gesture
	root.HandleEvent(tcell.NewEventMouse(tabX, tabsY, tcell.Button1, 0))
	if !bp.Tabs.PointerGestureActive() {
		t.Fatal("tab press did not start a pointer gesture")
	}

	// 2. Move horizontally beyond drag threshold (2 columns)
	root.HandleEvent(tcell.NewEventMouse(tabX+5, tabsY, tcell.Button1, 0))
	root.Render(grid)

	if !bp.Tabs.OwnsPointerCapture() {
		t.Fatal("expected bp.Tabs to own pointer capture after crossing drag threshold")
	}

	// 3. Pointer shape must remain PointerGrabbing over tab bar
	if got := root.PointerShapeAt(tabX+5, tabsY); got != term.PointerGrabbing {
		t.Errorf("dragging over tab bar = %q, want %q", got, term.PointerGrabbing)
	}

	// 4. Pointer shape must remain PointerGrabbing over editor (cs.Top)
	editorY := 10
	if got := root.PointerShapeAt(tabX+5, editorY); got != term.PointerGrabbing {
		t.Errorf("dragging over editor (top) = %q, want %q", got, term.PointerGrabbing)
	}

	// 5. Pointer shape must remain PointerGrabbing over divider
	if got := root.PointerShapeAt(tabX+5, divY); got != term.PointerGrabbing {
		t.Errorf("dragging over divider = %q, want %q", got, term.PointerGrabbing)
	}

	// 6. Pointer shape must remain PointerGrabbing over bottom panel content
	contentY := tabsY + 3
	if got := root.PointerShapeAt(tabX+5, contentY); got != term.PointerGrabbing {
		t.Errorf("dragging over panel content = %q, want %q", got, term.PointerGrabbing)
	}

	// 7. Mouse release ends drag and restores normal pointers
	root.HandleEvent(tcell.NewEventMouse(tabX+5, tabsY, tcell.ButtonNone, 0))
	root.Render(grid)

	if bp.Tabs.PointerGestureActive() {
		t.Fatal("gesture should be inactive after mouse release")
	}

	// Editor should return to PointerText
	if got := root.PointerShapeAt(tabX+5, editorY); got != term.PointerText {
		t.Errorf("after release over editor = %q, want %q", got, term.PointerText)
	}

	// Divider should return to PointerNSResize
	if got := root.PointerShapeAt(tabX+5, divY); got != term.PointerNSResize {
		t.Errorf("after release over divider = %q, want %q", got, term.PointerNSResize)
	}

	// Tab should return to PointerHand
	if got := root.PointerShapeAt(tabX, tabsY); got != term.PointerHand {
		t.Errorf("after release over tab = %q, want %q", got, term.PointerHand)
	}

	_ = reordered
}

func TestSidebarTabDragPointerGrabbingAcrossPanels(t *testing.T) {
	sidebar := NewSidebarWidget()
	sidebar.Tabs.Config.Reorderable = true
	var reordered bool
	sidebar.Tabs.Config.OnReorder = func(from, to int) {
		sidebar.MovePanel(from, to)
		reordered = true
	}
	sidebar.AddPanel("explore", "Explore", &mockWidget{})
	sidebar.AddPanel("search", "Search", &mockWidget{})

	right := &shapeWidget{shape: term.PointerText}
	right.SetRect(Rect{X: 21, Y: 0, W: 60, H: 30})

	split := NewSplitPanelWidget()
	split.Left = sidebar
	split.Right = right
	split.ShowLeft = true
	split.DividerPos = 20
	split.SetRect(Rect{X: 0, Y: 0, W: 80, H: 30})

	root := NewRoot(split)
	root.SetSize(80, 30)

	grid := makeGrid(80, 30)
	root.Render(grid)

	tabsY := sidebar.GetRect().Y
	tabX := 3
	divX := split.DividerScreenX()

	// Initial hover over tab should be PointerHand
	if got := root.PointerShapeAt(tabX, tabsY); got != term.PointerHand {
		t.Fatalf("hover over sidebar tab = %q, want %q", got, term.PointerHand)
	}

	// 1. Mouse down on tab starts drag gesture
	root.HandleEvent(tcell.NewEventMouse(tabX, tabsY, tcell.Button1, 0))
	if !sidebar.Tabs.PointerGestureActive() {
		t.Fatal("sidebar tab press did not start a pointer gesture")
	}

	// 2. Move horizontally beyond drag threshold
	root.HandleEvent(tcell.NewEventMouse(tabX+5, tabsY, tcell.Button1, 0))
	root.Render(grid)

	if !sidebar.Tabs.OwnsPointerCapture() {
		t.Fatal("expected sidebar.Tabs to own pointer capture after crossing drag threshold")
	}

	// 3. Pointer shape must remain PointerGrabbing over sidebar tab bar
	if got := root.PointerShapeAt(tabX+5, tabsY); got != term.PointerGrabbing {
		t.Errorf("dragging over sidebar tab bar = %q, want %q", got, term.PointerGrabbing)
	}

	// 4. Pointer shape must remain PointerGrabbing over right panel
	rightX := 40
	if got := root.PointerShapeAt(rightX, tabsY); got != term.PointerGrabbing {
		t.Errorf("dragging over right panel = %q, want %q", got, term.PointerGrabbing)
	}

	// 5. Pointer shape must remain PointerGrabbing over sidebar divider
	if got := root.PointerShapeAt(divX, 10); got != term.PointerGrabbing {
		t.Errorf("dragging over sidebar divider = %q, want %q", got, term.PointerGrabbing)
	}

	// 6. Mouse release ends drag and restores normal pointers
	root.HandleEvent(tcell.NewEventMouse(tabX+5, tabsY, tcell.ButtonNone, 0))
	root.Render(grid)

	if sidebar.Tabs.PointerGestureActive() {
		t.Fatal("sidebar gesture should be inactive after mouse release")
	}

	// Right pane should return to PointerText
	if got := root.PointerShapeAt(rightX, tabsY); got != term.PointerText {
		t.Errorf("after release over right pane = %q, want %q", got, term.PointerText)
	}

	// Divider should return to PointerEWResize
	if got := root.PointerShapeAt(divX, 10); got != term.PointerEWResize {
		t.Errorf("after release over divider = %q, want %q", got, term.PointerEWResize)
	}

	_ = reordered
}

func TestBottomPanelOwnsPointerCaptureLifecycle(t *testing.T) {
	bp := NewBottomPanelWidget(nil)
	bp.Tabs.Config.Reorderable = true
	bp.AddPanel("p1", "Panel 1", &mockWidget{})
	bp.AddPanel("p2", "Panel 2", &mockWidget{})
	bp.SetRect(Rect{X: 0, Y: 0, W: 50, H: 10})

	var invalidated bool
	bp.SetPointerCaptureInvalidated(func() {
		invalidated = true
	})

	grid := makeGrid(50, 10)
	bp.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 50, H: 10}))

	if bp.OwnsPointerCapture() {
		t.Error("expected OwnsPointerCapture to be false initially")
	}

	// Start gesture on tab 1
	bp.HandleEvent(tcell.NewEventMouse(2, 0, tcell.Button1, 0))
	if !bp.OwnsPointerCapture() {
		t.Error("expected OwnsPointerCapture to be true after tab press")
	}

	// When hidden or height < 3, OwnsPointerCapture returns false
	bp.Visible = false
	if bp.OwnsPointerCapture() {
		t.Error("expected OwnsPointerCapture to be false when hidden")
	}
	bp.Visible = true

	bp.SetRect(Rect{X: 0, Y: 0, W: 50, H: 2})
	if bp.OwnsPointerCapture() {
		t.Error("expected OwnsPointerCapture to be false when H < 3")
	}
	bp.SetRect(Rect{X: 0, Y: 0, W: 50, H: 10})

	// Cancel pointer capture
	if !bp.CancelPointerCapture() {
		t.Error("CancelPointerCapture returned false")
	}
	if bp.OwnsPointerCapture() {
		t.Error("expected OwnsPointerCapture to be false after cancel")
	}
	if !invalidated {
		t.Error("expected invalidation callback to fire")
	}
}
