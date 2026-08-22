package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

// --- helpers ---

// makeTreeItems builds a simple flat list of tree nodes.
func makeTreeItems(ids ...string) []*TreeNode {
	nodes := make([]*TreeNode, len(ids))
	for i, id := range ids {
		nodes[i] = &TreeNode{ID: id, Label: id}
	}
	return nodes
}

// pressKey sends a key event to the tree widget.
func pressKey(t *TreeWidget, key tcell.Key) {
	t.HandleEvent(tcell.NewEventKey(key, "", tcell.ModNone))
}

// pressRune sends a rune event to the tree widget.
func pressRune(t *TreeWidget, r rune) {
	t.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
}

// --- Rendering tests ---

func TestTreeRenderItemPositions(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("Alpha", "Beta", "Gamma"),
	})
	tree.SetFocused(true)
	s := renderWidget(tree, 0, 0, 20, 10)

	// Each item renders on its own row: 0, 1, 2
	row0 := surfaceRowText(s, 0)
	row1 := surfaceRowText(s, 1)
	row2 := surfaceRowText(s, 2)

	if len(row0) < 5 || row0[:5] != "Alpha" {
		t.Errorf("row 0 should start with Alpha, got %q", row0)
	}
	if len(row1) < 4 || row1[:4] != "Beta" {
		t.Errorf("row 1 should start with Beta, got %q", row1)
	}
	if len(row2) < 5 || row2[:5] != "Gamma" {
		t.Errorf("row 2 should start with Gamma, got %q", row2)
	}
}

func TestTreeRenderIndentation(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:       "root",
				Label:    "Root",
				Expanded: true,
				Children: []*TreeNode{
					{ID: "child", Label: "Child"},
				},
			},
		},
		Indent: 4,
	})
	s := renderWidget(tree, 0, 0, 30, 10)

	// Root is at depth 0, so label starts at x=0 after the chevron (2 chars: chevron + space)
	// Root has children so chevron at x=0
	if s.cells[0][0].Ch != '▼' {
		t.Errorf("root should show expanded chevron, got %c", s.cells[0][0].Ch)
	}

	// Child is at depth 1 with indent=4, so it starts at x=4
	// Child has no children, so no chevron — label starts at x=4
	row1 := surfaceRowText(s, 1)
	if len(row1) < 9 || row1[4:9] != "Child" {
		t.Errorf("child should be indented to x=4, got row %q", row1)
	}
}

func TestTreeRenderDefaultIndent(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:       "root",
				Label:    "Root",
				Expanded: true,
				Children: []*TreeNode{
					{ID: "child", Label: "Child"},
				},
			},
		},
		// Indent defaults to 2
	})
	s := renderWidget(tree, 0, 0, 30, 10)

	// Child at depth 1, indent=2 => starts at x=2
	row1 := surfaceRowText(s, 1)
	if len(row1) < 7 || row1[2:7] != "Child" {
		t.Errorf("child with default indent should start at x=2, got row %q", row1)
	}
}

func TestTreeRenderNegativeIndent(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:       "root",
				Label:    "Root",
				Expanded: true,
				Children: []*TreeNode{
					{ID: "child", Label: "Child"},
				},
			},
		},
		Indent: -1, // should be clamped to 0
	})
	s := renderWidget(tree, 0, 0, 30, 10)

	// With indent=0, child at depth 1 starts at x=0*1=0
	row1 := surfaceRowText(s, 1)
	if len(row1) < 5 || row1[0:5] != "Child" {
		t.Errorf("child with indent=0 should start at x=0, got row %q", row1)
	}
}

func TestTreeRenderChevrons(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:    "collapsed",
				Label: "Collapsed",
				Children: []*TreeNode{
					{ID: "c1", Label: "C1"},
				},
			},
			{
				ID:       "expanded",
				Label:    "Expanded",
				Expanded: true,
				Children: []*TreeNode{
					{ID: "c2", Label: "C2"},
				},
			},
			{ID: "leaf", Label: "Leaf"},
		},
	})
	s := renderWidget(tree, 0, 0, 30, 10)

	// Row 0: collapsed node has right-pointing chevron
	if s.cells[0][0].Ch != '▶' {
		t.Errorf("collapsed node should show ▶, got %c", s.cells[0][0].Ch)
	}

	// Row 1: expanded node has down-pointing chevron
	if s.cells[1][0].Ch != '▼' {
		t.Errorf("expanded node should show ▼, got %c", s.cells[1][0].Ch)
	}

	// Row 3: leaf node (after expanded + its child C2 at row 2) has no chevron
	// Leaf is at flatList index 3 (collapsed, expanded, c2, leaf) but
	// collapsed is not expanded, so flatList is: collapsed, expanded, c2, leaf
	// rows: 0=collapsed, 1=expanded, 2=c2, 3=leaf
	row3 := surfaceRowText(s, 3)
	if len(row3) < 4 || row3[:4] != "Leaf" {
		t.Errorf("leaf node should have no chevron, label at x=0, got row %q", row3)
	}
}

func TestTreeRenderExpandableNoChildren(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "lazy", Label: "Lazy", Expandable: true},
		},
	})
	s := renderWidget(tree, 0, 0, 30, 10)

	// Expandable flag without children should still show a chevron
	if s.cells[0][0].Ch != '▶' {
		t.Errorf("expandable node should show ▶, got %c", s.cells[0][0].Ch)
	}
}

