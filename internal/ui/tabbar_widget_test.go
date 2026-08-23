package ui

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestTabBarRender(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{
		{Name: "main.go", Active: true},
		{Name: "buf.go", Dirty: true},
	})
	tb.SetRect(Rect{X: 0, Y: 0, W: 30, H: 3})

	grid := makeGrid(30, 3)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 30, H: 3})
	tb.Render(surface)

	// Row 0: top border of active tab
	if grid[0][0].Ch != '┌' {
		t.Fatalf("expected TopLeft at row 0 col 0, got '%c'", grid[0][0].Ch)
	}
	// Row 1: active tab label with │ sides
	if grid[1][0].Ch != '│' {
		t.Fatalf("expected Vertical at row 1 col 0, got '%c'", grid[1][0].Ch)
	}
	if grid[1][2].Ch != 'm' {
		t.Fatalf("expected 'm' at row 1 col 2, got '%c'", grid[1][2].Ch)
	}
	if grid[1][2].Style != term.StyleActiveTab {
		t.Fatal("active tab should have StyleActiveTab")
	}
	// Row 2: baseline with gap
	if grid[2][0].Ch != '┘' {
		t.Fatalf("expected BottomRight at row 2 col 0, got '%c'", grid[2][0].Ch)
	}
}

func TestTabBarOverflowArrows(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{
		{Name: "main.go", Active: true, Closable: true},
		{Name: "buffer.go"},
		{Name: "cursor.go"},
		{Name: "highlight.go"},
		{Name: "undo.go"},
		{Name: "selection.go"},
	})
	tb.SetRect(Rect{X: 0, Y: 0, W: 30, H: 3})

	grid := makeGrid(30, 3)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 30, H: 3})
	tb.Render(surface)

	if tb.hasOverflowLeft {
		t.Fatal("should not have left overflow when active tab is first")
	}
	if !tb.hasOverflowRight {
		t.Fatal("should have right overflow with narrow width")
	}
	// Right arrow is at innerRight+1 = (30-3)+1 = 28
	if grid[1][28].Ch != '▶' {
		t.Fatalf("expected right arrow at row 1 col 28, got '%c'", grid[1][28].Ch)
	}
}

func TestTabBarOverflowScrollLeft(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{
		{Name: "main.go"},
		{Name: "buffer.go"},
		{Name: "cursor.go"},
		{Name: "highlight.go"},
		{Name: "undo.go", Active: true, Closable: true},
	})
	tb.SetRect(Rect{X: 0, Y: 0, W: 30, H: 3})

	grid := makeGrid(30, 3)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 30, H: 3})
	tb.Render(surface)

	if !tb.hasOverflowLeft {
		t.Fatal("should have left overflow when active tab is scrolled right")
	}
	// Left arrow " ◀ " — chevron is at col 1
	if grid[1][1].Ch != '◀' {
		t.Fatalf("expected left arrow at row 1 col 1, got '%c'", grid[1][1].Ch)
	}
}

func TestTabBarDragCapturesPendingPressWithThemedIndicator(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{
		{Name: "one.go", Active: true},
		{Name: "two.go"},
		{Name: "three.go"},
	})
	tb.SetRect(Rect{X: 4, Y: 2, W: 50, H: 3})
	grid := makeGrid(50, 3)
	tb.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 50, H: 3}))

	fromX := tb.GetRect().X + tb.tabSpans[0].start + 2
	toX := tb.GetRect().X + tb.tabSpans[2].end - 2
	var gotFrom, gotTo = -1, -1
	tb.OnTabReorder = func(from, to int) { gotFrom, gotTo = from, to }

	if got := tb.HandleEvent(tcell.NewEventMouse(fromX, 3, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("mouse down = %v, want captured", got)
	}
	if got := tb.HandleEvent(tcell.NewEventMouse(fromX+1, 3, tcell.Button1, 0)); got != EventConsumed {
		t.Fatalf("jitter = %v, want consumed", got)
	}
	if got := tb.HandleEvent(tcell.NewEventMouse(toX, 3, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("drag = %v, want captured", got)
	}
	tb.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 50, H: 3}))
	indicatorX := tb.dropIndicatorX()
	if indicatorX < 0 || grid[1][indicatorX].Ch != '│' || grid[1][indicatorX].Style != term.StyleBorderActive {
		t.Fatal("drag should render a themed drop indicator")
	}
	tb.HandleEvent(tcell.NewEventMouse(toX, 3, tcell.ButtonNone, 0))
	if gotFrom != 0 || gotTo != 2 {
		t.Fatalf("reorder = %d -> %d, want 0 -> 2", gotFrom, gotTo)
	}
}

