package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

// testSurface implements Surface for testing. Tracks cells and sub-surface offsets.
type testSurface struct {
	w, h   int
	ox, oy int // absolute origin offset
	cells  [][]term.Cell
}

func newTestSurface(w, h int) *testSurface {
	cells := make([][]term.Cell, h)
	for y := range h {
		cells[y] = make([]term.Cell, w)
	}
	return &testSurface{w: w, h: h, cells: cells}
}

func (s *testSurface) Size() (int, int)   { return s.w, s.h }
func (s *testSurface) Origin() (int, int) { return s.ox, s.oy }
func (s *testSurface) SetCell(x, y int, c term.Cell) {
	ax, ay := x+s.ox, y+s.oy
	if ax >= 0 && ax < len(s.cells[0]) && ay >= 0 && ay < len(s.cells) {
		s.cells[ay][ax] = c
	}
}
func (s *testSurface) DrawText(x, y int, text string, maxW int, style term.Style) int {
	limit := s.w
	if maxW > 0 && maxW < limit {
		limit = maxW
	}
	for _, ch := range text {
		if x >= limit {
			break
		}
		width := textwidth.Rune(ch)
		if x+width > limit {
			s.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
			x++
			break
		}
		s.SetCell(x, y, term.Cell{Ch: ch, Style: style})
		x += width
	}
	return x
}
func (s *testSurface) DrawBorder(x, y, w, h int, b term.BorderSet, style term.Style) {
	for i := x + 1; i < x+w-1; i++ {
		s.SetCell(i, y, term.Cell{Ch: b.Horizontal, Style: style})
		s.SetCell(i, y+h-1, term.Cell{Ch: b.Horizontal, Style: style})
	}
	for j := y + 1; j < y+h-1; j++ {
		s.SetCell(x, j, term.Cell{Ch: b.Vertical, Style: style})
		s.SetCell(x+w-1, j, term.Cell{Ch: b.Vertical, Style: style})
	}
	s.SetCell(x, y, term.Cell{Ch: b.TopLeft, Style: style})
	s.SetCell(x+w-1, y, term.Cell{Ch: b.TopRight, Style: style})
	s.SetCell(x, y+h-1, term.Cell{Ch: b.BottomLeft, Style: style})
	s.SetCell(x+w-1, y+h-1, term.Cell{Ch: b.BottomRight, Style: style})
}
func (s *testSurface) ClearRect(x, y, w, h int, style term.Style) {}
func (s *testSurface) Fill(c term.Cell)                           {}
func (s *testSurface) Sub(r Rect) Surface {
	return &testSurface{
		w: r.W, h: r.H,
		ox: s.ox + r.X, oy: s.oy + r.Y,
		cells: s.cells,
	}
}

func mouseClick(x, y int) *tcell.EventMouse {
	return tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone)
}

func mouseRelease(x, y int) *tcell.EventMouse {
	return tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone)
}

// renderWidget sets rect and renders, simulating what the layout system does.
func renderWidget(w Widget, x, y, width, height int) *testSurface {
	s := newTestSurface(width, height)
	w.SetRect(Rect{X: x, Y: y, W: width, H: height})
	w.Render(s)
	return s
}

func TestInputClickFocus(t *testing.T) {
	inp := NewInputWidget(InputConfig{Bordered: true})

	// Simulate layout: place input at (5, 10) with width 20, height 3
	renderWidget(inp, 5, 10, 20, 3)

	r := inp.GetRect()
	t.Logf("input rect: X=%d Y=%d W=%d H=%d", r.X, r.Y, r.W, r.H)

	if r.W == 0 || r.H == 0 {
		t.Fatal("input rect is zero after render")
	}

	// Set up focus manager
	fm := NewFocusManager()
	fm.SetActive(true)
	vs := NewVStackWidget(inp)
	vs.SetRect(Rect{X: 5, Y: 10, W: 20, H: 3})
	fm.Collect(vs)

	if len(fm.items) != 1 {
		t.Fatalf("expected 1 focusable, got %d", len(fm.items))
	}
	if !inp.IsFocused() {
		t.Error("input should be auto-focused after collect")
	}

	// Click inside the input (text area is at x+1, y+1 for bordered)
	click := mouseClick(6, 11)
	fm.FocusByClick(6, 11)
	if !inp.IsFocused() {
		t.Error("input should remain focused after click inside")
	}

	handled := inp.HandleEvent(click)
	if handled != EventConsumed {
		t.Error("input should handle click inside its rect")
	}
}