func TestTreeRenderEllipsisTruncation(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "long", Label: "ThisIsAVeryLongLabelThatShouldBeTruncated"},
		},
	})
	// Use a narrow width so the label can't fit
	s := renderWidget(tree, 0, 0, 12, 5)

	// maxX = w - 2 - rightSideWidth = 12 - 2 - 0 = 10
	// Label should be truncated with '...' before maxX
	row := surfaceRowText(s, 0)
	found := false
	for _, ch := range row {
		if ch == '…' {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("long label should be truncated with ellipsis, got %q", row)
	}
}

func TestTreeRenderTruncateLeft(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		TruncateLeft: true,
		Items: []*TreeNode{
			{ID: "path", Label: "internal/app/changes_panel.go"},
		},
	})
	// maxX = w - 2 = 10; the leading … plus the tail of the path must be shown.
	s := renderWidget(tree, 0, 0, 12, 5)
	row := surfaceRowText(s, 0)

	if []rune(row)[0] != '…' {
		t.Errorf("left-truncated label should start with …, got %q", row)
	}
	// The filename tail must survive; the head (internal/app) must be dropped.
	if !strings.Contains(row, "panel.go") {
		t.Errorf("left truncation should keep the tail visible, got %q", row)
	}
	if strings.Contains(row, "internal") {
		t.Errorf("left truncation should drop the head, got %q", row)
	}
}

func TestTreeRenderSelectionHighlight(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("Alpha", "Beta"),
	})
	tree.SetFocused(true)
	s := renderWidget(tree, 0, 0, 20, 10)

	// Row 0 is selected by default, should use StyleSidebarSelected when focused
	if s.cells[0][0].Style != term.StyleSidebarSelected {
		t.Errorf("selected+focused row should use StyleSidebarSelected, got %v", s.cells[0][0].Style)
	}

	// Row 1 is not selected, should use StyleDefault
	if s.cells[1][0].Style != term.StyleDefault {
		t.Errorf("unselected row should use StyleDefault, got %v", s.cells[1][0].Style)
	}
}

func TestTreeRenderSelectionNotHighlightedWhenUnfocused(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("Alpha", "Beta"),
	})
	// Not focused
	tree.SetFocused(false)
	s := renderWidget(tree, 0, 0, 20, 10)

	// Row 0 is selected but tree is not focused, so it should use StyleDefault
	if s.cells[0][0].Style != term.StyleDefault {
		t.Errorf("selected but unfocused row should use StyleDefault, got %v", s.cells[0][0].Style)
	}
}

func TestTreeRenderEmptyText(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items:     []*TreeNode{},
		EmptyText: "No items",
	})
	s := renderWidget(tree, 0, 0, 20, 5)

	// EmptyText renders at y=0, starting at x=1
	row := surfaceRowText(s, 0)
	if len(row) < 9 || row[1:9] != "No items" {
		t.Errorf("empty tree should show EmptyText, got row %q", row)
	}
}

func TestTreeRenderEmptyNoEmptyText(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{},
	})
	s := renderWidget(tree, 0, 0, 20, 5)

	// With no EmptyText and no items, row should be empty (all spaces/zeroes)
	row := surfaceRowText(s, 0)
	for _, ch := range row {
		if ch != ' ' && ch != 0 {
			t.Errorf("empty tree with no EmptyText should be blank, got %q", row)
			break
		}
	}
}

func TestTreeRenderIcon(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "a", Label: "File", Icon: "F"},
		},
	})
	s := renderWidget(tree, 0, 0, 20, 5)

	// Leaf node, no chevron. Icon "F" at x=0, space at x=1, label "File" starting at x=2
	if s.cells[0][0].Ch != 'F' {
		t.Errorf("icon should render at x=0, got %c", s.cells[0][0].Ch)
	}
	row := surfaceRowText(s, 0)
	if len(row) < 6 || row[2:6] != "File" {
		t.Errorf("label should follow icon+space, got %q", row)
	}
}

// --- Keyboard Navigation tests ---

func TestTreeKeyUpDown(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A", "B", "C"),
	})
	renderWidget(tree, 0, 0, 20, 10)

	if tree.SelectedIndex() != 0 {
		t.Fatalf("initial selection should be 0, got %d", tree.SelectedIndex())
	}

	// Down moves to next
	pressKey(tree, tcell.KeyDown)
	if tree.SelectedIndex() != 1 {
		t.Errorf("down should move to 1, got %d", tree.SelectedIndex())
	}

	pressKey(tree, tcell.KeyDown)
	if tree.SelectedIndex() != 2 {
		t.Errorf("down should move to 2, got %d", tree.SelectedIndex())
	}

	// Down at last item clamps (no wrap)
	pressKey(tree, tcell.KeyDown)
	if tree.SelectedIndex() != 2 {
		t.Errorf("down at last item should clamp at 2, got %d", tree.SelectedIndex())
	}

	// Up moves back
	pressKey(tree, tcell.KeyUp)
	if tree.SelectedIndex() != 1 {
		t.Errorf("up should move to 1, got %d", tree.SelectedIndex())
	}

	pressKey(tree, tcell.KeyUp)
	if tree.SelectedIndex() != 0 {
		t.Errorf("up should move to 0, got %d", tree.SelectedIndex())
	}

	// Up at first item clamps (no wrap)
	pressKey(tree, tcell.KeyUp)
	if tree.SelectedIndex() != 0 {
		t.Errorf("up at first item should clamp at 0, got %d", tree.SelectedIndex())
	}
}