func TestTabBarCapturedClickActivatesWithoutReorder(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{{Name: "one.go", Active: true}, {Name: "two.go"}})
	tb.SetRect(Rect{X: 0, Y: 0, W: 30, H: 3})
	tb.Render(NewRenderSurface(makeGrid(30, 3), Rect{X: 0, Y: 0, W: 30, H: 3}))

	clicked, reordered := -1, false
	tb.OnTabClick = func(index int) { clicked = index }
	tb.OnTabReorder = func(_, _ int) { reordered = true }
	x := tb.tabSpans[1].start + 2
	if got := tb.HandleEvent(tcell.NewEventMouse(x, 1, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("mouse down = %v, want captured", got)
	}
	tb.HandleEvent(tcell.NewEventMouse(x+1, 1, tcell.Button1, 0))
	tb.HandleEvent(tcell.NewEventMouse(x+1, 1, tcell.ButtonNone, 0))

	if clicked != 1 {
		t.Fatalf("clicked tab = %d, want 1", clicked)
	}
	if reordered {
		t.Fatal("one-column jitter reordered a tab")
	}
	if tb.drag.Active() {
		t.Fatal("click release left a pending tab gesture")
	}
}

func TestTabBarPinnedTargetUsesEffectiveMarkerAndCommitTarget(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{
		{Name: "pin-one.go", Pinned: true},
		{Name: "pin-two.go", Pinned: true},
		{Name: "preview.go"},
		{Name: "later.go"},
	})
	tb.SetRect(Rect{X: 0, Y: 0, W: 80, H: 3})
	tb.Render(NewRenderSurface(makeGrid(80, 3), Rect{X: 0, Y: 0, W: 80, H: 3}))
	tb.NormalizeDropTarget = func(from, to int) int {
		if from >= 2 && to < 2 {
			return 2
		}
		return to
	}
	var moves [][2]int
	tb.OnTabReorder = func(from, to int) { moves = append(moves, [2]int{from, to}) }

	press := func(index int) int {
		x := tb.tabSpans[index].start + 2
		if got := tb.HandleEvent(tcell.NewEventMouse(x, 1, tcell.Button1, 0)); got != EventCaptured {
			t.Fatalf("tab %d mouse down = %v, want captured", index, got)
		}
		return x
	}
	dropX := tb.tabSpans[0].start

	press(2)
	tb.HandleEvent(tcell.NewEventMouse(dropX, 1, tcell.Button1, 0))
	if got := tb.drag.Target(); got != 2 {
		t.Fatalf("first unpinned effective target = %d, want 2", got)
	}
	if got := tb.dropIndicatorX(); got != -1 {
		t.Fatalf("effective no-op indicator = %d, want hidden", got)
	}
	tb.HandleEvent(tcell.NewEventMouse(dropX, 1, tcell.ButtonNone, 0))
	if len(moves) != 0 {
		t.Fatalf("effective no-op committed moves %v", moves)
	}

	press(3)
	tb.HandleEvent(tcell.NewEventMouse(dropX, 1, tcell.Button1, 0))
	if got := tb.drag.Target(); got != 2 {
		t.Fatalf("later unpinned effective target = %d, want 2", got)
	}
	wantIndicator := tb.tabSpans[2].start - tb.ScrollOffset + tb.renderArrowW
	if got := tb.dropIndicatorX(); got != wantIndicator {
		t.Fatalf("boundary indicator = %d, want %d", got, wantIndicator)
	}
	tb.HandleEvent(tcell.NewEventMouse(dropX, 1, tcell.ButtonNone, 0))
	if !slices.Equal(moves, [][2]int{{3, 2}}) {
		t.Fatalf("committed moves = %v, want [[3 2]]", moves)
	}
}

