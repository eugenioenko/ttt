package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func renderContextMenu(menu *ContextMenuWidget, w, h int) [][]term.Cell {
	cells := makeGrid(w, h)
	menu.Render(NewRenderSurface(cells, Rect{X: 0, Y: 0, W: w, H: h}))
	return cells
}

func readyContextMenuForInput(menu *ContextMenuWidget) {
	menu.firstEvent = false
}

func TestContextMenuCheckedStateControlsIndicatorLayout(t *testing.T) {
	menus := map[string]*ContextMenuWidget{
		"omitted":   NewContextMenuWidget([]ContextMenuItem{{Label: "Long menu choice"}}, 0, 0),
		"unchecked": NewContextMenuWidget([]ContextMenuItem{{Label: "Long menu choice", Checked: MenuUnchecked}}, 0, 0),
		"checked":   NewContextMenuWidget([]ContextMenuItem{{Label: "Long menu choice", Checked: MenuChecked}}, 0, 0),
	}

	if got, want := menus["omitted"].menuWidth(), 22; got != want {
		t.Fatalf("omitted checked width = %d, want legacy width %d", got, want)
	}
	for _, state := range []string{"unchecked", "checked"} {
		if got, want := menus[state].menuWidth(), 24; got != want {
			t.Errorf("%s width = %d, want indicator width %d", state, got, want)
		}
	}

	checkedCells := renderContextMenu(menus["checked"], 40, 5)
	if got := checkedCells[1][1].Ch; got != '✓' {
		t.Errorf("checked indicator = %q, want checkmark", got)
	}
	uncheckedCells := renderContextMenu(menus["unchecked"], 40, 5)
	if got := uncheckedCells[1][1].Ch; got != ' ' {
		t.Errorf("unchecked indicator slot = %q, want space", got)
	}
	omittedCells := renderContextMenu(menus["omitted"], 40, 5)
	if got := strings.TrimRight(cellRow(omittedCells, 1), "."); !strings.HasPrefix(got, "│ Long menu choice") {
		t.Errorf("omitted checked row = %q, want indicator-free legacy spacing", got)
	}
}

func TestContextMenuWidthUsesTerminalColumns(t *testing.T) {
	menu := NewContextMenuWidget([]ContextMenuItem{{
		Label:    "界界界界界",
		Shortcut: "界",
	}}, 0, 0)

	if got, want := menu.menuWidth(), 20; got != want {
		t.Fatalf("menu width = %d, want %d terminal columns", got, want)
	}
}

func TestContextMenuSelectionStillSkipsSeparatorsAndExecutes(t *testing.T) {
	var commands []string
	menu := NewContextMenuWidget([]ContextMenuItem{
		MenuSep(),
		{Label: "First", Command: "first", Checked: MenuChecked},
		MenuSep(),
		{Label: "Second", Command: "second", Checked: MenuUnchecked},
	}, 4, 2)
	menu.OnExec = func(command string) { commands = append(commands, command) }

	menu.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	menu.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if len(commands) != 1 || commands[0] != "second" {
		t.Fatalf("keyboard command = %v, want [second]", commands)
	}

	renderContextMenu(menu, 40, 10)
	readyContextMenuForInput(menu)
	menu.HandleEvent(tcell.NewEventMouse(6, 4, tcell.Button1, tcell.ModNone))
	if len(commands) != 1 {
		t.Fatalf("mouse-down executed command: %v", commands)
	}
	menu.HandleEvent(tcell.NewEventMouse(6, 6, tcell.ButtonNone, tcell.ModNone))
	if len(commands) != 1 {
		t.Fatalf("release on a different item executed command: %v", commands)
	}
	menu.HandleEvent(tcell.NewEventMouse(6, 4, tcell.Button1, tcell.ModNone))
	menu.HandleEvent(tcell.NewEventMouse(6, 4, tcell.ButtonNone, tcell.ModNone))
	if len(commands) != 2 || commands[1] != "first" {
		t.Fatalf("mouse commands = %v, want second then first", commands)
	}
}