func TestTreeJKHL(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "a", Label: "A", Expandable: true, Children: []*TreeNode{
				{ID: "a1", Label: "A1"},
			}},
			{ID: "b", Label: "B"},
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	pressRune(tree, 'j')
	if tree.SelectedIndex() != 1 {
		t.Errorf("j should move down to 1, got %d", tree.SelectedIndex())
	}
	pressRune(tree, 'k')
	if tree.SelectedIndex() != 0 {
		t.Errorf("k should move up to 0, got %d", tree.SelectedIndex())
	}
	pressRune(tree, 'l')
	if !tree.flatList[0].Expanded {
		t.Error("l should expand node A")
	}
	pressRune(tree, 'h')
	if tree.flatList[0].Expanded {
		t.Error("h should collapse node A")
	}
}

func TestTreeKeyLeftCollapse(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:       "root",
				Label:    "Root",
				Expanded: true,
				Children: []*TreeNode{
					{ID: "child", Label: "Child"},
				},
			},
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Initially root is expanded, flatList: [root, child]
	if tree.ItemCount() != 2 {
		t.Fatalf("expected 2 flat items, got %d", tree.ItemCount())
	}

	// Left on expanded node should collapse it
	pressKey(tree, tcell.KeyLeft)
	if tree.ItemCount() != 1 {
		t.Errorf("left should collapse root, expected 1 item, got %d", tree.ItemCount())
	}
	if tree.Config.Items[0].Expanded {
		t.Error("root should be collapsed after pressing left")
	}
}

func TestTreeKeyLeftAlreadyCollapsed(t *testing.T) {
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
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Root is collapsed. Left on collapsed node does nothing (no parent to go to for root).
	pressKey(tree, tcell.KeyLeft)
	if tree.SelectedIndex() != 0 {
		t.Errorf("left on collapsed root should not change selection, got %d", tree.SelectedIndex())
	}
}

func TestTreeKeyRightExpand(t *testing.T) {
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
	renderWidget(tree, 0, 0, 20, 10)

	// Root is collapsed. Right should expand it.
	pressKey(tree, tcell.KeyRight)
	if tree.ItemCount() != 2 {
		t.Errorf("right should expand root, expected 2 items, got %d", tree.ItemCount())
	}
	if !tree.Config.Items[0].Expanded {
		t.Error("root should be expanded after pressing right")
	}
	if len(expanded) != 1 || expanded[0] != "root" {
		t.Errorf("OnExpand should be called with root, got %v", expanded)
	}
}

func TestTreeKeyRightAlreadyExpanded(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:       "root",
				Label:    "Root",
				Expanded: true,
				Children: []*TreeNode{
					{ID: "child", Label: "Child"},
				},
			},
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Right on already expanded node should do nothing
	pressKey(tree, tcell.KeyRight)
	if tree.ItemCount() != 2 {
		t.Errorf("right on expanded node should not change items, got %d", tree.ItemCount())
	}
}

func TestTreeKeyEnterSelect(t *testing.T) {
	var activated []string
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "leaf", Label: "Leaf"},
		},
		OnCommand: func(cmd string, node *TreeNode) {
			activated = append(activated, cmd+":"+node.ID)
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Enter on a leaf node should call OnCommand with "activate"
	pressKey(tree, tcell.KeyEnter)
	if len(activated) != 1 || activated[0] != "activate:leaf" {
		t.Errorf("enter on leaf should trigger activate command, got %v", activated)
	}
}

func TestTreeKeyEnterToggleExpandable(t *testing.T) {
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
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Enter on expandable node should toggle expansion
	pressKey(tree, tcell.KeyEnter)
	if !tree.Config.Items[0].Expanded {
		t.Error("enter should expand the node")
	}
	if tree.ItemCount() != 2 {
		t.Errorf("expanded root should have 2 flat items, got %d", tree.ItemCount())
	}

	// Enter again should collapse
	pressKey(tree, tcell.KeyEnter)
	if tree.Config.Items[0].Expanded {
		t.Error("second enter should collapse the node")
	}
	if tree.ItemCount() != 1 {
		t.Errorf("collapsed root should have 1 flat item, got %d", tree.ItemCount())
	}
}

func TestTreeKeySpaceActivate(t *testing.T) {
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
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Space should also toggle
	pressRune(tree, ' ')
	if !tree.Config.Items[0].Expanded {
		t.Error("space should expand the node")
	}
}