func TestTabBarDragAutoScrollsOverflow(t *testing.T) {
	tb := NewTabBarWidget()
	tabs := make([]Tab, 10)
	for i := range tabs {
		tabs[i] = Tab{Name: fmt.Sprintf("file-%d.go", i), Active: i == 0}
	}
	tb.SetTabs(tabs)
	tb.SetRect(Rect{X: 0, Y: 0, W: 30, H: 3})
	grid := makeGrid(30, 3)
	tb.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 30, H: 3}))
	tb.OnTabReorder = func(_, _ int) {}

	startX := tb.renderArrowW + 2
	tb.HandleEvent(tcell.NewEventMouse(startX, 1, tcell.Button1, 0))
	for range 3 {
		tb.HandleEvent(tcell.NewEventMouse(tb.renderInnerRight-1, 1, tcell.Button1, 0))
		tb.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 30, H: 3}))
	}
	if tb.ScrollOffset == 0 {
		t.Fatal("dragging at the right edge should scroll hidden tabs into view")
	}
}

func TestTabBarDragAutoScrollContinuesAtStationaryEdge(t *testing.T) {
	tb := NewTabBarWidget()
	tabs := make([]Tab, 20)
	for i := range tabs {
		tabs[i] = Tab{Name: fmt.Sprintf("file-%02d.go", i), Active: i == 0}
	}
	tb.SetTabs(tabs)
	tb.SetRect(Rect{X: 0, Y: 0, W: 30, H: 3})
	tb.Render(NewRenderSurface(makeGrid(30, 3), Rect{X: 0, Y: 0, W: 30, H: 3}))
	tb.OnTabReorder = func(_, _ int) {}
	tb.dragAutoScrollDelay = time.Millisecond
	ticks := make(chan uint64, 4)
	tb.PostDragAutoScrollTick = func(generation uint64) { ticks <- generation }
	t.Cleanup(func() { tb.CancelPointerCapture() })

	startX := tb.renderArrowW + 2
	if got := tb.HandleEvent(tcell.NewEventMouse(startX, 1, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("mouse down = %v, want captured", got)
	}
	edgeX := tb.renderInnerRight - 1
	tb.HandleEvent(tcell.NewEventMouse(edgeX, 1, tcell.Button1, 0))
	afterMove := tb.ScrollOffset

	var generation uint64
	select {
	case generation = <-ticks:
	case <-time.After(time.Second):
		t.Fatal("stationary edge did not schedule an auto-scroll tick")
	}
	if !tb.HandleDragAutoScrollTick(generation) {
		t.Fatal("current auto-scroll tick was not applied")
	}
	if tb.ScrollOffset <= afterMove {
		t.Fatalf("stationary pointer scroll offset = %d, want greater than %d", tb.ScrollOffset, afterMove)
	}

	tb.HandleEvent(tcell.NewEventMouse(edgeX, 1, tcell.ButtonNone, 0))
	if tb.HandleDragAutoScrollTick(generation) {
		t.Fatal("release should generation-drop an old auto-scroll tick")
	}
}

func TestTabBarDragAutoScrollDropsTickAfterEdgeExit(t *testing.T) {
	tb := NewTabBarWidget()
	tabs := make([]Tab, 12)
	for i := range tabs {
		tabs[i] = Tab{Name: fmt.Sprintf("file-%02d.go", i), Active: i == 0}
	}
	tb.SetTabs(tabs)
	tb.SetRect(Rect{X: 0, Y: 0, W: 30, H: 3})
	tb.Render(NewRenderSurface(makeGrid(30, 3), Rect{X: 0, Y: 0, W: 30, H: 3}))
	tb.OnTabReorder = func(_, _ int) {}
	tb.dragAutoScrollDelay = time.Hour
	tb.PostDragAutoScrollTick = func(uint64) {}
	t.Cleanup(func() { tb.CancelPointerCapture() })

	startX := tb.renderArrowW + 2
	tb.HandleEvent(tcell.NewEventMouse(startX, 1, tcell.Button1, 0))
	tb.HandleEvent(tcell.NewEventMouse(tb.renderInnerRight-1, 1, tcell.Button1, 0))
	generation := tb.autoScrollGeneration
	if tb.autoScrollTimer == nil {
		t.Fatal("test setup: edge drag did not arm auto-scroll")
	}

	centerX := (tb.renderArrowW + tb.renderInnerRight) / 2
	tb.HandleEvent(tcell.NewEventMouse(centerX, 1, tcell.Button1, 0))
	if tb.autoScrollTimer != nil || tb.autoScrollDirection != 0 {
		t.Fatal("leaving the edge did not cancel auto-scroll")
	}
	if tb.HandleDragAutoScrollTick(generation) {
		t.Fatal("edge exit should generation-drop the old tick")
	}
}

func TestTabBarResizeCancelsCapturedGestureAndClearsGeometry(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{{Name: "one.go", Active: true}, {Name: "two.go"}})
	root := NewRoot(tb)
	root.SetSize(30, 3)
	tb.Render(NewRenderSurface(makeGrid(30, 3), Rect{X: 0, Y: 0, W: 30, H: 3}))
	tb.OnTabReorder = func(_, _ int) {}

	x := tb.tabSpans[0].start + 2
	root.HandleEvent(tcell.NewEventMouse(x, 1, tcell.Button1, 0))
	if root.capturedWidget == nil || !tb.drag.Active() {
		t.Fatal("test setup: pending tab press was not captured")
	}

	root.SetSize(0, 0)
	root.Render(makeGrid(0, 0))
	if root.capturedWidget != nil || tb.drag.Active() {
		t.Fatal("resize did not cancel the captured tab gesture")
	}
	if len(tb.tabSpans) != 0 || tb.renderArrowW != 0 || tb.renderInnerRight != 0 {
		t.Fatal("non-renderable resize retained old tab geometry")
	}
}

