package ui

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestVerticalTabBarShowsALabelBadge(t *testing.T) {
	v := NewVerticalTabBar()
	v.Count = 2
	v.Active = 0
	v.Labels = []string{"pi", ""}
	v.SetRect(Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 4})

	grid := makeGrid(VerticalTabBarWidth, 4)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 4})
	v.Render(surface)

	// row 0: the labelled tab shows [pi] instead of the number
	for x, want := range "[pi]" {
		if grid[0][x].Ch != want {
			t.Fatalf("row 0 col %d = %q, want %q", x, grid[0][x].Ch, want)
		}
	}
	if grid[0][0].Style != term.StyleActiveTab {
		t.Fatal("the active labelled tab must keep the active style")
	}
	// row 1: an unlabelled tab keeps the numbered badge
	for x, want := range "[>2]" {
		if grid[1][x].Ch != want {
			t.Fatalf("row 1 col %d = %q, want %q", x, grid[1][x].Ch, want)
		}
	}
}

func TestVerticalTabBarTruncatesLongLabels(t *testing.T) {
	v := NewVerticalTabBar()
	v.Count = 1
	v.Labels = []string{"planner"}
	v.SetRect(Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 2})

	grid := makeGrid(VerticalTabBarWidth, 2)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 2})
	v.Render(surface)

	// the strip is 4 content cells: a long label is clipped, never spills over
	// the border at x=4
	for x, want := range "[pl]" {
		if grid[0][x].Ch != want {
			t.Fatalf("col %d = %q, want %q", x, grid[0][x].Ch, want)
		}
	}
	if grid[0][4].Ch != '│' || grid[0][4].Style != term.StyleBorder {
		t.Fatalf("border cell = %+v, want vertical border", grid[0][4])
	}
}

func TestVerticalTabBarRendersAccentedAndFullwidthLabelsByDisplayWidth(t *testing.T) {
	v := NewVerticalTabBar()
	v.Count = 5
	v.Active = 1
	v.Labels = []string{"élan", "日本", "a日", "é", ""}
	v.SetRect(Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 6})

	grid := makeGrid(VerticalTabBarWidth, 6)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 6})
	v.Render(surface)

	// row 0: multibyte narrow runes advance one column per rune and truncate to 2 columns -> [él]
	wantRow0 := []rune{'[', 'é', 'l', ']'}
	for x, want := range wantRow0 {
		if grid[0][x].Ch != want {
			t.Fatalf("row 0 col %d = %q, want %q", x, grid[0][x].Ch, want)
		}
		if grid[0][x].Style != term.StyleInactiveTab {
			t.Fatalf("row 0 col %d style = %v, want StyleInactiveTab", x, grid[0][x].Style)
		}
	}

	// row 1: fullwidth label truncates to 2 display columns ("日") and advances 2 columns before ']'
	if grid[1][0].Ch != '[' || grid[1][1].Ch != '日' || grid[1][2].Ch != ' ' || grid[1][3].Ch != ']' {
		t.Fatalf("row 1 cells = [%q %q %q %q], want ['[' '日' ' ' ']']",
			grid[1][0].Ch, grid[1][1].Ch, grid[1][2].Ch, grid[1][3].Ch)
	}
	for _, x := range []int{0, 1, 3} {
		if grid[1][x].Style != term.StyleActiveTab {
			t.Fatalf("row 1 col %d style = %v, want StyleActiveTab", x, grid[1][x].Style)
		}
	}

	// row 2: "a日" (1 + 2 = 3 columns) stops before '日' so ']' stays visible at col 2 -> [a]
	if grid[2][0].Ch != '[' || grid[2][1].Ch != 'a' || grid[2][2].Ch != ']' || grid[2][3].Ch != ' ' {
		t.Fatalf("row 2 cells = [%q %q %q %q], want ['[' 'a' ']' ' ']",
			grid[2][0].Ch, grid[2][1].Ch, grid[2][2].Ch, grid[2][3].Ch)
	}

	// row 3: single accented rune places ']' immediately after 'é' at col 2 with no byte-offset gap
	if grid[3][0].Ch != '[' || grid[3][1].Ch != 'é' || grid[3][2].Ch != ']' || grid[3][3].Ch != ' ' {
		t.Fatalf("row 3 cells = [%q %q %q %q], want ['[' 'é' ']' ' ']",
			grid[3][0].Ch, grid[3][1].Ch, grid[3][2].Ch, grid[3][3].Ch)
	}

	// row 4: unlabeled tab preserves numbered badge [>5]
	for x, want := range "[>5]" {
		if grid[4][x].Ch != want {
			t.Fatalf("row 4 col %d = %q, want %q", x, grid[4][x].Ch, want)
		}
	}

	// border at col 4 and padding at col 5 remain intact across all rows
	for y := 0; y < 5; y++ {
		if grid[y][4].Ch != '│' || grid[y][4].Style != term.StyleBorder {
			t.Fatalf("row %d border = %+v, want vertical border", y, grid[y][4])
		}
		if grid[y][5].Ch != ' ' || grid[y][5].Style != term.StyleDefault {
			t.Fatalf("row %d padding = %+v, want space", y, grid[y][5])
		}
	}
}

func TestVerticalTabBarNeverDrawsFullwidthRuneAcrossClipBoundary(t *testing.T) {
	v := NewVerticalTabBar()
	v.Count = 1
	v.Labels = []string{"日本"}
	v.SetRect(Rect{X: 0, Y: 0, W: 2, H: 1})

	grid := makeGrid(VerticalTabBarWidth, 1)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 2, H: 1})
	v.Render(surface)

	if grid[0][0].Ch != '[' {
		t.Fatalf("col 0 = %q, want '['", grid[0][0].Ch)
	}
	if grid[0][1].Ch == '日' {
		t.Fatal("fullwidth rune drawn at clip boundary would bleed into column 2")
	}
	if grid[0][2].Ch != '.' {
		t.Fatalf("col 2 outside clip = %q, want '.'", grid[0][2].Ch)
	}
}

func TestTerminalPanelKeepsLabelsAlignedWithTabs(t *testing.T) {
	tp := NewTerminalPanelWidget()
	tp.AddTerminalLabeled(&captureStubWidget{}, "pi")
	tp.AddTerminal(&captureStubWidget{})
	tp.AddTerminalLabeled(&captureStubWidget{}, "lg")

	if got := tp.TabBar.Labels; !slices.Equal(got, []string{"pi", "", "lg"}) {
		t.Fatalf("labels = %v, want [pi  lg]", got)
	}

	tp.RemoveTerminal(0)
	if got := tp.TabBar.Labels; !slices.Equal(got, []string{"", "lg"}) {
		t.Fatalf("labels after remove = %v, want [ lg]", got)
	}
	if tp.TabBar.Count != 2 {
		t.Fatalf("count = %d, want 2", tp.TabBar.Count)
	}
}
