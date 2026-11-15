package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestEditorLayoutStackedSideKeepsSharedDivider(t *testing.T) {
	for _, borderName := range []string{"rounded", "double", "ascii"} {
		for _, stackLeft := range []bool{false, true} {
			side := "right"
			if stackLeft {
				side = "left"
			}
			t.Run(borderName+"/"+side, func(t *testing.T) {
				borders := term.BorderSetByName(borderName)
				left := NewEditorGroupWidget(&borders, 4, true, "relative")
				right := NewEditorGroupWidget(&borders, 4, true, "relative")
				bottom := NewEditorGroupWidget(&borders, 4, true, "relative")
				layout := NewEditorLayoutWidget(left, &borders)
				layout.SplitGroup(left, right, SplitRight)
				stack := layout.Second
				top, wantJoin := right, borders.LeftTee
				if stackLeft {
					stack, top, wantJoin = layout.First, left, borders.RightTee
				}
				layout.SplitGroup(top, bottom, SplitBottom)
				layout.SetRect(Rect{W: 120, H: 40})
				for _, ratio := range []float64{0.5, 0.35} {
					stack.Split.BottomRatio = ratio
					cells := makeGrid(120, 40)
					layout.Render(NewRenderSurface(cells, layout.GetRect()))
					x, tabs := layout.Split.DividerScreenX(), bottom.TabBar.GetRect()
					for y := tabs.Y; y < tabs.Y+tabs.H; y++ {
						want := borders.Vertical
						if y == tabs.Y+tabs.H-1 {
							want = wantJoin
						}
						if got := cells[y][x].Ch; got != want {
							t.Fatalf("ratio %v: join at (%d, %d) = %q, want %q", ratio, x, y, got, want)
						}
					}
					if cells[0][x].Ch != ' ' || cells[1][x].Ch != ' ' || cells[2][x].Ch != borders.TopTee {
						t.Fatal("shared tab rows lost their gap or top T joint")
					}
				}
			})
		}
	}
}

func TestEditorLayoutEqualizesThreeColumns(t *testing.T) {
	first := NewEditorGroupWidget(nil, 4, true, "relative")
	second := NewEditorGroupWidget(nil, 4, true, "relative")
	third := NewEditorGroupWidget(nil, 4, true, "relative")
	layout := NewEditorLayoutWidget(first, nil)
	layout.SplitGroup(first, second, SplitRight)
	layout.SplitGroup(second, third, SplitRight)
	layout.Equalize()
	layout.SetRect(Rect{W: 122, H: 28})
	layout.Render(NewRenderSurface(makeGrid(122, 28), Rect{W: 122, H: 28}))
	for _, group := range layout.Groups() {
		if width := group.GetRect().W; width < 39 || width > 41 {
			t.Fatalf("equalized column = %d, want about 40", width)
		}
	}
	layout.Split.BottomRatio = 0.7
	layout.SetRect(Rect{W: 202, H: 28})
	layout.Render(NewRenderSurface(makeGrid(202, 28), Rect{W: 202, H: 28}))
	if width := first.GetRect().W; width < 59 || width > 61 {
		t.Fatalf("resizing window lost divider ratio: first width %d", width)
	}
	if !layout.RemoveGroup(second) || len(layout.Groups()) != 2 || layout.leaf(second) != nil {
		t.Fatal("removing a nested group did not collapse its split")
	}
}

func TestEditorLayoutAlignedLowerTabsKeepDivider(t *testing.T) {
	for _, borderName := range []string{"rounded", "double", "ascii"} {
		for _, style := range []string{"extended", "hidden"} {
			t.Run(borderName+"/"+style, func(t *testing.T) {
				borders := term.BorderSetByName(borderName)
				groups := make([]*EditorGroupWidget, 4)
				for i := range groups {
					groups[i] = NewEditorGroupWidget(&borders, 4, true, "relative")
					groups[i].TabBar.SetStyle(style)
				}
				layout := NewEditorLayoutWidget(groups[0], &borders)
				layout.SplitGroup(groups[0], groups[1], SplitRight)
				layout.SplitGroup(groups[0], groups[2], SplitBottom)
				layout.SplitGroup(groups[1], groups[3], SplitBottom)
				layout.SetRect(Rect{W: 120, H: 40})
				for _, ratio := range []float64{0.5, 0.35} {
					layout.First.Split.BottomRatio, layout.Second.Split.BottomRatio = ratio, ratio
					cells := makeGrid(120, 40)
					layout.Render(NewRenderSurface(cells, layout.GetRect()))
					x, tabs := layout.Split.DividerScreenX(), groups[2].TabBar.GetRect()
					for y := tabs.Y; y < tabs.Y+tabs.H; y++ {
						want := borders.Vertical
						if y == tabs.Y+tabs.H-1 {
							want = borders.Cross
						}
						if got := cells[y][x].Ch; got != want {
							t.Fatalf("ratio %v: divider at (%d, %d) = %q, want %q", ratio, x, y, got, want)
						}
						if got := layout.PointerShapeAt(x, y); got != term.PointerEWResize {
							t.Fatalf("divider at (%d, %d) has pointer %q", x, y, got)
						}
					}
				}
			})
		}
	}
}

func TestEditorLayoutHorizontalResizeWithMixedTabHeights(t *testing.T) {
	top := NewEditorGroupWidget(nil, 4, true, "relative")
	left := NewEditorGroupWidget(nil, 4, true, "relative")
	right := NewEditorGroupWidget(nil, 4, true, "relative")
	left.TabBar.SetStyle("hidden")
	layout := NewEditorLayoutWidget(top, nil)
	layout.SplitGroup(top, left, SplitBottom)
	layout.SplitGroup(left, right, SplitRight)
	layout.SetRect(Rect{W: 120, H: 44})
	render := func() { layout.Render(NewRenderSurface(makeGrid(120, 44), layout.GetRect())) }
	render()
	for _, group := range []*EditorGroupWidget{left, right} {
		tabs := group.TabBar.GetRect()
		x, y := tabs.X+tabs.W-6, tabs.Y+tabs.H-1
		if layout.PointerShapeAt(x, y) != term.PointerNSResize {
			t.Fatal("mixed-height header did not expose its visible resize line")
		}
		layout.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
		layout.HandleEvent(tcell.NewEventMouse(x, y+2, tcell.Button1, 0))
		render()
		tabs = group.TabBar.GetRect()
		if got := tabs.Y + tabs.H - 1; got != y+2 {
			t.Fatalf("visible border moved to row %d, want %d", got, y+2)
		}
		layout.HandleEvent(tcell.NewEventMouse(x, y+2, tcell.ButtonNone, 0))
	}
}