func TestTabBarInvalidatedSourceDoesNotCaptureNextClick(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{{Name: "one.go"}, {Name: "two.go", Active: true}})
	root := NewRoot(tb)
	root.SetSize(30, 3)
	tb.Render(NewRenderSurface(makeGrid(30, 3), Rect{X: 0, Y: 0, W: 30, H: 3}))
	clicked := -1
	tb.OnTabClick = func(index int) { clicked = index }
	tb.OnTabReorder = func(_, _ int) {}

	secondX := tb.tabSpans[1].start + 2
	root.HandleEvent(tcell.NewEventMouse(secondX, 1, tcell.Button1, 0))
	if root.capturedWidget == nil {
		t.Fatal("test setup: second tab press was not captured")
	}

	tb.SetTabs([]Tab{{Name: "one.go", Active: true}})
	root.Render(makeGrid(30, 3))
	if root.capturedWidget != nil {
		t.Fatal("invalidated source retained root capture after render")
	}
	clicked = -1
	firstX := tb.tabSpans[0].start + 2
	root.HandleEvent(tcell.NewEventMouse(firstX, 1, tcell.Button1, 0))
	root.HandleEvent(tcell.NewEventMouse(firstX, 1, tcell.ButtonNone, 0))

	if clicked != 0 {
		t.Fatalf("first click after invalidation activated %d, want 0", clicked)
	}
}