func TestTreeKeyOnSelectCallback(t *testing.T) {
	var selections []string
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A", "B", "C"),
		OnSelect: func(node *TreeNode) {
			selections = append(selections, node.ID)
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Move down - should trigger OnSelect because selection changed
	pressKey(tree, tcell.KeyDown)
	if len(selections) != 1 || selections[0] != "B" {
		t.Errorf("OnSelect should be called with B, got %v", selections)
	}

	pressKey(tree, tcell.KeyDown)
	if len(selections) != 2 || selections[1] != "C" {
		t.Errorf("OnSelect should be called with C, got %v", selections)
	}

	// Down again at boundary should not call OnSelect (no change)
	pressKey(tree, tcell.KeyDown)
	if len(selections) != 2 {
		t.Errorf("OnSelect should not be called when clamped, got %d calls", len(selections))
	}
}

func TestTreeKeyOnKey(t *testing.T) {
	var captured []rune
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A"),
		OnKey: func(ev *tcell.EventKey, node *TreeNode) bool {
			captured = append(captured, term.KeyRune(ev))
			return true
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	pressRune(tree, 'x')
	if len(captured) != 1 || captured[0] != 'x' {
		t.Errorf("OnKey should capture x, got %v", captured)
	}
}

func TestTreeKeyOnKeyNotConsumed(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A"),
		OnKey: func(ev *tcell.EventKey, node *TreeNode) bool {
			return false // don't consume
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Space should fall through to ActivateSelected when OnKey returns false
	// But A is a leaf, so it fires "activate" via OnCommand
	var activated bool
	tree.Config.OnCommand = func(cmd string, node *TreeNode) {
		if cmd == "activate" {
			activated = true
		}
	}
	pressRune(tree, ' ')
	if !activated {
		t.Error("space should fall through to activate when OnKey returns false")
	}
}

// --- Mouse tests ---

func TestTreeMouseClickExpand(t *testing.T) {
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
	})
	renderWidget(tree, 0, 0, 30, 10)

	// Click on chevron at (0, 0) to expand
	click := mouseClick(0, 0)
	tree.HandleEvent(click)

	if !tree.Config.Items[0].Expanded {
		t.Error("clicking on the row should expand the node")
	}
	if tree.ItemCount() != 2 {
		t.Errorf("expanded root should have 2 flat items, got %d", tree.ItemCount())
	}

	// Click again to collapse
	tree.HandleEvent(mouseClick(0, 0))
	if tree.Config.Items[0].Expanded {
		t.Error("clicking again should collapse the node")
	}
}

func TestTreeMouseClickSelectsRow(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A", "B", "C"),
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Click on row 2 (Gamma)
	click := mouseClick(5, 2)
	tree.HandleEvent(click)

	if tree.SelectedIndex() != 2 {
		t.Errorf("click on row 2 should select index 2, got %d", tree.SelectedIndex())
	}
}

func TestTreeMouseWheelUp(t *testing.T) {
	items := make([]*TreeNode, 20)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('A' + i)), Label: string(rune('A' + i))}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	renderWidget(tree, 0, 0, 20, 5)

	// Scroll down first
	tree.scrollTop = 10

	// Wheel up should decrease scrollTop
	wheelUp := tcell.NewEventMouse(5, 2, tcell.WheelUp, tcell.ModNone)
	result := tree.HandleEvent(wheelUp)
	if result != EventConsumed {
		t.Error("wheel up should be consumed")
	}
	if tree.ScrollTop() != 7 { // 10 - 3
		t.Errorf("wheel up should decrease scrollTop by 3, got %d", tree.ScrollTop())
	}
}

func TestTreeMouseWheelDown(t *testing.T) {
	items := make([]*TreeNode, 20)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('A' + i)), Label: string(rune('A' + i))}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	renderWidget(tree, 0, 0, 20, 5)

	tree.scrollTop = 0

	// Wheel down should increase scrollTop
	wheelDown := tcell.NewEventMouse(5, 2, tcell.WheelDown, tcell.ModNone)
	result := tree.HandleEvent(wheelDown)
	if result != EventConsumed {
		t.Error("wheel down should be consumed")
	}
	if tree.ScrollTop() != 3 {
		t.Errorf("wheel down should increase scrollTop by 3, got %d", tree.ScrollTop())
	}
}

func TestTreeMouseWheelUpClampsAtZero(t *testing.T) {
	items := make([]*TreeNode, 20)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('A' + i)), Label: string(rune('A' + i))}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	renderWidget(tree, 0, 0, 20, 5)

	tree.scrollTop = 1

	wheelUp := tcell.NewEventMouse(5, 2, tcell.WheelUp, tcell.ModNone)
	tree.HandleEvent(wheelUp)
	if tree.ScrollTop() != 0 {
		t.Errorf("wheel up from scrollTop=1 should clamp to 0, got %d", tree.ScrollTop())
	}
}

func TestTreeMouseWheelDownClampsAtMax(t *testing.T) {
	items := make([]*TreeNode, 20)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('A' + i)), Label: string(rune('A' + i))}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	renderWidget(tree, 0, 0, 20, 5)

	// max = 20 - 5 = 15
	tree.scrollTop = 14

	wheelDown := tcell.NewEventMouse(5, 2, tcell.WheelDown, tcell.ModNone)
	tree.HandleEvent(wheelDown)
	if tree.ScrollTop() != 15 {
		t.Errorf("wheel down from 14 should clamp to 15, got %d", tree.ScrollTop())
	}
}

func TestTreeMouseRightClickSelectsRow(t *testing.T) {
	var menuCalled bool
	var menuNodeID string
	tree := NewTreeWidget(TreeConfig{
		Items:    makeTreeItems("A", "B", "C"),
		NodeMenu: []MenuEntry{{Label: "Delete", Command: "delete"}},
		OnMenu: func(entries []MenuEntry, node *TreeNode, screenX, screenY int) {
			menuCalled = true
			menuNodeID = node.ID
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Right click (Button2) on row 1
	rightClick := tcell.NewEventMouse(5, 1, tcell.Button2, tcell.ModNone)
	result := tree.HandleEvent(rightClick)

	if result != EventConsumed {
		t.Error("right click should be consumed")
	}
	if tree.SelectedIndex() != 1 {
		t.Errorf("right click should select row 1, got %d", tree.SelectedIndex())
	}
	if !menuCalled {
		t.Error("right click should trigger OnMenu")
	}
	if menuNodeID != "B" {
		t.Errorf("OnMenu should be called with node B, got %s", menuNodeID)
	}
}

func TestTreeMouseClickOutsideItems(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A", "B"),
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Click on row 5 (beyond item count)
	click := mouseClick(5, 5)
	result := tree.HandleEvent(click)
	if result == EventConsumed {
		t.Error("click beyond items should not be consumed")
	}
}

// --- Scrolling tests ---

func TestTreeScrollTopAdjustsOnDownNavigation(t *testing.T) {
	items := make([]*TreeNode, 20)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('A' + i)), Label: string(rune('A' + i))}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	renderWidget(tree, 0, 0, 20, 5)

	// Navigate down past the viewport
	for i := 0; i < 6; i++ {
		pressKey(tree, tcell.KeyDown)
	}
	// Render to trigger ensureVisible
	renderWidget(tree, 0, 0, 20, 5)

	if tree.SelectedIndex() != 6 {
		t.Fatalf("selection should be at 6, got %d", tree.SelectedIndex())
	}
	// scrollTop should have adjusted to keep selection visible
	// selected=6, h=5 => scrollTop should be at least 6-5+1=2
	if tree.ScrollTop() < 2 {
		t.Errorf("scrollTop should adjust to keep selection visible, got %d", tree.ScrollTop())
	}
}

func TestTreeScrollTopAdjustsOnUpNavigation(t *testing.T) {
	items := make([]*TreeNode, 20)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('A' + i)), Label: string(rune('A' + i))}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	renderWidget(tree, 0, 0, 20, 5)

	// Scroll down
	tree.SetSelectedIndex(10)
	renderWidget(tree, 0, 0, 20, 5)

	// Navigate up past the viewport
	for i := 0; i < 8; i++ {
		pressKey(tree, tcell.KeyUp)
	}
	renderWidget(tree, 0, 0, 20, 5)

	if tree.SelectedIndex() != 2 {
		t.Fatalf("selection should be at 2, got %d", tree.SelectedIndex())
	}
	// scrollTop should be <= 2
	if tree.ScrollTop() > 2 {
		t.Errorf("scrollTop should adjust to keep selection visible, got %d", tree.ScrollTop())
	}
}

