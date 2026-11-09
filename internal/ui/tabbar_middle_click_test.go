package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func tabLabelX(t *testing.T, tb *TabBarWidget, name string) int {
	t.Helper()
	grid := makeGrid(60, 3)
	tb.Render(NewRenderSurface(grid, Rect{W: 60, H: 3}))
	var row strings.Builder
	for _, c := range grid[1] {
		row.WriteRune(c.Ch)
	}
	x := strings.Index(row.String(), name)
	if x < 0 {
		t.Fatalf("tab %q not rendered", name)
	}
	return len([]rune(row.String()[:x]))
}

func newMiddleClickTabBar(closed *[]int) *TabBarWidget {
	tb := NewTabBarWidget()
	tb.OnTabClose = func(i int) { *closed = append(*closed, i) }
	tb.SetTabs([]Tab{
		{Name: "a.go", Active: true, Closable: true},
		{Name: "b.go", Closable: true},
		{Name: "c.go", Closable: true, Pinned: true},
	})
	tb.SetRect(Rect{W: 60, H: 3})
	return tb
}

func TestTabBarMiddleClickClosesInactiveTab(t *testing.T) {
	var closed []int
	tb := newMiddleClickTabBar(&closed)
	x := tabLabelX(t, tb, "b.go")
	tb.HandleEvent(tcell.NewEventMouse(x, 1, tcell.Button3, tcell.ModNone))
	tb.HandleEvent(tcell.NewEventMouse(x, 1, tcell.ButtonNone, tcell.ModNone))
	if len(closed) != 1 || closed[0] != 1 {
		t.Fatalf("closed = %v, want [1]", closed)
	}
}

func TestTabBarMiddleClickSkipsPinnedTab(t *testing.T) {
	var closed []int
	tb := newMiddleClickTabBar(&closed)
	x := tabLabelX(t, tb, "c.go")
	tb.HandleEvent(tcell.NewEventMouse(x, 1, tcell.Button3, tcell.ModNone))
	if len(closed) != 0 {
		t.Fatalf("closed = %v, want none", closed)
	}
}

func TestTabBarMiddleDragClosesOnlyFirstTab(t *testing.T) {
	var closed []int
	tb := newMiddleClickTabBar(&closed)
	xa, xb := tabLabelX(t, tb, "a.go"), tabLabelX(t, tb, "b.go")
	tb.HandleEvent(tcell.NewEventMouse(xa, 1, tcell.Button3, tcell.ModNone))
	tb.HandleEvent(tcell.NewEventMouse(xb, 1, tcell.Button3, tcell.ModNone))
	if len(closed) != 1 || closed[0] != 0 {
		t.Fatalf("closed = %v, want [0]", closed)
	}
}