func TestInputClickOutsideRect(t *testing.T) {
	inp := NewInputWidget(InputConfig{Bordered: true})
	renderWidget(inp, 5, 10, 20, 3)

	click := mouseClick(0, 0)
	handled := inp.HandleEvent(click)
	if handled == EventConsumed {
		t.Error("input should NOT handle click outside its rect")
	}
}

func TestTabbedTabSwitchFocus(t *testing.T) {
	// Tab 0: a tree
	tree := NewTreeWidget(TreeConfig{})

	// Tab 1: a vstack with two inputs
	inp1 := NewInputWidget(InputConfig{Bordered: true})
	inp2 := NewInputWidget(InputConfig{Bordered: true})
	settingsVS := NewVStackWidget(
		NewLabelWidget(LabelConfig{Text: "Name"}),
		inp1,
		NewLabelWidget(LabelConfig{Text: "Value"}),
		inp2,
	)

	tabs := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "tab0", Label: "Tab 0"},
			{ID: "tab1", Label: "Tab 1"},
		},
	})

	tabbed := NewTabbedWidget(tabs, []Widget{
		NewVStackWidget(tree),
		settingsVS,
	})

	fm := NewFocusManager()
	fm.SetActive(true)

	// Initial state: tab 0 active
	surface := renderWidget(tabbed, 0, 0, 30, 20)
	_ = surface
	fm.Collect(tabbed)

	t.Logf("tab 0 focusable items: %d", len(fm.items))
	if len(fm.items) != 2 {
		t.Fatalf("tab 0 should have 2 focusables (tree + tabs), got %d", len(fm.items))
	}
	if !tree.IsFocused() {
		t.Error("tree should be auto-focused on tab 0 (content before tabs)")
	}

	// Switch to tab 1
	tabbed.SetActive(1)

	// Re-render so rects update
	renderWidget(tabbed, 0, 0, 30, 20)
	fm.Collect(tabbed)

	t.Logf("tab 1 focusable items: %d", len(fm.items))
	for i, item := range fm.items {
		r := item.GetRect()
		t.Logf("  item %d: rect X=%d Y=%d W=%d H=%d focused=%v", i, r.X, r.Y, r.W, r.H, item.IsFocused())
	}

	if len(fm.items) != 3 {
		t.Fatalf("tab 1 should have 3 focusables (2 inputs + tabs), got %d", len(fm.items))
	}

	// Old tree should NOT be focused
	if tree.IsFocused() {
		t.Error("tree from tab 0 should not be focused after tab switch")
	}

	// First input should be auto-focused (content before tabs)
	if !inp1.IsFocused() {
		t.Error("inp1 should be auto-focused after tab switch")
	}

	// Verify inp2 rect is non-zero
	r2 := inp2.GetRect()
	if r2.W == 0 || r2.H == 0 {
		t.Fatalf("inp2 rect is zero: %+v", r2)
	}

	// Click on inp2
	clickX := r2.X + 1
	clickY := r2.Y + 1
	t.Logf("clicking inp2 at (%d, %d)", clickX, clickY)

	fm.FocusByClick(clickX, clickY)
	if !inp2.IsFocused() {
		t.Error("inp2 should be focused after click")
	}
	if inp1.IsFocused() {
		t.Error("inp1 should NOT be focused after clicking inp2")
	}

	// Also verify handleMouse works
	click := mouseClick(clickX, clickY)
	handled := inp2.HandleEvent(click)
	if handled != EventConsumed {
		t.Errorf("inp2 should handle click at (%d, %d), rect=%+v", clickX, clickY, r2)
	}
}

