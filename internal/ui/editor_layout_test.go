package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
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
