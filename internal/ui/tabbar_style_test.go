package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestEditorTabBarStylesReclaimRowsAndKeepContent(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "minimal")
	g.NewFileFromText("editor content")
	g.SetRect(Rect{W: 80, H: 20})
	for _, tc := range []struct {
		style  string
		height int
	}{
		{"extended", 3}, {"hidden", 1}, {"auto-hide", 1}, {"extended", 3},
	} {
		t.Run(tc.style, func(t *testing.T) {
			g.TabBar.SetStyle(tc.style)
			grid := makeGrid(80, 20)
			g.Render(NewRenderSurface(grid, g.GetRect()))
			if r := g.Editor.GetRect(); r.Y != tc.height || r.H != 20-tc.height {
				t.Fatalf("editor rect = %+v", r)
			}
			if !strings.Contains(gridRow(grid, tc.height, 80), "editor content") {
				t.Fatal("editor content was clipped or covered")
			}
			if tc.style == "hidden" || tc.style == "auto-hide" {
				if len(g.TabBar.tabSpans) != 0 || g.TabBar.MoreButton.GetRect().H != 0 {
					t.Fatal("hidden tabs kept clickable geometry")
				}
				if got := g.TabBar.HandleEvent(tcell.NewEventMouse(2, 0, tcell.WheelUp, 0)); got != EventIgnored {
					t.Fatal("hidden tabs consumed an editor event")
				}
				if g.TabBar.PointerShapeAt(2, 0) != "" {
					t.Fatal("hidden tabs kept a pointer target")
				}
			} else if !strings.Contains(gridRow(grid, 1, 80), "untitled") {
				t.Fatal("tab names not drawn on the label row")
			}
			if tc.style == "hidden" && grid[0][20].Ch != term.SingleBorderSet().Horizontal {
				t.Fatal("hidden style lost its divider")
			}
		})
	}
}

func TestEditorTabBarStyleSplitJunctions(t *testing.T) {
	for _, style := range []string{"extended", "hidden", "auto-hide"} {
		t.Run(style, func(t *testing.T) {
			left := NewEditorGroupWidget(nil, 4, false, "minimal")
			right := NewEditorGroupWidget(nil, 4, false, "minimal")
			bottom := NewEditorGroupWidget(nil, 4, false, "minimal")
			layout := NewEditorLayoutWidget(left, nil)
			layout.SplitGroup(left, right, SplitRight)
			layout.SplitGroup(right, bottom, SplitBottom)
			for _, g := range layout.Groups() {
				g.TabBar.SetStyle(style)
			}
			layout.SetRect(Rect{W: 120, H: 40})
			grid := makeGrid(120, 40)
			layout.Render(NewRenderSurface(grid, layout.GetRect()))
			x := layout.Split.DividerScreenX()
			b := term.SingleBorderSet()
			for y := bottom.GetRect().Y; y < 40; y++ {
				want := b.Vertical
				if baseline, ok := bottom.TabBar.baselineY(); ok && y == baseline {
					want = b.LeftTee
				}
				if grid[y][x].Ch != want {
					t.Fatalf("divider row %d = %q, want %q", y, grid[y][x].Ch, want)
				}
			}
		})
	}
}

func TestHiddenTabBarConnectsToSidebar(t *testing.T) {
	for _, style := range []string{"rounded", "double", "ascii"} {
		t.Run(style, func(t *testing.T) {
			borders := term.BorderSetByName(style)
			group := NewEditorGroupWidget(&borders, 4, false, "minimal")
			group.TabBar.SetStyle("hidden")
			content := NewContentSplitWidget()
			content.Top = NewEditorLayoutWidget(group, &borders)
			panel := NewSplitPanelWidget()
			panel.Borders = &borders
			panel.Right = content
			panel.SetRect(Rect{W: 80, H: 20})
			grid := makeGrid(80, 20)
			panel.Render(NewRenderSurface(grid, panel.GetRect()))
			x := panel.DividerScreenX()
			if got := grid[0][x].Ch; got != borders.TopTee {
				t.Fatalf("sidebar join = %q, want %q", got, borders.TopTee)
			}
			if grid[0][x-1].Ch != borders.Horizontal || grid[0][x+1].Ch != borders.Horizontal || grid[1][x].Ch != borders.Vertical {
				t.Fatal("T joint does not connect the sidebar and editor borders")
			}
		})
	}
}