func TestTreeEnsureVisible(t *testing.T) {
	items := make([]*TreeNode, 20)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('A' + i)), Label: string(rune('A' + i))}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})

	// Set selected to item 15, scrollTop=0, viewport=5
	tree.SetSelectedIndex(15)
	renderWidget(tree, 0, 0, 20, 5)

	// ensureVisible should scroll to show item 15
	// selected=15, h=5 => scrollTop should be 15-5+1=11
	if tree.ScrollTop() < 11 {
		t.Errorf("ensureVisible should scroll to show item 15, scrollTop=%d", tree.ScrollTop())
	}
}

func TestTreeScrollTopClamped(t *testing.T) {
	items := make([]*TreeNode, 3)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('A' + i)), Label: string(rune('A' + i))}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	tree.scrollTop = 100 // artificially high

	renderWidget(tree, 0, 0, 20, 10)

	// maxScroll = 3 - 10 = negative, clamped to 0
	if tree.ScrollTop() != 0 {
		t.Errorf("scrollTop should be clamped to 0 when items fit, got %d", tree.ScrollTop())
	}
}

// --- Edge case tests ---

func TestTreeEmptyTree(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{Items: []*TreeNode{}})
	renderWidget(tree, 0, 0, 20, 10)

	if tree.ItemCount() != 0 {
		t.Errorf("empty tree should have 0 items, got %d", tree.ItemCount())
	}

	sel := tree.Selected()
	if sel != nil {
		t.Error("Selected() on empty tree should return nil")
	}

	// Navigation on empty tree should not panic
	pressKey(tree, tcell.KeyDown)
	pressKey(tree, tcell.KeyUp)
	pressKey(tree, tcell.KeyLeft)
	pressKey(tree, tcell.KeyRight)
	pressKey(tree, tcell.KeyEnter)
}

func TestTreeSingleItem(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("Only"),
	})
	renderWidget(tree, 0, 0, 20, 10)

	if tree.ItemCount() != 1 {
		t.Errorf("single item tree should have 1 item, got %d", tree.ItemCount())
	}
	if tree.SelectedIndex() != 0 {
		t.Errorf("selected should be 0, got %d", tree.SelectedIndex())
	}

	// Up/Down should clamp
	pressKey(tree, tcell.KeyDown)
	if tree.SelectedIndex() != 0 {
		t.Errorf("down in single-item tree should clamp, got %d", tree.SelectedIndex())
	}
	pressKey(tree, tcell.KeyUp)
	if tree.SelectedIndex() != 0 {
		t.Errorf("up in single-item tree should clamp, got %d", tree.SelectedIndex())
	}
}

func TestTreeDeeplyNested(t *testing.T) {
	// Build a 4-level deep tree: root > L1 > L2 > L3
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:       "root",
				Label:    "Root",
				Expanded: true,
				Children: []*TreeNode{
					{
						ID:       "L1",
						Label:    "Level1",
						Expanded: true,
						Children: []*TreeNode{
							{
								ID:       "L2",
								Label:    "Level2",
								Expanded: true,
								Children: []*TreeNode{
									{ID: "L3", Label: "Level3"},
								},
							},
						},
					},
				},
			},
		},
		Indent: 2,
	})

	s := renderWidget(tree, 0, 0, 40, 10)

	// Flat list should be: root(0), L1(1), L2(2), L3(3)
	if tree.ItemCount() != 4 {
		t.Fatalf("deeply nested tree should have 4 flat items, got %d", tree.ItemCount())
	}

	// Check indentation: L3 at depth 3, indent=2 => x offset = 6
	// L3 is a leaf, so label starts at x=6
	row3 := surfaceRowText(s, 3)
	if len(row3) < 12 || row3[6:12] != "Level3" {
		t.Errorf("L3 should be indented to x=6, got row %q", row3)
	}

	// L2 at depth 2 has children, chevron at x=4 (2*2), then space, then label at x=6
	if s.cells[2][4].Ch != '▼' {
		t.Errorf("L2 should have chevron at x=4, got %c", s.cells[2][4].Ch)
	}
}

