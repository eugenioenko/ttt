package ui

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestEditorTabDragPointerGrabbingAcrossPanels(t *testing.T) {
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	group.OpenFile("alpha.go")
	group.CommitActiveTab()
	group.OpenFile("beta.go")
	group.CommitActiveTab()
	group.OpenFile("gamma.go")
	group.CommitActiveTab()

	group.TabBar.OnTabReorder = func(from, to int) {
		group.MoveTab(from, to)
	}

	bp := NewBottomPanelWidget(nil)
	bp.Tabs.Config.Reorderable = true
	bp.AddPanel("term", "Terminal", &mockWidget{})
	bp.AddPanel("probs", "Diagnostics", &mockWidget{})

	cs := NewContentSplitWidget()
	cs.Top = group
	cs.Bottom = bp
	cs.ShowBottom = true
	cs.BottomH = 8
	cs.SetRect(Rect{X: 20, Y: 1, W: 60, H: 28})

	sidebar := NewSidebarWidget()
	sidebar.AddPanel("files", "Files", &mockWidget{})
	sidebar.SetRect(Rect{X: 0, Y: 1, W: 20, H: 28})

	split := NewSplitPanelWidget()
	split.Left = sidebar
	split.Right = cs
	split.ShowLeft = true
	split.DividerPos = 20
	split.SetRect(Rect{X: 0, Y: 1, W: 80, H: 28})

	menuBar := NewMenuBarWidget([]MenuItem{
		{Name: "File"},
		{Name: "Edit"},
		{Name: "View"},
	})
	menuBar.SetRect(Rect{X: 0, Y: 0, W: 80, H: 1})

	statusBar := widgets.NewLabelWidget(widgets.LabelConfig{Text: "status"})
	statusBar.SetRect(Rect{X: 0, Y: 29, W: 80, H: 1})

	rootBox := widgets.NewVStackWidget(menuBar, split, statusBar)
	root := NewRoot(rootBox)
	root.SetSize(80, 30)

	grid := makeGrid(80, 30)
	root.Render(grid)

	tabsY := group.TabBar.GetRect().Y + 1
	tabSpans := group.TabBar.tabSpans
	if len(tabSpans) < 3 {
		t.Fatalf("expected at least 3 tabs rendered, got %d", len(tabSpans))
	}
	betaX := group.TabBar.GetRect().X + group.TabBar.renderArrowW + tabSpans[1].start + 2
	alphaX := group.TabBar.GetRect().X + group.TabBar.renderArrowW + tabSpans[0].start + 2

	csDivY := cs.DividerScreenY()
	splitDivX := split.DividerScreenX()
	editorY := group.GetRect().Y + 5
	editorX := group.GetRect().X + 5

	// Initial hover checks
	if got := root.PointerShapeAt(betaX, tabsY); got != term.PointerHand {
		t.Fatalf("initial hover over tab = %q, want %q", got, term.PointerHand)
	}
	if got := root.PointerShapeAt(editorX, editorY); got != term.PointerText {
		t.Fatalf("initial hover over editor = %q, want %q", got, term.PointerText)
	}
	if got := root.PointerShapeAt(editorX, csDivY); got != term.PointerNSResize {
		t.Fatalf("initial hover over content split divider = %q, want %q", got, term.PointerNSResize)
	}
	if got := root.PointerShapeAt(splitDivX, editorY); got != term.PointerEWResize {
		t.Fatalf("initial hover over split divider = %q, want %q", got, term.PointerEWResize)
	}

	// 1. Mouse down on tab starts drag gesture
	root.HandleEvent(tcell.NewEventMouse(betaX, tabsY, tcell.Button1, 0))
	if !group.PointerGestureActive() {
		t.Fatal("tab press did not start a pointer gesture")
	}

	// 2. Move horizontally beyond drag threshold (2 columns)
	root.HandleEvent(tcell.NewEventMouse(betaX+5, tabsY, tcell.Button1, 0))
	root.Render(grid)

	if !group.TabBar.OwnsPointerCapture() {
		t.Fatal("expected group.TabBar to own pointer capture after crossing threshold")
	}
	if !group.OwnsPointerCapture() {
		t.Fatal("expected group to own pointer capture after crossing threshold")
	}

	// 3. Pointer shape must remain PointerGrabbing over all regions:
	// Over tab bar
	if got := root.PointerShapeAt(betaX+5, tabsY); got != term.PointerGrabbing {
		t.Errorf("dragging over tab bar = %q, want %q", got, term.PointerGrabbing)
	}
	// Over editor text
	if got := root.PointerShapeAt(editorX, editorY); got != term.PointerGrabbing {
		t.Errorf("dragging over editor text = %q, want %q", got, term.PointerGrabbing)
	}
	// Over content split divider
	if got := root.PointerShapeAt(editorX, csDivY); got != term.PointerGrabbing {
		t.Errorf("dragging over content split divider = %q, want %q", got, term.PointerGrabbing)
	}
	// Over bottom panel
	bpTabsY := bp.GetRect().Y
	if got := root.PointerShapeAt(editorX, bpTabsY); got != term.PointerGrabbing {
		t.Errorf("dragging over bottom panel tabs = %q, want %q", got, term.PointerGrabbing)
	}
	if got := root.PointerShapeAt(editorX, bpTabsY+3); got != term.PointerGrabbing {
		t.Errorf("dragging over bottom panel content = %q, want %q", got, term.PointerGrabbing)
	}
	// Over split divider
	if got := root.PointerShapeAt(splitDivX, editorY); got != term.PointerGrabbing {
		t.Errorf("dragging over split divider = %q, want %q", got, term.PointerGrabbing)
	}
	// Over sidebar
	if got := root.PointerShapeAt(5, editorY); got != term.PointerGrabbing {
		t.Errorf("dragging over sidebar = %q, want %q", got, term.PointerGrabbing)
	}
	// Over menu bar (row 0)
	if got := root.PointerShapeAt(10, 0); got != term.PointerGrabbing {
		t.Errorf("dragging over menu bar = %q, want %q", got, term.PointerGrabbing)
	}
	// Over status bar (row 29)
	if got := root.PointerShapeAt(10, 29); got != term.PointerGrabbing {
		t.Errorf("dragging over status bar = %q, want %q", got, term.PointerGrabbing)
	}

	// 4. Moving over menu bar must not trigger menu open or break capture
	root.HandleEvent(tcell.NewEventMouse(10, 0, tcell.Button1, 0))
	if root.HasOverlay() {
		t.Fatal("dragging over menu bar opened a menu overlay")
	}
	if !group.PointerGestureActive() {
		t.Fatal("dragging over menu bar canceled the pointer gesture")
	}
	if got := root.PointerShapeAt(10, 0); got != term.PointerGrabbing {
		t.Errorf("pointer shape after move over menu bar = %q, want %q", got, term.PointerGrabbing)
	}

	// 5. Drag back over alpha.go position and release to reorder
	root.HandleEvent(tcell.NewEventMouse(alphaX, tabsY, tcell.Button1, 0))
	root.Render(grid)
	root.HandleEvent(tcell.NewEventMouse(alphaX, tabsY, tcell.ButtonNone, 0))
	root.Render(grid)

	if group.PointerGestureActive() {
		t.Fatal("gesture should be inactive after mouse release")
	}

	// Verify tab reorder: beta.go was dragged before alpha.go
	want := []string{"beta.go", "alpha.go", "gamma.go"}
	if got := tabPaths(group); !slices.Equal(got, want) {
		t.Fatalf("after reorder tabs = %v, want %v", got, want)
	}

	// Verify shapes returned to normal
	if got := root.PointerShapeAt(editorX, editorY); got != term.PointerText {
		t.Errorf("after release over editor = %q, want %q", got, term.PointerText)
	}
	if got := root.PointerShapeAt(editorX, csDivY); got != term.PointerNSResize {
		t.Errorf("after release over content divider = %q, want %q", got, term.PointerNSResize)
	}
	if got := root.PointerShapeAt(splitDivX, editorY); got != term.PointerEWResize {
		t.Errorf("after release over split divider = %q, want %q", got, term.PointerEWResize)
	}
	if got := root.PointerShapeAt(betaX, tabsY); got != term.PointerHand {
		t.Errorf("after release over tab = %q, want %q", got, term.PointerHand)
	}
}