func TestTabBarRemovedSourceDoesNotRetargetReplacementAtSameIndex(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{
		{ID: "a", Name: "a.go"},
		{ID: "b", Name: "b.go", Active: true},
		{ID: "c", Name: "c.go"},
	})
	root := NewRoot(tb)
	root.SetSize(40, 3)
	tb.Render(NewRenderSurface(makeGrid(40, 3), Rect{X: 0, Y: 0, W: 40, H: 3}))
	var moves [][2]int
	tb.OnTabReorder = func(from, to int) { moves = append(moves, [2]int{from, to}) }

	pressX := tb.tabSpans[1].start + 2
	if got := root.HandleEvent(tcell.NewEventMouse(pressX, 1, tcell.Button1, 0)); got != EventConsumed {
		t.Fatalf("b mouse down = %v, want consumed by Root", got)
	}
	if got := tb.drag.SourceID(); got != "b" {
		t.Fatalf("pressed source = %q, want b", got)
	}

	tb.SetTabs([]Tab{{ID: "a", Name: "a.go"}, {ID: "c", Name: "c.go"}, {ID: "d", Name: "d.go"}})
	if root.capturedWidget != nil {
		t.Fatal("removed b did not synchronously clear Root capture")
	}
	if tb.OwnsPointerCapture() {
		t.Fatal("removed b retained capture when c occupied index 1")
	}
	tb.HandleEvent(tcell.NewEventMouse(tb.tabSpans[2].start+2, 1, tcell.ButtonNone, 0))
	if len(moves) != 0 {
		t.Fatalf("release after source removal reordered replacement: %v", moves)
	}
}

func TestTabBarRapidRemoveReopenDoesNotReviveGesture(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{{ID: "a", Name: "a.go"}, {ID: "b", Name: "b.go"}})
	tb.SetRect(Rect{X: 0, Y: 0, W: 30, H: 3})
	tb.Render(NewRenderSurface(makeGrid(30, 3), Rect{X: 0, Y: 0, W: 30, H: 3}))
	reordered := false
	tb.OnTabReorder = func(_, _ int) { reordered = true }
	tb.HandleEvent(tcell.NewEventMouse(tb.tabSpans[1].start+2, 1, tcell.Button1, 0))

	tb.SetTabs([]Tab{{ID: "a", Name: "a.go"}})
	tb.SetTabs([]Tab{{ID: "a", Name: "a.go"}, {ID: "b", Name: "b.go"}})
	tb.HandleEvent(tcell.NewEventMouse(0, 1, tcell.ButtonNone, 0))
	if tb.OwnsPointerCapture() || reordered {
		t.Fatal("remove/reopen revived the canceled b gesture")
	}
}

func TestTabBarSourceRemovalSynchronouslyClearsNestedCaptureChain(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{{ID: "a", Name: "a.go"}, {ID: "b", Name: "b.go"}, {ID: "c", Name: "c.go"}})
	tb.OnTabReorder = func(_, _ int) {}
	content := NewContentSplitWidget()
	content.Top = tb
	split := NewSplitPanelWidget()
	split.ShowLeft = false
	split.Right = content
	main := widgets.NewVStackWidget(split)
	root := NewRoot(main)
	root.SetSize(50, 6)
	root.Render(makeGrid(50, 6))

	pressX := tb.GetRect().X + tb.tabSpans[1].start + 2
	pressY := tb.GetRect().Y + 1
	root.HandleEvent(tcell.NewEventMouse(pressX, pressY, tcell.Button1, 0))
	if root.capturedWidget == nil || split.capturedChild == nil || content.capturedChild == nil {
		t.Fatal("test setup did not establish the nested capture chain")
	}

	tb.SetTabs([]Tab{{ID: "a", Name: "a.go"}, {ID: "c", Name: "c.go"}, {ID: "d", Name: "d.go"}})
	if root.capturedWidget != nil || split.capturedChild != nil || content.capturedChild != nil {
		t.Fatal("source removal did not synchronously clear every capture layer")
	}
}

