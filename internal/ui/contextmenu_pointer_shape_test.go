package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestContextMenuPointerShapeActionableAndDefaultRows(t *testing.T) {
	executed := false
	menu := NewContextMenuWidget([]ContextMenuItem{
		{Label: "Save", Shortcut: "Ctrl+S", Command: "file.save"},
		MenuSep(),
		{Label: "Submenu", Submenu: []ContextMenuItem{{Label: "Child", Command: "child"}}},
		{Label: "Disabled/Info"}, // No command, no submenu -> non-actionable
	}, 10, 5)

	menu.OnExec = func(cmd string) {
		executed = true
	}

	// Render in 60x20 screen
	renderContextMenu(menu, 60, 20)
	r := menu.GetRect()

	// 1. Command row (row 0, y = r.Y + 1)
	for x := r.X + 1; x < r.X+r.W-1; x++ {
		if got := menu.PointerShapeAt(x, r.Y+1); got != term.PointerHand {
			t.Errorf("command row cell (%d, %d) = %q, want %q", x, r.Y+1, got, term.PointerHand)
		}
	}

	// 2. Separator row (row 1, y = r.Y + 2)
	for x := r.X + 1; x < r.X+r.W-1; x++ {
		if got := menu.PointerShapeAt(x, r.Y+2); got != term.PointerDefault {
			t.Errorf("separator row cell (%d, %d) = %q, want %q", x, r.Y+2, got, term.PointerDefault)
		}
	}

	// 3. Submenu-opening row (row 2, y = r.Y + 3)
	for x := r.X + 1; x < r.X+r.W-1; x++ {
		if got := menu.PointerShapeAt(x, r.Y+3); got != term.PointerHand {
			t.Errorf("submenu-opening row cell (%d, %d) = %q, want %q", x, r.Y+3, got, term.PointerHand)
		}
	}

	// 4. Non-actionable row (row 3, y = r.Y + 4)
	for x := r.X + 1; x < r.X+r.W-1; x++ {
		if got := menu.PointerShapeAt(x, r.Y+4); got != term.PointerDefault {
			t.Errorf("non-actionable row cell (%d, %d) = %q, want %q", x, r.Y+4, got, term.PointerDefault)
		}
	}

	// 5. Borders
	borderPoints := [][2]int{
		{r.X, r.Y},                     // Top-left corner
		{r.X + 5, r.Y},                 // Top border
		{r.X + r.W - 1, r.Y},           // Top-right corner
		{r.X, r.Y + 2},                 // Left border
		{r.X + r.W - 1, r.Y + 2},       // Right border
		{r.X, r.Y + r.H - 1},           // Bottom-left corner
		{r.X + 5, r.Y + r.H - 1},       // Bottom border
		{r.X + r.W - 1, r.Y + r.H - 1}, // Bottom-right corner
	}
	for _, pt := range borderPoints {
		if got := menu.PointerShapeAt(pt[0], pt[1]); got != term.PointerDefault {
			t.Errorf("border point (%d, %d) = %q, want %q", pt[0], pt[1], got, term.PointerDefault)
		}
	}

	// 6. Outside points
	outsidePoints := [][2]int{
		{r.X - 1, r.Y},
		{r.X + r.W, r.Y},
		{r.X, r.Y - 1},
		{r.X, r.Y + r.H},
		{0, 0},
		{59, 19},
	}
	for _, pt := range outsidePoints {
		if got := menu.PointerShapeAt(pt[0], pt[1]); got != term.PointerDefault {
			t.Errorf("outside point (%d, %d) = %q, want %q", pt[0], pt[1], got, term.PointerDefault)
		}
	}

	// Verify queries do not execute commands or change selection
	if executed {
		t.Fatal("PointerShapeAt triggered command execution")
	}
}

func TestContextMenuPointerShapeNestedSubmenuAndClose(t *testing.T) {
	menu := NewContextMenuWidget([]ContextMenuItem{
		{Label: "Item 1", Command: "cmd1"},
		{
			Label: "Nested",
			Submenu: []ContextMenuItem{
				{Label: "Sub 1", Command: "sub1"},
				MenuSep(),
				{Label: "Sub 2", Command: "sub2"},
			},
		},
	}, 5, 2)

	renderContextMenu(menu, 80, 25)

	// Before opening submenu, submenu coordinates are outside/default
	// Open submenu at index 1
	menu.Selected = 1
	if !menu.openSelectedSubmenu() {
		t.Fatal("failed to open submenu")
	}

	// Re-render to lay out the submenu
	renderContextMenu(menu, 80, 25)

	sub := menu.Submenu
	if sub == nil {
		t.Fatal("expected menu.Submenu to be non-nil")
	}
	subR := sub.GetRect()

	// Actionable item in submenu
	if got := menu.PointerShapeAt(subR.X+2, subR.Y+1); got != term.PointerHand {
		t.Errorf("submenu item 'Sub 1' = %q, want %q", got, term.PointerHand)
	}

	// Separator in submenu
	if got := menu.PointerShapeAt(subR.X+2, subR.Y+2); got != term.PointerDefault {
		t.Errorf("submenu separator = %q, want %q", got, term.PointerDefault)
	}

	// Another actionable item in submenu
	if got := menu.PointerShapeAt(subR.X+2, subR.Y+3); got != term.PointerHand {
		t.Errorf("submenu item 'Sub 2' = %q, want %q", got, term.PointerHand)
	}

	// Parent menu item "Nested" still reports hand
	parentR := menu.GetRect()
	if got := menu.PointerShapeAt(parentR.X+2, parentR.Y+2); got != term.PointerHand {
		t.Errorf("parent submenu-opening item = %q, want %q", got, term.PointerHand)
	}

	// Close the submenu
	menu.Submenu = nil
	renderContextMenu(menu, 80, 25)

	// Coordinates where submenu used to be should no longer report hand pointer
	if got := menu.PointerShapeAt(subR.X+2, subR.Y+1); got != term.PointerDefault {
		t.Errorf("after closing submenu, point in old submenu area = %q, want %q", got, term.PointerDefault)
	}
}