func TestEditorTabDragPointerOverHoverAndAutocomplete(t *testing.T) {
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	group.OpenFile("one.go")
	group.CommitActiveTab()
	group.OpenFile("two.go")
	group.CommitActiveTab()

	root := NewRoot(group)
	root.SetSize(80, 30)

	grid := makeGrid(80, 30)
	root.Render(grid)

	tabsY := group.TabBar.GetRect().Y + 1
	tabX := group.TabBar.GetRect().X + group.TabBar.tabSpans[1].start + 1

	// Mouse down on tab starts drag gesture
	root.HandleEvent(tcell.NewEventMouse(tabX, tabsY, tcell.Button1, 0))
	// Move horizontally beyond drag threshold (2 columns)
	root.HandleEvent(tcell.NewEventMouse(tabX+5, tabsY, tcell.Button1, 0))
	root.Render(grid)

	if !group.TabBar.drag.Dragging() {
		t.Fatal("expected tab to be dragging")
	}

	// Attach a Hover popup over editor text during drag
	hover := NewHoverWidget("documentation text", 15, 10)
	hover.SetRect(Rect{X: 10, Y: 10, W: 30, H: 5})
	group.Hover = hover

	// Check shape over the hover popup: must remain PointerGrabbing
	hoverX, hoverY := 15, 12
	if got := root.PointerShapeAt(hoverX, hoverY); got != term.PointerGrabbing {
		t.Fatalf("dragging over hover popup = %q, want %q", got, term.PointerGrabbing)
	}

	// Attach an Autocomplete popup over editor text
	ac := NewAutocompleteWidget([]CompletionItem{{Label: "item1"}}, 15, 10)
	ac.SetRect(Rect{X: 10, Y: 10, W: 20, H: 5})
	group.Autocomplete = ac

	if got := root.PointerShapeAt(hoverX, hoverY); got != term.PointerGrabbing {
		t.Fatalf("dragging over autocomplete popup = %q, want %q", got, term.PointerGrabbing)
	}

	// Mouse release ends drag
	root.HandleEvent(tcell.NewEventMouse(tabX+5, tabsY, tcell.ButtonNone, 0))
	root.Render(grid)

	if group.PointerGestureActive() {
		t.Fatal("gesture should be inactive after release")
	}
}

func TestEditorTabReleaseWithoutDragDoesNotClickEditor(t *testing.T) {
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	group.OpenFile("one.go")
	group.CommitActiveTab()
	group.OpenFile("two.go")
	group.CommitActiveTab()

	root := NewRoot(group)
	root.SetSize(80, 30)

	grid := makeGrid(80, 30)
	root.Render(grid)

	tabsY := group.TabBar.GetRect().Y + 1
	tabX := group.TabBar.GetRect().X + group.TabBar.tabSpans[0].start + 1

	// Mouse down on tab 0
	root.HandleEvent(tcell.NewEventMouse(tabX, tabsY, tcell.Button1, 0))
	if !group.PointerGestureActive() {
		t.Fatal("expected pointer gesture to be active")
	}

	// Mouse up without moving (not dragged)
	res := root.HandleEvent(tcell.NewEventMouse(tabX, tabsY, tcell.ButtonNone, 0))
	if res != EventConsumed {
		t.Fatalf("release event result = %v, want EventConsumed", res)
	}

	if group.PointerGestureActive() {
		t.Fatal("expected pointer gesture to be inactive after release")
	}
	if group.Editor.Selection.Active {
		t.Fatal("release without drag should not activate editor selection")
	}
}