func TestTreeCollapseParentWhileChildSelected(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:       "root",
				Label:    "Root",
				Expanded: true,
				Children: []*TreeNode{
					{ID: "child1", Label: "Child1"},
					{ID: "child2", Label: "Child2"},
				},
			},
			{ID: "other", Label: "Other"},
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	// flatList: root(0), child1(1), child2(2), other(3)
	if tree.ItemCount() != 4 {
		t.Fatalf("expected 4 items, got %d", tree.ItemCount())
	}

	// Select child2 at index 2
	tree.SetSelectedIndex(2)
	if tree.Selected().ID != "child2" {
		t.Fatalf("expected child2 selected, got %s", tree.Selected().ID)
	}

	// Collapse root via SetItems (simulating parent collapse externally)
	tree.Config.Items[0].Expanded = false
	tree.SetItems(tree.Config.Items)

	// After collapse: flatList: root(0), other(1)
	if tree.ItemCount() != 2 {
		t.Fatalf("after collapse expected 2 items, got %d", tree.ItemCount())
	}

	// clampSelected should bring selection to valid range
	if tree.SelectedIndex() >= tree.ItemCount() {
		t.Errorf("selection should be clamped, got %d", tree.SelectedIndex())
	}
}

func TestTreeCollapseViaLeftKeyClampsSelection(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:       "root",
				Label:    "Root",
				Expanded: true,
				Children: []*TreeNode{
					{ID: "child1", Label: "Child1"},
					{ID: "child2", Label: "Child2"},
				},
			},
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Select root (index 0), then collapse with Left
	tree.SetSelectedIndex(0)
	pressKey(tree, tcell.KeyLeft)

	// After collapse: only root remains
	if tree.ItemCount() != 1 {
		t.Errorf("after collapse expected 1 item, got %d", tree.ItemCount())
	}
	if tree.SelectedIndex() != 0 {
		t.Errorf("selection should remain 0, got %d", tree.SelectedIndex())
	}
}

// --- FlatList and state tests ---

func TestTreeFlatten(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:    "a",
				Label: "A",
				Children: []*TreeNode{
					{ID: "a1", Label: "A1"},
					{ID: "a2", Label: "A2"},
				},
			},
			{
				ID:       "b",
				Label:    "B",
				Expanded: true,
				Children: []*TreeNode{
					{ID: "b1", Label: "B1"},
				},
			},
		},
	})

	// a is collapsed, b is expanded
	// flatList: a, b, b1
	fl := tree.FlatList()
	if len(fl) != 3 {
		t.Fatalf("expected 3 flat items, got %d", len(fl))
	}
	if fl[0].ID != "a" || fl[1].ID != "b" || fl[2].ID != "b1" {
		t.Errorf("unexpected flat order: %s, %s, %s", fl[0].ID, fl[1].ID, fl[2].ID)
	}
}

func TestTreeSetItems(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A", "B"),
	})
	tree.SetSelectedIndex(1)

	newItems := makeTreeItems("X", "Y", "Z")
	tree.SetItems(newItems)

	if tree.ItemCount() != 3 {
		t.Errorf("after SetItems expected 3 items, got %d", tree.ItemCount())
	}
	if tree.SelectedIndex() != 1 {
		t.Errorf("selection should be preserved, got %d", tree.SelectedIndex())
	}
}

func TestTreeSetItemsClampsSelection(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A", "B", "C"),
	})
	tree.SetSelectedIndex(2)

	// Replace with fewer items
	tree.SetItems(makeTreeItems("X"))

	if tree.SelectedIndex() != 0 {
		t.Errorf("selection should be clamped to 0, got %d", tree.SelectedIndex())
	}
}

func TestTreeSetSelectedIndex(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A", "B", "C"),
	})

	tree.SetSelectedIndex(2)
	if tree.SelectedIndex() != 2 {
		t.Errorf("expected selected=2, got %d", tree.SelectedIndex())
	}

	// Out of bounds should clamp
	tree.SetSelectedIndex(100)
	if tree.SelectedIndex() != 2 {
		t.Errorf("out of bounds should clamp to 2, got %d", tree.SelectedIndex())
	}

	tree.SetSelectedIndex(-5)
	if tree.SelectedIndex() != 0 {
		t.Errorf("negative should clamp to 0, got %d", tree.SelectedIndex())
	}
}

func TestTreeFocusable(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{})

	if !tree.Focusable() {
		t.Error("tree should be focusable")
	}

	tree.SetFocused(true)
	if !tree.IsFocused() {
		t.Error("tree should be focused after SetFocused(true)")
	}

	tree.SetFocused(false)
	if tree.IsFocused() {
		t.Error("tree should not be focused after SetFocused(false)")
	}
}

func TestTreeHeightWidth(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{})

	// Tree uses flexible sizing (returns 0)
	if tree.Height() != 0 {
		t.Errorf("Height should return 0, got %d", tree.Height())
	}
	if tree.Width() != 0 {
		t.Errorf("Width should return 0, got %d", tree.Width())
	}
}

// --- Collect/Restore expanded state ---

func TestTreeCollectRestoreExpanded(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:       "root",
				Label:    "Root",
				Expanded: true,
				Children: []*TreeNode{
					{
						ID:       "child",
						Label:    "Child",
						Expanded: true,
						Children: []*TreeNode{
							{ID: "grandchild", Label: "Grandchild"},
						},
					},
				},
			},
		},
	})

	expanded := map[string]bool{}
	tree.CollectExpanded(expanded)

	if !expanded["root"] || !expanded["child"] {
		t.Errorf("should collect root and child as expanded, got %v", expanded)
	}

	// Collapse everything
	tree.Config.Items[0].Expanded = false
	tree.Config.Items[0].Children[0].Expanded = false
	tree.flatten()

	// Restore
	tree.RestoreExpanded(expanded)
	if !tree.Config.Items[0].Children[0].Expanded {
		t.Error("child should be restored as expanded")
	}
}