func TestContextMenuPointerShapeLeftOpeningRepositionedMenu(t *testing.T) {
	// Position parent menu near right edge of 40-column screen so submenu must open to the left
	menu := NewContextMenuWidget([]ContextMenuItem{
		{
			Label: "Submenu Trigger",
			Submenu: []ContextMenuItem{
				{Label: "Left Child", Command: "left.child"},
			},
		},
	}, 25, 2)

	renderContextMenu(menu, 40, 20)
	menu.Selected = 0
	menu.openSelectedSubmenu()
	renderContextMenu(menu, 40, 20)

	sub := menu.Submenu
	if sub == nil {
		t.Fatal("expected submenu to open")
	}
	subR := sub.GetRect()
	parentR := menu.GetRect()

	// Verify submenu opened to the left of parent
	if subR.X >= parentR.X {
		t.Errorf("expected submenu on left (subR.X=%d < parentR.X=%d)", subR.X, parentR.X)
	}

	// Actionable item inside left-opening submenu reports hand pointer
	if got := menu.PointerShapeAt(subR.X+2, subR.Y+1); got != term.PointerHand {
		t.Errorf("left-opening submenu item = %q, want %q", got, term.PointerHand)
	}
}

func TestContextMenuPointerShapeScreenEdgeClipping(t *testing.T) {
	items := make([]ContextMenuItem, 10)
	for i := range items {
		items[i] = ContextMenuItem{Label: "Item", Command: "cmd"}
	}
	menu := NewContextMenuWidget(items, 5, 5)

	// Screen height is only 6 rows; menu requires 12 rows, so it clips to 6
	renderContextMenu(menu, 40, 6)
	r := menu.GetRect()

	if r.H > 6 {
		t.Fatalf("expected menu height to be clipped <= 6, got %d", r.H)
	}

	// Visible content rows (y within r.Y+1 .. r.Y+r.H-2)
	for row := r.Y + 1; row < r.Y+r.H-1; row++ {
		if got := menu.PointerShapeAt(r.X+2, row); got != term.PointerHand {
			t.Errorf("visible row y=%d = %q, want %q", row, got, term.PointerHand)
		}
	}

	// Rows clipped away past the bottom border of the menu
	clippedY := r.Y + r.H
	if got := menu.PointerShapeAt(r.X+2, clippedY); got != term.PointerDefault {
		t.Errorf("clipped-away row y=%d = %q, want %q", clippedY, got, term.PointerDefault)
	}
}

func TestContextMenuPointerShapeStateImmutability(t *testing.T) {
	menu := NewContextMenuWidget([]ContextMenuItem{
		{Label: "Item 1", Command: "cmd1"},
		{Label: "Item 2", Command: "cmd2"},
	}, 0, 0)
	menu.Selected = 0
	renderContextMenu(menu, 40, 10)

	// Query across various points
	for x := -5; x < 50; x++ {
		for y := -5; y < 15; y++ {
			_ = menu.PointerShapeAt(x, y)
		}
	}

	// Verify Selected index was not mutated
	if menu.Selected != 0 {
		t.Errorf("menu.Selected changed to %d, want 0", menu.Selected)
	}
	// Verify no submenus were accidentally created
	if menu.Submenu != nil {
		t.Error("menu.Submenu was unexpectedly created by query")
	}
}

func TestContextMenuPointerShapeComposedRootOverlay(t *testing.T) {
	// Underlying editor widget that normally returns PointerText
	editor := &shapeWidget{shape: term.PointerText}
	editor.SetRect(widgets.Rect{X: 0, Y: 0, W: 80, H: 25})

	root := NewRoot(editor)

	// Without overlay, editor reports PointerText
	if got := root.PointerShapeAt(10, 5); got != term.PointerText {
		t.Fatalf("underlying editor shape = %q, want %q", got, term.PointerText)
	}

	// Push open menu overlay
	menu := NewContextMenuWidget([]ContextMenuItem{
		{Label: "Format Document", Command: "editor.format"},
		MenuSep(),
		{Label: "Find References", Command: "editor.references"},
	}, 10, 5)

	renderContextMenu(menu, 80, 25)
	root.PushOverlay(Overlay{Widget: menu, Modal: true})

	r := menu.GetRect()

	// 1. Over actionable item inside menu -> PointerHand
	if got := root.PointerShapeAt(r.X+2, r.Y+1); got != term.PointerHand {
		t.Errorf("over menu item through Root = %q, want %q", got, term.PointerHand)
	}

	// 2. Over separator inside menu -> PointerDefault (NOT PointerText from editor beneath!)
	if got := root.PointerShapeAt(r.X+2, r.Y+2); got != term.PointerDefault {
		t.Errorf("over menu separator through Root = %q, want %q", got, term.PointerDefault)
	}

	// 3. Over border of menu -> PointerDefault (NOT PointerText from editor beneath!)
	if got := root.PointerShapeAt(r.X, r.Y); got != term.PointerDefault {
		t.Errorf("over menu border through Root = %q, want %q", got, term.PointerDefault)
	}

	// 4. Outside menu -> PointerDefault (modal overlay blocks editor text shape!)
	if got := root.PointerShapeAt(50, 15); got != term.PointerDefault {
		t.Errorf("outside menu through Root = %q, want %q", got, term.PointerDefault)
	}
}