func TestTabbedClickInputViaFocusManager(t *testing.T) {
	// Simulate the full adapter flow:
	// 1. Build widget tree + focus manager
	// 2. Render (sets rects)
	// 3. Click tab to switch
	// 4. Re-render (sets new rects)
	// 5. Click on input in new tab

	inp1 := NewInputWidget(InputConfig{Bordered: true})
	inp2 := NewInputWidget(InputConfig{Prefix: " ❯ "})
	tree := NewTreeWidget(TreeConfig{})

	tabs := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "containers", Label: "Containers"},
			{ID: "settings", Label: "Settings"},
		},
	})
	tabbed := NewTabbedWidget(tabs, []Widget{
		NewVStackWidget(tree),
		NewVStackWidget(
			NewLabelWidget(LabelConfig{Text: "Name"}),
			inp1,
			NewLabelWidget(LabelConfig{Text: "Cmd"}),
			inp2,
		),
	})

	fm := NewFocusManager()
	fm.SetActive(true)

	// Wire OnChange like the adapter does
	tabbed.OnChange = func(int) {
		fm.Collect(tabbed)
	}

	// Step 1: initial collect (tab 0 active, no rects yet)
	fm.Collect(tabbed)
	t.Logf("after initial collect: %d items", len(fm.items))

	// Step 2: render tab 0
	renderWidget(tabbed, 0, 0, 30, 20)
	t.Logf("tree rect after render: %+v", tree.GetRect())

	// Step 3: simulate clicking on the "Settings" tab
	// The tab bar is at Y=0. Click on it.
	tabY := 0
	tabX := 15 // somewhere on the "Settings" tab label
	click := mouseClick(tabX, tabY)

	// Send through focus manager (like adapter does)
	consumed := fm.HandleEvent(click)
	t.Logf("tab click consumed by focus: %v", consumed)

	// If focus didn't consume it, send through widget tree (like adapter does)
	if consumed != EventConsumed {
		tabbed.HandleEvent(click)
		// Need mouse release for wasPressed tracking
		tabbed.HandleEvent(mouseRelease(tabX, tabY))
	}

	t.Logf("active tab after click: %d", tabbed.active)

	// Step 4: re-render with new tab active
	renderWidget(tabbed, 0, 0, 30, 20)

	t.Logf("after tab switch + re-render: %d focus items", len(fm.items))
	for i, item := range fm.items {
		r := item.GetRect()
		t.Logf("  item %d: rect=%+v focused=%v", i, r, item.IsFocused())
	}

	// Verify we have the right items
	if tabbed.active != 1 {
		t.Fatalf("expected tab 1 active, got %d", tabbed.active)
	}

	// Step 5: click on inp1
	r1 := inp1.GetRect()
	t.Logf("inp1 rect: %+v", r1)
	if r1.W == 0 || r1.H == 0 {
		t.Fatal("inp1 rect is zero after render")
	}

	cx, cy := r1.X+1, r1.Y+1 // inside the bordered input
	t.Logf("clicking inp1 at (%d, %d)", cx, cy)

	inputClick := mouseClick(cx, cy)
	consumed = fm.HandleEvent(inputClick)
	t.Logf("inp1 focus consumed: %v, inp1.focused=%v", consumed, inp1.IsFocused())

	// Simulate adapter: if focus didn't consume, route through widget tree
	if consumed != EventConsumed {
		wHandled := tabbed.HandleEvent(inputClick)
		t.Logf("inp1 widget tree handled: %v", wHandled)
		if wHandled != EventConsumed {
			t.Error("widget tree should handle click on inp1")
		}
	}

	if !inp1.IsFocused() {
		t.Error("inp1 should be focused after click")
	}

	// Step 6: click on inp2
	r2 := inp2.GetRect()
	t.Logf("inp2 rect: %+v", r2)

	cx2, cy2 := r2.X+1, r2.Y
	t.Logf("clicking inp2 at (%d, %d)", cx2, cy2)

	inputClick2 := mouseClick(cx2, cy2)
	consumed = fm.HandleEvent(inputClick2)
	t.Logf("inp2 focus consumed: %v, inp2.focused=%v", consumed, inp2.IsFocused())

	if consumed != EventConsumed {
		wHandled := tabbed.HandleEvent(inputClick2)
		t.Logf("inp2 widget tree handled: %v", wHandled)
		if wHandled != EventConsumed {
			t.Error("widget tree should handle click on inp2")
		}
	}

	if !inp2.IsFocused() {
		t.Error("inp2 should be focused after click")
	}
	if inp1.IsFocused() {
		t.Error("inp1 should NOT be focused after clicking inp2")
	}
}

func TestTabbedClickOnFirstTabAfterSwitch(t *testing.T) {
	// Ensure widgets from tab 0 don't interfere after switching to tab 1
	tree := NewTreeWidget(TreeConfig{})
	inp := NewInputWidget(InputConfig{Bordered: true})

	tabs := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "t0", Label: "T0"},
			{ID: "t1", Label: "T1"},
		},
	})
	tabbed := NewTabbedWidget(tabs, []Widget{
		NewVStackWidget(tree),
		NewVStackWidget(inp),
	})

	fm := NewFocusManager()

	// Render tab 0, collect
	renderWidget(tabbed, 0, 0, 30, 20)
	fm.Collect(tabbed)

	treeRect := tree.GetRect()
	t.Logf("tree rect on tab 0: %+v", treeRect)

	// Switch to tab 1
	tabbed.SetActive(1)
	renderWidget(tabbed, 0, 0, 30, 20)
	fm.Collect(tabbed)

	// tree should no longer be focused
	if tree.IsFocused() {
		t.Error("tree should be unfocused after switching to tab 1")
	}

	// Click where tree WAS - should NOT focus tree
	fm.FocusByClick(treeRect.X+1, treeRect.Y+1)
	if tree.IsFocused() {
		t.Error("tree should NOT get focused by click when on tab 1")
	}
}