// renderInMoreButtonWindow sizes the bar to exactly the total tab width with a
// MoreButton present, so tabs overflow the inner zone but not the full width —
// the #354 window. Returns the rendered grid and the bar width.
func renderInMoreButtonWindow(t *testing.T) (*TabBarWidget, [][]term.Cell, int) {
	t.Helper()
	tb := NewTabBarWidget()
	tb.MoreButton = NewMoreButtonWidget()
	tb.SetTabs([]Tab{
		{Name: "main.go", Active: true, Closable: true},
		{Name: "buffer.go"},
		{Name: "cursor.go"},
	})

	tb.SetRect(Rect{X: 0, Y: 0, W: 200, H: 3})
	probe := makeGrid(200, 3)
	tb.Render(NewRenderSurface(probe, Rect{X: 0, Y: 0, W: 200, H: 3}))
	w := tb.totalTabWidth

	tb.SetRect(Rect{X: 0, Y: 0, W: w, H: 3})
	grid := makeGrid(w, 3)
	tb.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: w, H: 3}))
	return tb, grid, w
}

// TestTabBarCloseHitInMoreButtonWindow guards issue #354: clicking the active
// tab's rendered × in the MoreButton window must close it (was a dead click).
func TestTabBarCloseHitInMoreButtonWindow(t *testing.T) {
	tb, grid, w := renderInMoreButtonWindow(t)

	if tb.renderArrowW == 0 {
		t.Fatal("expected the arrow gutter to be reserved once the strip overflows the inner zone")
	}

	// Find the rendered close × of the active tab (row 1, StyleActiveTab).
	closeX := -1
	for x := 0; x < w; x++ {
		if grid[1][x].Ch == 'x' && grid[1][x].Style == term.StyleActiveTab {
			closeX = x
		}
	}
	if closeX < 0 {
		t.Fatal("test setup: could not find rendered close × for active tab")
	}

	closed := -1
	tb.OnTabClose = func(i int) { closed = i }

	// A real click is mouse-down then mouse-up at the same cell.
	tb.HandleEvent(tcell.NewEventMouse(closeX, 1, tcell.Button1, 0))
	tb.HandleEvent(tcell.NewEventMouse(closeX, 1, tcell.ButtonNone, 0))

	if closed != 0 {
		t.Fatalf("clicking the visible close × should close tab 0, got closed=%d", closed)
	}
}

// TestTabBarChevronNotDrawnOverTab: when a chevron shows, its gutter must be
// reserved so tabs never render on top of it.
func TestTabBarChevronNotDrawnOverTab(t *testing.T) {
	tb, grid, _ := renderInMoreButtonWindow(t)

	if (tb.hasOverflowLeft || tb.hasOverflowRight) && tb.renderArrowW == 0 {
		t.Fatal("chevron shown without a reserved gutter — tabs will overlap it")
	}
	if tb.hasOverflowLeft && grid[1][1].Ch != '◀' {
		t.Fatalf("left chevron cell overwritten by a tab, got '%c'", grid[1][1].Ch)
	}
}

// TestTabBarNoOverScrollAfterClose: closing tabs must not leave the strip scrolled
// past the last tab (only the final tab visible with empty space to its right).
func TestTabBarNoOverScrollAfterClose(t *testing.T) {
	tb := NewTabBarWidget()
	tb.MoreButton = NewMoreButtonWidget()

	many := make([]Tab, 20)
	for i := range many {
		many[i] = Tab{Name: "untitled-" + string(rune('a'+i)) + ".go"}
	}
	many[19].Active = true
	tb.SetTabs(many)
	tb.SetRect(Rect{X: 0, Y: 0, W: 40, H: 3})
	tb.Render(NewRenderSurface(makeGrid(40, 3), Rect{X: 0, Y: 0, W: 40, H: 3}))
	if tb.ScrollOffset == 0 {
		t.Fatal("test setup: expected a non-zero scroll offset with 20 tabs at width 40")
	}

	// Close down to three tabs, first active (like closing everything to the right).
	tb.SetTabs([]Tab{
		{Name: "untitled-a.go", Active: true},
		{Name: "untitled-b.go"},
		{Name: "untitled-c.go"},
	})
	tb.SetRect(Rect{X: 0, Y: 0, W: 40, H: 3})
	tb.Render(NewRenderSurface(makeGrid(40, 3), Rect{X: 0, Y: 0, W: 40, H: 3}))

	if tb.ScrollOffset != 0 {
		t.Fatalf("offset should snap back to 0 once the tabs fit, got %d", tb.ScrollOffset)
	}
	if tb.hasOverflowLeft {
		t.Fatal("no left overflow expected once the tabs fit")
	}
}