func TestContextMenuDragDoesNotRetargetPressedItem(t *testing.T) {
	var commands []string
	menu := NewContextMenuWidget([]ContextMenuItem{
		{Label: "First", Command: "first"},
		{Label: "Second", Command: "second"},
	}, 2, 1)
	menu.OnExec = func(command string) { commands = append(commands, command) }
	renderContextMenu(menu, 40, 10)
	readyContextMenuForInput(menu)

	r := menu.GetRect()
	menu.HandleEvent(tcell.NewEventMouse(r.X+2, r.Y+1, tcell.Button1, tcell.ModNone))
	menu.HandleEvent(tcell.NewEventMouse(r.X+2, r.Y+2, tcell.Button1, tcell.ModNone))
	menu.HandleEvent(tcell.NewEventMouse(r.X+2, r.Y+2, tcell.ButtonNone, tcell.ModNone))
	if len(commands) != 0 {
		t.Fatalf("dragging from First to Second executed commands: %v", commands)
	}
}

func TestContextMenuMixedFullwidthLabelsKeepExactMouseBounds(t *testing.T) {
	var commands []string
	menu := NewContextMenuWidget([]ContextMenuItem{
		{Label: "Mode 界A", Command: "first", Checked: MenuUnchecked},
		{Label: "界界 option", Command: "second", Checked: MenuChecked},
	}, 14, 1)
	menu.OnExec = func(command string) { commands = append(commands, command) }

	renderContextMenu(menu, 15, 6)
	r := menu.GetRect()
	if r.X != 0 || r.W != 15 {
		t.Fatalf("rendered bounds = %+v, want display-width-clamped 15-column menu", r)
	}

	readyContextMenuForInput(menu)
	menu.HandleEvent(tcell.NewEventMouse(r.X+r.W-1, r.Y+2, tcell.Button1, tcell.ModNone))
	if len(commands) != 0 {
		t.Fatalf("mouse-down executed command: %v", commands)
	}
	menu.HandleEvent(tcell.NewEventMouse(r.X+r.W-1, r.Y+2, tcell.ButtonNone, tcell.ModNone))
	if len(commands) != 1 || commands[0] != "second" {
		t.Fatalf("right-edge command = %v, want [second]", commands)
	}

	menu.HandleEvent(tcell.NewEventMouse(r.X+r.W, r.Y+1, tcell.Button1, tcell.ModNone))
	if len(commands) != 1 {
		t.Fatalf("outside click executed command: %v", commands)
	}
}

func TestContextMenuRenderClampsToZeroAndNarrowBounds(t *testing.T) {
	tests := []struct {
		name string
		w    int
		h    int
	}{
		{name: "zero", w: 0, h: 0},
		{name: "one-column", w: 1, h: 4},
		{name: "one-row", w: 8, h: 1},
		{name: "narrow", w: 8, h: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			menu := NewContextMenuWidget([]ContextMenuItem{{Label: "界界界界", Checked: MenuChecked}}, 20, 20)
			renderContextMenu(menu, test.w, test.h)
			r := menu.GetRect()
			if r.X < 0 || r.Y < 0 || r.W < 0 || r.H < 0 || r.X+r.W > test.w || r.Y+r.H > test.h {
				t.Fatalf("rendered bounds %+v escape %dx%d surface", r, test.w, test.h)
			}
		})
	}
}

func TestContextMenuKeyboardNavigatesSubmenu(t *testing.T) {
	var executed string
	menu := NewContextMenuWidget([]ContextMenuItem{{
		Label: "Diff View",
		Submenu: []ContextMenuItem{
			{Label: "Split", Command: "split", Checked: MenuChecked},
			{Label: "Unified", Command: "unified", Checked: MenuUnchecked},
		},
	}}, 2, 1)
	menu.OnExec = func(command string) { executed = command }
	renderContextMenu(menu, 50, 15)

	menu.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if menu.Submenu == nil {
		t.Fatal("Right should open the selected submenu")
	}
	menu.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if menu.Submenu.Selected != 1 {
		t.Fatalf("submenu selection = %d, want 1", menu.Submenu.Selected)
	}
	menu.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	if menu.Submenu != nil {
		t.Fatal("Left should return to the parent menu")
	}

	menu.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	menu.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	menu.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if executed != "unified" {
		t.Fatalf("executed %q, want unified", executed)
	}
}

