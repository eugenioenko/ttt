package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestContextMenuKeyboardNavigatesSubmenu(t *testing.T) {
	var executed string
	menu := NewContextMenuWidget([]ContextMenuItem{
		{Label: "Git Files", Submenu: []ContextMenuItem{
			{Label: "Tree", Command: "tree"},
			{Label: "List", Command: "list"},
		}},
	}, 2, 1)
	menu.OnExec = func(command string) { executed = command }
	menu.Render(NewRenderSurface(makeGrid(50, 15), Rect{W: 50, H: 15}))

	menu.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", 0))
	if menu.Submenu == nil {
		t.Fatal("Right should open the selected submenu")
	}
	menu.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", 0))
	if menu.Submenu.Selected != 1 {
		t.Fatalf("submenu selection = %d, want 1", menu.Submenu.Selected)
	}
	menu.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", 0))
	if menu.Submenu != nil {
		t.Fatal("Left should return to the parent menu")
	}

	menu.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	menu.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", 0))
	menu.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	if executed != "list" {
		t.Fatalf("executed %q, want list", executed)
	}
}

func TestContextMenuHoverOpensSubmenuAndFlipsAtRightEdge(t *testing.T) {
	var executed string
	menu := NewContextMenuWidget([]ContextMenuItem{
		{Label: "Diff View", Submenu: []ContextMenuItem{
			{Label: "Split", Command: "split"},
			{Label: "Unified", Command: "unified"},
		}},
	}, 31, 1)
	menu.OnExec = func(command string) { executed = command }
	menu.firstEvent = false
	surface := NewRenderSurface(makeGrid(36, 15), Rect{W: 36, H: 15})
	menu.Render(surface)

	r := menu.GetRect()
	menu.HandleEvent(tcell.NewEventMouse(r.X+2, r.Y+1, tcell.ButtonNone, 0))
	if menu.Submenu == nil {
		t.Fatal("hover should open a submenu")
	}
	menu.Render(surface)
	childRect := menu.Submenu.GetRect()
	if childRect.X >= r.X {
		t.Fatalf("submenu x = %d, parent x = %d; expected it to flip left", childRect.X, r.X)
	}

	menu.HandleEvent(tcell.NewEventMouse(childRect.X+2, childRect.Y+2, tcell.Button1, 0))
	if executed != "unified" {
		t.Fatalf("executed %q, want unified", executed)
	}
}

func TestContextMenuWidthUsesDisplayColumns(t *testing.T) {
	menu := NewContextMenuWidget([]ContextMenuItem{{Label: "界界界界界界界界界界界界"}}, 0, 0)
	if got := menu.menuWidth(); got < 30 {
		t.Fatalf("menu width = %d, want at least 30 display columns", got)
	}
}
