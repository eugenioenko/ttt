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
		if grid[0][x].Style != term.StyleActiveTab {
			t.Fatalf("row 0 col %d style = %v, want StyleActiveTab", x, grid[0][x].Style)
		}
	}
	// row 1: an unlabelled tab keeps the numbered badge
	for x, want := range "[>2]" {
		if grid[1][x].Ch != want {
			t.Fatalf("row 1 col %d = %q, want %q", x, grid[1][x].Ch, want)
		}
		if grid[1][x].Style != term.StyleInactiveTab {
			t.Fatalf("row 1 col %d style = %v, want StyleInactiveTab", x, grid[1][x].Style)
		}
	}
}

func TestVerticalTabBarAccentedLabelBadge(t *testing.T) {
	v := NewVerticalTabBar()
	v.Count = 2
	v.Active = 0
	v.Labels = []string{"él", "ñ"}
	v.SetRect(Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 4})

	grid := makeGrid(VerticalTabBarWidth, 4)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 4})
	v.Render(surface)

	for x, want := range []rune{'[', 'é', 'l', ']'} {
		if grid[0][x].Ch != want {
			t.Fatalf("row 0 col %d = %q, want %q", x, grid[0][x].Ch, want)
		}
		if grid[0][x].Style != term.StyleActiveTab {
			t.Fatalf("row 0 col %d style = %v, want StyleActiveTab", x, grid[0][x].Style)
		}
	}
	if grid[0][4].Ch != '│' || grid[0][4].Style != term.StyleBorder {
		t.Fatalf("border at col 4 = %q, want '│' with StyleBorder", grid[0][4].Ch)
	}

	for x, want := range []rune{'[', 'ñ', ']'} {
		if grid[1][x].Ch != want {
			t.Fatalf("row 1 col %d = %q, want %q", x, grid[1][x].Ch, want)
		}
		if grid[1][x].Style != term.StyleInactiveTab {
			t.Fatalf("row 1 col %d style = %v, want StyleInactiveTab", x, grid[1][x].Style)
		}
	}
	if grid[1][4].Ch != '│' || grid[1][4].Style != term.StyleBorder {
		t.Fatalf("border at col 4 = %q, want '│' with StyleBorder", grid[1][4].Ch)
	}
}

func TestVerticalTabBarFullwidthLabelBadge(t *testing.T) {
	v := NewVerticalTabBar()
	v.Count = 2
	v.Active = 0
	v.Labels = []string{"日本", "語"}
	v.SetRect(Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 4})

	grid := makeGrid(VerticalTabBarWidth, 4)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 4})
	v.Render(surface)

	if grid[0][0].Ch != '[' || grid[0][0].Style != term.StyleActiveTab {
		t.Fatalf("col 0 = %q, want '[' with StyleActiveTab", grid[0][0].Ch)
	}
	if grid[0][1].Ch != '日' || grid[0][1].Style != term.StyleActiveTab {
		t.Fatalf("col 1 = %q, want '日' with StyleActiveTab", grid[0][1].Ch)
	}
	if grid[0][3].Ch != ']' || grid[0][3].Style != term.StyleActiveTab {
		t.Fatalf("col 3 = %q, want ']' with StyleActiveTab", grid[0][3].Ch)
	}
	if grid[0][4].Ch != '│' || grid[0][4].Style != term.StyleBorder {
		t.Fatalf("border at col 4 = %q, want '│' with StyleBorder", grid[0][4].Ch)
	}
	if grid[0][5].Ch != ' ' {
		t.Fatalf("padding at col 5 = %q, want ' '", grid[0][5].Ch)
	}

	if grid[1][0].Ch != '[' || grid[1][0].Style != term.StyleInactiveTab {
		t.Fatalf("row 1 col 0 = %q, want '[' with StyleInactiveTab", grid[1][0].Ch)
	}
	if grid[1][1].Ch != '語' || grid[1][1].Style != term.StyleInactiveTab {
		t.Fatalf("row 1 col 1 = %q, want '語' with StyleInactiveTab", grid[1][1].Ch)
	}
	if grid[1][3].Ch != ']' || grid[1][3].Style != term.StyleInactiveTab {
		t.Fatalf("row 1 col 3 = %q, want ']' with StyleInactiveTab", grid[1][3].Ch)
	}
	if grid[1][4].Ch != '│' || grid[1][4].Style != term.StyleBorder {
		t.Fatalf("row 1 border at col 4 = %q, want '│' with StyleBorder", grid[1][4].Ch)
	}
}

func TestVerticalTabBarTruncation(t *testing.T) {
	cases := []struct {
		input     string
		wantInner string
	}{
		{"planner", "pl"},
		{"日本", "日"},
		{"école", "éc"},
		{"a日本", "a"},
		{"日a", "日"},
	}

	for _, tc := range cases {
		if got := truncateTabLabel(tc.input); got != tc.wantInner {
			t.Errorf("truncateTabLabel(%q) = %q, want %q", tc.input, got, tc.wantInner)
		}
	}

	v := NewVerticalTabBar()
	v.Count = len(cases)
	for _, tc := range cases {
		v.Labels = append(v.Labels, tc.input)
	}
	v.SetRect(Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: len(cases)})

	grid := makeGrid(VerticalTabBarWidth, len(cases))
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: len(cases)})
	v.Render(surface)

	for row, tc := range cases {
		if grid[row][4].Ch != '│' || grid[row][4].Style != term.StyleBorder {
			t.Errorf("row %d (%q): border at col 4 = %q, want '│'", row, tc.input, grid[row][4].Ch)
		}
		if grid[row][5].Ch != ' ' {
			t.Errorf("row %d (%q): padding at col 5 = %q, want ' '", row, tc.input, grid[row][5].Ch)
		}
		if grid[row][0].Ch != '[' {
			t.Errorf("row %d (%q): col 0 = %q, want '['", row, tc.input, grid[row][0].Ch)
		}
		foundClose := false
		for x := 1; x < 4; x++ {
			if grid[row][x].Ch == ']' {
				foundClose = true
				break
			}
		}
		if !foundClose {
			t.Errorf("row %d (%q): closing bracket ']' not found in cols 1..3", row, tc.input)
		}
	}
}

func TestVerticalTabBarDefaultNumberedBadges(t *testing.T) {
	v := NewVerticalTabBar()
	v.Count = 10
	v.Active = 9
	v.SetRect(Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 10})

	grid := makeGrid(VerticalTabBarWidth, 10)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: VerticalTabBarWidth, H: 10})
	v.Render(surface)

	for x, want := range "[>1]" {
		if grid[0][x].Ch != want {
			t.Fatalf("row 0 col %d = %q, want %q", x, grid[0][x].Ch, want)
		}
		if grid[0][x].Style != term.StyleInactiveTab {
			t.Fatalf("row 0 col %d style = %v, want StyleInactiveTab", x, grid[0][x].Style)
		}
	}
	if grid[0][4].Ch != '│' || grid[0][4].Style != term.StyleBorder {
		t.Fatalf("row 0 border = %q, want '│'", grid[0][4].Ch)
	}

	for x, want := range "[10]" {
		if grid[9][x].Ch != want {
			t.Fatalf("row 9 col %d = %q, want %q", x, grid[9][x].Ch, want)
		}
		if grid[9][x].Style != term.StyleActiveTab {
			t.Fatalf("row 9 col %d style = %v, want StyleActiveTab", x, grid[9][x].Style)
		}
	}
	if grid[9][4].Ch != '│' || grid[9][4].Style != term.StyleBorder {
		t.Fatalf("row 9 border = %q, want '│'", grid[9][4].Ch)
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