func TestContextMenuFullwidthSubmenuFlipsAndKeepsMouseBounds(t *testing.T) {
	var executed string
	menu := NewContextMenuWidget([]ContextMenuItem{{
		Label: "Diff 界 View",
		Submenu: []ContextMenuItem{
			{Label: "Split 界", Command: "split", Checked: MenuUnchecked},
			{Label: "Unified", Command: "unified", Checked: MenuChecked},
		},
	}}, 31, 1)
	menu.OnExec = func(command string) { executed = command }
	readyContextMenuForInput(menu)
	cells := makeGrid(36, 15)
	surface := NewRenderSurface(cells, Rect{W: 36, H: 15})
	menu.Render(surface)

	parent := menu.GetRect()
	menu.HandleEvent(tcell.NewEventMouse(parent.X+2, parent.Y+1, tcell.ButtonNone, tcell.ModNone))
	menu.Render(surface)
	if menu.Submenu == nil {
		t.Fatal("hover should open a submenu")
	}
	child := menu.Submenu.GetRect()
	if child.X >= parent.X {
		t.Fatalf("submenu x = %d, parent x = %d; expected it to flip left", child.X, parent.X)
	}
	if child.X < 0 || child.X+child.W > 36 {
		t.Fatalf("submenu bounds %+v escape surface", child)
	}

	menu.HandleEvent(tcell.NewEventMouse(child.X+child.W-1, child.Y+2, tcell.Button1, tcell.ModNone))
	if executed != "" {
		t.Fatalf("mouse-down executed submenu command %q", executed)
	}
	menu.HandleEvent(tcell.NewEventMouse(child.X+child.W-1, child.Y+2, tcell.ButtonNone, tcell.ModNone))
	if executed != "unified" {
		t.Fatalf("right-edge submenu command = %q, want unified", executed)
	}
}

func TestContextMenuMouseReleaseOpensSubmenu(t *testing.T) {
	var executed string
	menu := NewContextMenuWidget([]ContextMenuItem{{
		Label: "Views",
		Submenu: []ContextMenuItem{
			{Label: "Split", Command: "split"},
			{Label: "Unified", Command: "unified"},
		},
	}}, 2, 1)
	readyContextMenuForInput(menu)
	menu.OnExec = func(command string) { executed = command }
	renderContextMenu(menu, 40, 10)

	parent := menu.GetRect()
	x, y := parent.X+2, parent.Y+1
	menu.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	if menu.Submenu != nil {
		t.Fatal("mouse-down opened submenu before release")
	}
	menu.HandleEvent(tcell.NewEventMouse(parent.X+parent.W+1, y, tcell.ButtonNone, tcell.ModNone))
	if menu.Submenu != nil || executed != "" {
		t.Fatal("release outside parent item activated submenu")
	}

	menu.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	menu.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
	if menu.Submenu == nil {
		t.Fatal("mouse-up on parent item did not open submenu")
	}
	renderContextMenu(menu, 40, 10)
	child := menu.Submenu.GetRect()
	childX, childY := child.X+2, child.Y+1
	menu.HandleEvent(tcell.NewEventMouse(childX, childY, tcell.Button1, tcell.ModNone))
	if executed != "" {
		t.Fatalf("mouse-down executed submenu command %q", executed)
	}
	menu.HandleEvent(tcell.NewEventMouse(childX, childY, tcell.ButtonNone, tcell.ModNone))
	if executed != "split" {
		t.Fatalf("submenu command = %q, want split", executed)
	}
}

func TestContextMenuSubmenuChevronFollowsIconMode(t *testing.T) {
	nerd := icons.Get(config.IconsNerdFont, icons.Submenu)
	cases := []struct {
		name string
		mode string
		want rune
	}{
		{name: "plain", mode: config.IconsNone, want: '›'},
		{name: "nerd font", mode: config.IconsNerdFont, want: []rune(nerd)[0]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			menu := NewContextMenuWidget([]ContextMenuItem{
				{Label: "File", Submenu: []ContextMenuItem{{Label: "New"}}},
			}, 0, 0)
			menu.Icons = tc.mode
			cells := renderContextMenu(menu, 30, 6)
			row := cells[1]
			found := false
			for _, cell := range row {
				if cell.Ch == tc.want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("icon mode %q: want chevron %q in the rendered menu", tc.mode, tc.want)
			}
		})
	}
}