func TestTreeExpandAndCollapseAllPreserveIdentity(t *testing.T) {
	root := &TreeNode{ID: "root", Label: "Root", Expandable: true, Expanded: true, Children: []*TreeNode{{
		ID: "child", Label: "Child", Expandable: true, Children: []*TreeNode{{ID: "leaf", Label: "Leaf"}},
	}}}
	tree := NewTreeWidget(TreeConfig{Items: []*TreeNode{root}})
	tree.SelectByID("root")
	tree.ExpandAll()
	if !root.Expanded || !root.Children[0].Expanded || tree.Selected().ID != "root" {
		t.Fatalf("expand all lost state or selection: root=%+v selected=%+v", root, tree.Selected())
	}
	tree.SelectByID("leaf")
	tree.CollapseAll()
	if root.Expanded || root.Children[0].Expanded || tree.Selected().ID != "root" {
		t.Fatalf("collapse all drifted hidden selection: root=%+v selected=%+v", root, tree.Selected())
	}
}

func TestTreeExpandAllLoadsOnlyOneLazyLevel(t *testing.T) {
	root := &TreeNode{ID: "root", Label: "Root", Expandable: true}
	loaded := []string{}
	tree := NewTreeWidget(TreeConfig{Items: []*TreeNode{root}, OnExpand: func(node *TreeNode) {
		loaded = append(loaded, node.ID)
		node.Children = []*TreeNode{{ID: node.ID + "/child", Label: "Child", Expandable: true}}
	}})
	tree.ExpandAll()
	if !root.Expanded || len(root.Children) != 1 || root.Children[0].Expanded || len(loaded) != 1 {
		t.Fatalf("lazy expand traversed beyond represented level: root=%+v loaded=%v", root, loaded)
	}
}

// --- Shortcut key tests ---

func TestTreeShortcutKey(t *testing.T) {
	var commands []string
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:    "file",
				Label: "File",
				Actions: []Action{
					{Icon: "d", Command: "delete"},
					{Icon: "r", Command: "rename"},
				},
			},
		},
		OnCommand: func(cmd string, node *TreeNode) {
			commands = append(commands, cmd)
		},
	})
	renderWidget(tree, 0, 0, 30, 10)

	// Press 'd' - should trigger delete
	pressRune(tree, 'd')
	if len(commands) != 1 || commands[0] != "delete" {
		t.Errorf("pressing 'd' should trigger delete, got %v", commands)
	}

	// Press 'r' - should trigger rename
	pressRune(tree, 'r')
	if len(commands) != 2 || commands[1] != "rename" {
		t.Errorf("pressing 'r' should trigger rename, got %v", commands)
	}

	// Press 'x' - no action, should not crash
	pressRune(tree, 'x')
	if len(commands) != 2 {
		t.Errorf("pressing 'x' should not trigger any command, got %v", commands)
	}
}

// --- Shift+Enter context menu ---

func TestTreeShiftEnterOpensMenu(t *testing.T) {
	var menuCalled bool
	tree := NewTreeWidget(TreeConfig{
		Items:    makeTreeItems("A", "B"),
		NodeMenu: []MenuEntry{{Label: "Delete", Command: "delete"}},
		OnMenu: func(entries []MenuEntry, node *TreeNode, screenX, screenY int) {
			menuCalled = true
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Shift+Enter
	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModShift)
	tree.HandleEvent(ev)

	if !menuCalled {
		t.Error("Shift+Enter should trigger OnMenu")
	}
}

// --- HandleEvent returns correct results ---

func TestTreeHandleEventReturns(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A"),
	})
	renderWidget(tree, 0, 0, 20, 10)

	// Key events should be consumed for recognized keys
	result := tree.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	if result != EventConsumed {
		t.Errorf("Up key should return EventConsumed, got %v", result)
	}

	result = tree.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if result != EventConsumed {
		t.Errorf("Down key should return EventConsumed, got %v", result)
	}

	// Unrecognized rune key without OnKey/shortcut should be ignored
	result = tree.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "z", tcell.ModNone))
	if result != EventIgnored {
		t.Errorf("unrecognized rune should return EventIgnored, got %v", result)
	}
}

// --- Mouse click on menu icon ---

func TestTreeMouseClickMenuIcon(t *testing.T) {
	var menuCalled bool
	var menuNodeID string
	tree := NewTreeWidget(TreeConfig{
		Items:    makeTreeItems("A"),
		NodeMenu: []MenuEntry{{Label: "Delete", Command: "delete"}},
		OnMenu: func(entries []MenuEntry, node *TreeNode, screenX, screenY int) {
			menuCalled = true
			menuNodeID = node.ID
		},
	})
	renderWidget(tree, 0, 0, 30, 10)

	// The menu icon zone starts at rect.X + contentW - menuIconWidth().
	// menuIconWidth for default icon "⋮" with no padding = dropdown(1 rune, 0 pad).Width()+1 = 2
	// So the zone starts at 0 + 30 - 2 = 28. Click at x=28 or x=29.
	menuW := tree.menuIconWidth()
	menuX := tree.rect.X + tree.contentW - menuW
	click := mouseClick(menuX, 0)
	tree.HandleEvent(click)

	if !menuCalled {
		t.Errorf("clicking menu icon area (x=%d, menuW=%d, contentW=%d) should trigger OnMenu", menuX, menuW, tree.contentW)
	}
	if menuNodeID != "A" {
		t.Errorf("OnMenu should be called for node A, got %s", menuNodeID)
	}
}