func TestTabsKeyboardNavigation(t *testing.T) {
	activated := -1
	tabs := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "a", Label: "Alpha", Active: true},
			{ID: "b", Label: "Beta"},
			{ID: "c", Label: "Gamma"},
		},
		OnTabClick: func(idx int) { activated = idx },
	})
	tabs.SetFocused(true)

	// Right moves selected cursor, does NOT activate
	tabs.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if tabs.selected != 1 {
		t.Fatalf("expected selected=1, got %d", tabs.selected)
	}
	if activated != -1 {
		t.Fatal("right arrow should not activate tab")
	}

	// Right again: 1 -> 2
	tabs.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if tabs.selected != 2 {
		t.Fatalf("expected selected=2, got %d", tabs.selected)
	}

	// Right wraps: 2 -> 0
	tabs.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if tabs.selected != 0 {
		t.Fatalf("expected wrap to 0, got %d", tabs.selected)
	}

	// Left wraps: 0 -> 2
	tabs.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	if tabs.selected != 2 {
		t.Fatalf("expected wrap to 2, got %d", tabs.selected)
	}

	// Enter activates the selected tab
	tabs.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if activated != 2 {
		t.Fatalf("enter should activate selected tab 2, got %d", activated)
	}

	// Space also activates
	activated = -1
	tabs.selected = 1
	tabs.HandleEvent(tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone))
	if activated != 1 {
		t.Fatalf("space should activate selected tab 1, got %d", activated)
	}

	// SetFocused resets selected to active index
	tabs.selected = 2
	tabs.SetFocused(false)
	tabs.SetFocused(true)
	if tabs.selected != 0 {
		t.Fatalf("SetFocused should reset selected to active (0), got %d", tabs.selected)
	}

	// Not focused: should not handle
	tabs.SetFocused(false)
	activated = -1
	handled := tabs.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if handled == EventConsumed {
		t.Error("unfocused tabs should not handle key events")
	}
}

func TestTreeActiveID(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
			{ID: "c", Label: "Gamma"},
		},
	})

	tree.SetActiveID("b")
	if tree.Config.ActiveID != "b" {
		t.Fatalf("expected ActiveID=b, got %s", tree.Config.ActiveID)
	}

	s := renderWidget(tree, 0, 0, 20, 10)

	// Row 1 (Beta) should use StyleSidebarSelected, row 0 (Alpha) uses default
	// Row 0 is also selected (idx==0) so it gets highlight too. Check row 2 is default.
	if s.cells[2][0].Style != term.StyleDefault {
		t.Error("Gamma (non-active, non-selected) should use default style")
	}
	if s.cells[1][0].Style != term.StyleSidebarSelected {
		t.Error("Beta (active) should use sidebar selected style")
	}
}

func TestTreeMutedNodes(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta", Muted: true},
		},
	})

	s := renderWidget(tree, 0, 0, 20, 10)

	// Beta is at row 1, not selected, muted — label chars should use StyleMuted
	if s.cells[1][0].Style != term.StyleMuted {
		t.Errorf("muted node label should use StyleMuted, got %v", s.cells[1][0].Style)
	}

	// Alpha is at row 0, selected — should NOT be muted even if Muted were true
	if s.cells[0][0].Style == term.StyleMuted {
		t.Error("selected node should not use muted style")
	}
}

func TestTreeBadgeStyle(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "a", Label: "A", Badge: "M", BadgeStyle: term.StyleWarning},
			{ID: "b", Label: "B", Badge: "D", BadgeStyle: term.StyleDanger},
			{ID: "c", Label: "C", Badge: "?"},
		},
	})

	// Select node "b" so "a" is unselected and shows its BadgeStyle
	tree.SelectByID("b")
	tree.SetFocused(true)
	s := renderWidget(tree, 0, 0, 20, 10)

	// Row 0 ("A"): badge "M" should render in StyleWarning
	// Label is at x=0, badge has a space gap then starts
	badgeX := 2 // "A" at x=0, space at x=1, badge at x=2
	if s.cells[0][badgeX].Style != term.StyleWarning {
		t.Errorf("badge M should use StyleWarning, got %v", s.cells[0][badgeX].Style)
	}

	// Row 2 ("C"): badge "?" has no BadgeStyle set, should fall back to StyleMuted
	if s.cells[2][badgeX].Style != term.StyleMuted {
		t.Errorf("badge ? with no BadgeStyle should use StyleMuted, got %v", s.cells[2][badgeX].Style)
	}

	// Row 1 ("B"): selected — badge should use selection style, not BadgeStyle
	if s.cells[1][badgeX].Style != term.StyleSidebarSelected {
		t.Errorf("selected node badge should use selection style, got %v", s.cells[1][badgeX].Style)
	}
}