// TestTabBarGutterClickDoesNotSpawnTab: at the first tab the ◀ is hidden but its
// gutter is still reserved. Double-clicking that empty gutter must be a no-op —
// it must NOT fall through to the empty-space double-click handler and spawn a
// tab (which looked like "jumping to the other side").
func TestTabBarGutterClickDoesNotSpawnTab(t *testing.T) {
	tb := NewTabBarWidget()
	tb.MoreButton = NewMoreButtonWidget()

	const n = 8
	tabs := make([]Tab, n)
	for i := range tabs {
		tabs[i] = Tab{Name: fmt.Sprintf("untitled-%d.go", i+1), Active: i == 0, Closable: i == 0}
	}
	tb.SetTabs(tabs)

	doubleClicks, prevTabs := 0, 0
	tb.OnDoubleClick = func() { doubleClicks++ }
	tb.OnPrevTab = func() { prevTabs++ }

	r := Rect{X: 0, Y: 0, W: 40, H: 3}
	tb.SetRect(r)
	tb.Render(NewRenderSurface(makeGrid(40, 3), r))
	if tb.hasOverflowLeft || tb.renderArrowW == 0 {
		t.Fatalf("test setup: want hidden ◀ with reserved gutter, got ovL=%v arrowW=%d",
			tb.hasOverflowLeft, tb.renderArrowW)
	}

	// Two clicks on the empty left gutter (where ◀ would be), fast enough to be a
	// double-click.
	for i := 0; i < 2; i++ {
		tb.HandleEvent(tcell.NewEventMouse(r.X+1, 1, tcell.Button1, 0))
		tb.HandleEvent(tcell.NewEventMouse(r.X+1, 1, tcell.ButtonNone, 0))
	}

	if doubleClicks != 0 {
		t.Fatalf("gutter double-click spawned a tab (OnDoubleClick fired %d times)", doubleClicks)
	}
	if prevTabs != 0 {
		t.Fatalf("gutter click on a hidden ◀ scrolled (OnPrevTab fired %d times)", prevTabs)
	}
}

func TestTabBarNoOverflowWhenFits(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{
		{Name: "a.go", Active: true},
		{Name: "b.go"},
	})
	tb.SetRect(Rect{X: 0, Y: 0, W: 40, H: 3})

	grid := makeGrid(40, 3)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 3})
	tb.Render(surface)

	if tb.hasOverflowLeft || tb.hasOverflowRight {
		t.Fatal("should not have overflow when all tabs fit")
	}
}

func TestTabBarDragDoesNotStartFromPressOutsideTabs(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{{Name: "one.go", Active: true}, {Name: "two.go"}})
	root := NewRoot(tb)
	root.SetSize(30, 3)
	tb.Render(NewRenderSurface(makeGrid(30, 3), Rect{X: 0, Y: 0, W: 30, H: 3}))
	tb.OnTabClick = func(int) {}
	tb.OnTabReorder = func(_, _ int) {}

	tabX := tb.tabSpans[1].start + 2

	// Press outside the tab bar (below it), then move onto a tab with button held.
	root.HandleEvent(tcell.NewEventMouse(tabX, 5, tcell.Button1, 0))
	root.HandleEvent(tcell.NewEventMouse(tabX, 1, tcell.Button1, 0))

	if tb.drag.Active() {
		t.Fatal("drag started from a press that originated outside the tab bar")
	}
	if root.capturedWidget != nil {
		t.Fatal("root captured widget from a foreign press")
	}

	// Release and do a real click — should work normally.
	root.HandleEvent(tcell.NewEventMouse(tabX, 1, tcell.ButtonNone, 0))
	root.HandleEvent(tcell.NewEventMouse(tabX, 1, tcell.Button1, 0))

	if !tb.drag.Active() {
		t.Fatal("legitimate click after foreign press did not start drag")
	}
}