// --- Mouse click outside rect is ignored ---

func TestTreeMouseClickOutsideRect(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A"),
	})
	renderWidget(tree, 10, 10, 20, 5)

	// Click at (0,0) which is outside the rect at (10,10)
	result := tree.HandleEvent(mouseClick(0, 0))
	if result == EventConsumed {
		t.Error("click outside rect should be ignored")
	}

	// Click at (30, 15) which is also outside (10+20=30 is edge)
	result = tree.HandleEvent(mouseClick(30, 15))
	if result == EventConsumed {
		t.Error("click at right edge should be ignored")
	}
}

// --- Scrollbar visibility ---

func TestTreeScrollbarNotVisibleWhenItemsFit(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A", "B"),
	})
	renderWidget(tree, 0, 0, 20, 10)

	// 2 items in 10-high viewport: scrollbar should not be visible
	if tree.scrollbar.visible() {
		t.Error("scrollbar should not be visible when all items fit")
	}
	if tree.contentW != 20 {
		t.Errorf("contentW should be full width (20) when no scrollbar, got %d", tree.contentW)
	}
}

func TestTreeScrollbarVisibleWhenItemsOverflow(t *testing.T) {
	items := make([]*TreeNode, 20)
	for i := range items {
		items[i] = &TreeNode{ID: string(rune('A' + i)), Label: string(rune('A' + i))}
	}
	tree := NewTreeWidget(TreeConfig{Items: items})
	renderWidget(tree, 0, 0, 20, 5)

	// 20 items in 5-high viewport: scrollbar should be visible
	if !tree.scrollbar.visible() {
		t.Error("scrollbar should be visible when items overflow")
	}
	if tree.contentW != 19 {
		t.Errorf("contentW should be reduced by 1 for scrollbar, got %d", tree.contentW)
	}
}

// --- ActivateSelected on leaf fires OnCommand ---

func TestTreeActivateSelectedLeaf(t *testing.T) {
	var cmds []string
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{ID: "leaf", Label: "Leaf"},
		},
		OnCommand: func(cmd string, node *TreeNode) {
			cmds = append(cmds, cmd)
		},
	})

	tree.ActivateSelected()
	if len(cmds) != 1 || cmds[0] != "activate" {
		t.Errorf("ActivateSelected on leaf should fire OnCommand(activate), got %v", cmds)
	}
}

// --- ActivateSelected out of bounds ---

func TestTreeActivateSelectedOutOfBounds(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{},
	})

	// Should not panic
	tree.ActivateSelected()
}

// --- Multiple expand/collapse cycles ---

func TestTreeMultipleExpandCollapseCycles(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{
		Items: []*TreeNode{
			{
				ID:    "root",
				Label: "Root",
				Children: []*TreeNode{
					{ID: "c1", Label: "C1"},
					{ID: "c2", Label: "C2"},
				},
			},
		},
	})

	for i := 0; i < 5; i++ {
		pressKey(tree, tcell.KeyRight) // expand
		if tree.ItemCount() != 3 {
			t.Fatalf("cycle %d: expand should show 3 items, got %d", i, tree.ItemCount())
		}
		pressKey(tree, tcell.KeyLeft) // collapse
		if tree.ItemCount() != 1 {
			t.Fatalf("cycle %d: collapse should show 1 item, got %d", i, tree.ItemCount())
		}
	}
}

// --- RenderItem custom rendering ---

func TestTreeRenderItemCallback(t *testing.T) {
	var rendered []string
	tree := NewTreeWidget(TreeConfig{
		Items: makeTreeItems("A", "B"),
		RenderItem: func(surface Surface, node *TreeNode, idx, y, w int, selected bool) {
			rendered = append(rendered, node.ID)
		},
	})
	renderWidget(tree, 0, 0, 20, 10)

	if len(rendered) != 2 || rendered[0] != "A" || rendered[1] != "B" {
		t.Errorf("RenderItem should be called for each visible item, got %v", rendered)
	}
}

func TestTreeAppendItem(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{Items: makeTreeItems("a", "b")})

	tree.AppendItem(&TreeNode{ID: "c", Label: "c"})

	if got := tree.ItemCount(); got != 3 {
		t.Fatalf("ItemCount = %d, want 3", got)
	}
	flat := tree.FlatList()
	if flat[2].ID != "c" {
		t.Errorf("appended node at index 2 = %q, want %q", flat[2].ID, "c")
	}
	// The appended node must be reachable through Config.Items too, so a later
	// SetItems/flatten round-trip does not lose it.
	if len(tree.Config.Items) != 3 || tree.Config.Items[2].ID != "c" {
		t.Error("appended node missing from Config.Items")
	}
}

func TestTreeAppendItemFlattensChildren(t *testing.T) {
	tree := NewTreeWidget(TreeConfig{Items: makeTreeItems("a")})

	tree.AppendItem(&TreeNode{
		ID: "b", Label: "b", Expanded: true,
		Children: []*TreeNode{{ID: "b1", Label: "b1"}},
	})

	if got := tree.ItemCount(); got != 3 {
		t.Fatalf("ItemCount = %d, want 3 (a, b, b1)", got)
	}
	if got := tree.FlatList()[2].ID; got != "b1" {
		t.Errorf("child at index 2 = %q, want %q", got, "b1")
	}
}