func TestTreeOnExpand(t *testing.T) {
	expanded := []string{}
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:    "root",
				Label: "Root",
				Children: []*TreeNode{
					{ID: "child", Label: "Child"},
				},
			},
		},
		OnExpand: func(node *TreeNode) {
			expanded = append(expanded, node.ID)
		},
	})

	// Select root and expand
	tree.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if len(expanded) != 1 || expanded[0] != "root" {
		t.Fatalf("expected OnExpand called with root, got %v", expanded)
	}

	// Collapse and re-expand
	tree.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	tree.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if len(expanded) != 2 {
		t.Fatalf("expected 2 OnExpand calls, got %d", len(expanded))
	}
}

func TestTreeSelectByID(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
			{ID: "c", Label: "Gamma"},
		},
	})

	tree.SelectByID("c")
	sel := tree.Selected()
	if sel == nil || sel.ID != "c" {
		t.Fatalf("expected selected=c, got %v", sel)
	}

	tree.SelectByID("nonexistent")
	sel = tree.Selected()
	if sel == nil || sel.ID != "c" {
		t.Fatal("SelectByID with unknown ID should not change selection")
	}
}

func TestTreeMouseBoundsCheck(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "a", Label: "Alpha"},
		},
	})
	renderWidget(tree, 5, 5, 20, 10)

	// Click outside tree rect should not be handled
	outside := mouseClick(0, 0)
	if tree.HandleEvent(outside) == EventConsumed {
		t.Error("click outside rect should not be handled")
	}

	// Click inside should be handled
	inside := mouseClick(6, 5)
	if tree.HandleEvent(inside) != EventConsumed {
		t.Error("click inside rect should be handled")
	}
}

func TestTreeReload(t *testing.T) {
	child := &TreeNode{ID: "child", Label: "Child"}
	root := &TreeNode{
		ID:       "root",
		Label:    "Root",
		Children: []*TreeNode{child},
		Expanded: true,
	}
	expandCalled := 0
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{root},
		OnExpand: func(node *TreeNode) {
			expandCalled++
		},
	})

	// Initially 2 items (root expanded + child)
	if len(tree.flatList) != 2 {
		t.Fatalf("expected 2 flat items, got %d", len(tree.flatList))
	}

	tree.Reload()
	// OnExpand should be called for the root
	if expandCalled != 1 {
		t.Fatalf("expected 1 OnExpand call on reload, got %d", expandCalled)
	}
	// Expanded state should be preserved
	if !root.Expanded {
		t.Error("root should still be expanded after reload")
	}
}

func TestTreeActionClickMultipleActions(t *testing.T) {
	var commands []string
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:    "changes",
				Label: "Changes (3)",
				Actions: []Action{
					{Icon: "✕", Command: "discardAll"},
					{Icon: "+", Command: "stageAll"},
				},
			},
		},
		OnCommand: func(cmd string, node *TreeNode) {
			commands = append(commands, cmd)
		},
	})
	renderWidget(tree, 0, 0, 40, 10)

	// Render places actions right-to-left from w-2=38:
	// "+" (last action) at x=38, space at 37, "✕" at x=36
	// Click on "+" at x=38 should trigger "stageAll"
	plusClick := mouseClick(38, 0)
	tree.HandleEvent(plusClick)
	tree.HandleEvent(mouseRelease(38, 0))

	if len(commands) != 1 || commands[0] != "stageAll" {
		t.Fatalf("clicking '+' action should trigger stageAll, got %v", commands)
	}

	// Click on "✕" at x=36 should trigger "discardAll"
	commands = nil
	discardClick := mouseClick(36, 0)
	tree.HandleEvent(discardClick)
	tree.HandleEvent(mouseRelease(36, 0))

	if len(commands) != 1 || commands[0] != "discardAll" {
		t.Fatalf("clicking '✕' action should trigger discardAll, got %v", commands)
	}
}
