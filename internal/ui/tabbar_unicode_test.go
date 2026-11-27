package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
	"golang.org/x/text/unicode/norm"
)

func TestTabBarUnicodeFilenameCellsAndCloseHit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cells []string
	}{
		{"example.txt", []string{"e", "x", "a", "m", "p", "l", "e", ".", "t", "x", "t"}},
		{"café.txt", []string{"c", "a", "f", "é", ".", "t", "x", "t"}},
		{"cafe\u0301.txt", []string{"c", "a", "f", "é", ".", "t", "x", "t"}},
		{"e\u0301lastique.txt", []string{"é", "l", "a", "s", "t", "i", "q", "u", "e", ".", "t", "x", "t"}},
		{"e\u0301te\u0301.txt", []string{"é", "t", "é", ".", "t", "x", "t"}},
		{"q\u0301.txt", []string{"q\u0301", ".", "t", "x", "t"}},
		{"日本.txt", []string{"日", " ", "本", " ", ".", "t", "x", "t"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := NewTabBarWidget()
			path := "/repo/" + tc.name
			tb.SetTabs([]Tab{
				{Name: tc.name, FilePath: path, Active: true, Closable: true},
				{Name: "next.txt"},
			})
			tb.SetRect(Rect{W: 40, H: 3})
			grid := makeGrid(40, 3)
			tb.Render(NewRenderSurface(grid, Rect{W: 40, H: 3}))
			sim := term.NewSimScreen()
			sim.SetSize(40, 3)
			screen := term.NewTcellScreenFrom(sim)
			for x, cell := range grid[1] {
				screen.SetCell(x, 1, cell)
			}
			for i, want := range tc.cells {
				got, _, _ := sim.Get(2+i, 1)
				if norm.NFC.String(got) != want {
					t.Errorf("filename column %d = %q, want %q", i, got, want)
				}
			}
			if got, want := tb.tabSpans[0].end, len(tc.cells)+6; got != want {
				t.Fatalf("tab end = %d, want %d", got, want)
			}
			if got, _, _ := sim.Get(len(tc.cells)+7, 1); got != "n" {
				t.Fatalf("following tab is misaligned: %q", got)
			}
			closeX := len(tc.cells) + 3
			if grid[1][closeX].Ch != icons.Rune(icons.Close) {
				t.Fatal("close button is misaligned")
			}
			closed := false
			tb.OnTabClose = func(index int) { closed = index == 0 }
			tb.HandleEvent(tcell.NewEventMouse(closeX, 1, tcell.Button1, 0))
			tb.HandleEvent(tcell.NewEventMouse(closeX, 1, tcell.ButtonNone, 0))
			if !closed {
				t.Fatal("clicking the rendered close button did not close the tab")
			}
			if tb.Tabs[0].Name != tc.name || tb.Tabs[0].FilePath != path {
				t.Fatal("rendering changed the original filename or path")
			}
		})
	}
}

func TestDrawTabLabelClipsCompleteGraphemes(t *testing.T) {
	for _, tc := range []struct {
		left, right int
		want        map[int]string
	}{
		{1, 5, map[int]string{1: "e\u0301", 2: "日", 4: "q\u0301"}},
		{1, 3, map[int]string{1: "e\u0301"}},
		{3, 6, map[int]string{4: "q\u0301", 5: "z"}},
	} {
		grid := makeGrid(8, 3)
		drawTabLabel(NewRenderSurface(grid, Rect{W: 8, H: 3}), 0, tc.left, tc.right,
			tabLabelLayout{text: "ae\u0301日q\u0301z"}, "", term.StyleActiveTab)
		for x, cell := range grid[1] {
			got := string(cell.Ch)
			if cell.Text != "" {
				got = cell.Text
			}
			want, drawn := tc.want[x]
			if !drawn {
				want = "."
			}
			if got != want {
				t.Errorf("clip [%d,%d) column %d = %q, want %q", tc.left, tc.right, x, got, want)
			}
		}
	}
}
