package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/icons"
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

func TestTabBarStyleControlsFollowLabels(t *testing.T) {
	for _, style := range []string{"extended"} {
		t.Run(style, func(t *testing.T) {
			bar := NewTabBarWidget()
			bar.SetStyle(style)
			bar.SetTabs([]Tab{{Name: "first", Active: true, Closable: true}, {Name: "second"}})
			bar.SetRect(Rect{W: 80, H: bar.Height()})
			bar.Render(NewRenderSurface(makeGrid(80, bar.Height()), bar.GetRect()))
			clicked, closed := -1, -1
			bar.OnTabClick = func(i int) { clicked = i }
			bar.OnTabClose = func(i int) { closed = i }
			y := 1
			x := bar.tabSpans[1].start + 2
			bar.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
			bar.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, 0))
			if clicked != 1 {
				t.Fatal("tab label did not switch tabs")
			}
			closeX := -1
			for x, cell := range makeTabStyleGrid(bar)[y] {
				if cell.Ch == icons.Rune(icons.Close) {
					closeX = x
				}
			}
			if closeX < 0 {
				t.Fatal("close button was not drawn")
			}
			bar.HandleEvent(tcell.NewEventMouse(closeX, y, tcell.Button1, 0))
			bar.HandleEvent(tcell.NewEventMouse(closeX, y, tcell.ButtonNone, 0))
			if closed != 0 {
				t.Fatal("close button hit target did not follow its glyph")
			}
		})
	}
}

func makeTabStyleGrid(bar *TabBarWidget) [][]term.Cell {
	grid := makeGrid(80, bar.Height())
	bar.Render(NewRenderSurface(grid, bar.GetRect()))
	return grid
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

func TestTabBarStylesMoveOuterFrame(t *testing.T) {
	b := term.SingleBorderSet()
	for _, tc := range []struct {
		style string
		row   int
		glyph rune
	}{
		{"extended", 2, b.TopRight},
		{"hidden", 0, b.TopRight}, {"auto-hide", 0, b.TopRight},
	} {
		t.Run(tc.style, func(t *testing.T) {
			group := NewEditorGroupWidget(nil, 4, false, "minimal")
			group.TabBar.SetStyle(tc.style)
			content := NewContentSplitWidget()
			content.Top = NewEditorLayoutWidget(group, nil)
			panel := NewSplitPanelWidget()
			panel.ShowLeft = false
			panel.Right = content
			panel.SetRect(Rect{W: 80, H: 20})
			grid := makeGrid(80, 20)
			panel.Render(NewRenderSurface(grid, panel.GetRect()))
			if got := grid[tc.row][79].Ch; got != tc.glyph {
				t.Fatalf("outer border at row %d = %q, want %q", tc.row, got, tc.glyph)
			}
			for y := tc.row + 1; y < 19; y++ {
				if grid[y][79].Ch != b.Vertical {
					t.Fatalf("floating corner or missing border at row %d", y)
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
