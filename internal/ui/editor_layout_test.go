package ui

import "testing"

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
