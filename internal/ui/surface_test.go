package ui

import (
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
)

func makeGrid(w, h int) [][]term.Cell {
	grid := make([][]term.Cell, h)
	for y := range grid {
		grid[y] = make([]term.Cell, w)
		for x := range grid[y] {
			grid[y][x] = term.Cell{Ch: '.'}
		}
	}
	return grid
}

func TestSetCellWithinBounds(t *testing.T) {
	grid := makeGrid(10, 5)
	s := NewRenderSurface(grid, Rect{X: 2, Y: 1, W: 5, H: 3})

	s.SetCell(0, 0, term.Cell{Ch: 'A'})
	if grid[1][2].Ch != 'A' {
		t.Fatalf("expected 'A' at grid[1][2], got '%c'", grid[1][2].Ch)
	}

	s.SetCell(4, 2, term.Cell{Ch: 'B'})
	if grid[3][6].Ch != 'B' {
		t.Fatalf("expected 'B' at grid[3][6], got '%c'", grid[3][6].Ch)
	}
}

func TestSetCellOutOfBoundsClipped(t *testing.T) {
	grid := makeGrid(10, 5)
	s := NewRenderSurface(grid, Rect{X: 2, Y: 1, W: 5, H: 3})

	s.SetCell(-1, 0, term.Cell{Ch: 'X'})
	s.SetCell(0, -1, term.Cell{Ch: 'X'})
	s.SetCell(5, 0, term.Cell{Ch: 'X'})
	s.SetCell(0, 3, term.Cell{Ch: 'X'})

	for y := range grid {
		for x := range grid[y] {
			if grid[y][x].Ch == 'X' {
				t.Fatalf("'X' written at grid[%d][%d], should have been clipped", y, x)
			}
		}
	}
}

func TestSubComposesOffsets(t *testing.T) {
	grid := makeGrid(20, 10)
	parent := NewRenderSurface(grid, Rect{X: 5, Y: 3, W: 10, H: 5})
	child := parent.Sub(Rect{X: 2, Y: 1, W: 4, H: 2})

	child.SetCell(0, 0, term.Cell{Ch: 'C'})
	// parent offset (5,3) + sub offset (2,1) = absolute (7,4)
	if grid[4][7].Ch != 'C' {
		t.Fatalf("expected 'C' at grid[4][7], got '%c'", grid[4][7].Ch)
	}
}

func TestSubClipsToParent(t *testing.T) {
	grid := makeGrid(20, 10)
	parent := NewRenderSurface(grid, Rect{X: 5, Y: 3, W: 10, H: 5})
	// Child extends beyond parent bounds
	child := parent.Sub(Rect{X: 8, Y: 3, W: 5, H: 5})

	w, h := child.Size()
	// Parent ends at X=15, child starts at X=13, so width should be clamped to 2
	if w != 2 {
		t.Fatalf("expected child width 2, got %d", w)
	}
	// Parent ends at Y=8, child starts at Y=6, so height should be clamped to 2
	if h != 2 {
		t.Fatalf("expected child height 2, got %d", h)
	}
}

func TestNestedSub(t *testing.T) {
	grid := makeGrid(30, 20)
	root := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 30, H: 20})
	level1 := root.Sub(Rect{X: 5, Y: 5, W: 20, H: 10})
	level2 := level1.Sub(Rect{X: 3, Y: 2, W: 10, H: 5})

	level2.SetCell(1, 1, term.Cell{Ch: 'N'})
	// absolute: (0+5+3+1, 0+5+2+1) = (9, 8)
	if grid[8][9].Ch != 'N' {
		t.Fatalf("expected 'N' at grid[8][9], got '%c'", grid[8][9].Ch)
	}
}

func TestFill(t *testing.T) {
	grid := makeGrid(10, 5)
	s := NewRenderSurface(grid, Rect{X: 1, Y: 1, W: 3, H: 2})
	s.Fill(term.Cell{Ch: '#'})

	for y := 1; y <= 2; y++ {
		for x := 1; x <= 3; x++ {
			if grid[y][x].Ch != '#' {
				t.Fatalf("expected '#' at grid[%d][%d], got '%c'", y, x, grid[y][x].Ch)
			}
		}
	}
	// Verify outside the fill region is untouched
	if grid[0][0].Ch != '.' {
		t.Fatal("fill leaked outside clip region")
	}
}

func TestSize(t *testing.T) {
	grid := makeGrid(10, 5)
	s := NewRenderSurface(grid, Rect{X: 2, Y: 1, W: 6, H: 3})
	w, h := s.Size()
	if w != 6 || h != 3 {
		t.Fatalf("expected size (6,3), got (%d,%d)", w, h)
	}
}

func TestDrawText(t *testing.T) {
	grid := makeGrid(20, 5)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 5})

	endX := s.DrawText(2, 1, "Hello", 0, term.StyleDefault)
	if endX != 7 {
		t.Fatalf("expected endX 7, got %d", endX)
	}
	expected := "Hello"
	for i, ch := range expected {
		if grid[1][2+i].Ch != ch {
			t.Fatalf("expected '%c' at grid[1][%d], got '%c'", ch, 2+i, grid[1][2+i].Ch)
		}
	}
}

func TestDrawTextMaxW(t *testing.T) {
	grid := makeGrid(20, 5)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 5})

	endX := s.DrawText(2, 1, "Hello World", 5, term.StyleDefault)
	if endX != 5 {
		t.Fatalf("expected endX 5, got %d", endX)
	}
	if grid[1][5].Ch != '.' {
		t.Fatal("text should have been clipped at maxW")
	}
}

func TestClearRect(t *testing.T) {
	grid := makeGrid(10, 5)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 10, H: 5})

	s.ClearRect(1, 1, 3, 2, term.StyleDefault)
	for y := 1; y <= 2; y++ {
		for x := 1; x <= 3; x++ {
			if grid[y][x].Ch != ' ' {
				t.Fatalf("expected ' ' at grid[%d][%d], got '%c'", y, x, grid[y][x].Ch)
			}
		}
	}
	if grid[0][0].Ch != '.' {
		t.Fatal("ClearRect leaked outside region")
	}
}

func TestClearRectClampsToSurfaceBounds(t *testing.T) {
	grid := makeGrid(30, 20)
	s := NewRenderSurface(grid, Rect{X: 5, Y: 4, W: 20, H: 10})
	done := make(chan struct{})
	go func() {
		s.ClearRect(-10, -10, 100000, 100000, term.StyleDefault)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ClearRect did not return within 2 seconds")
	}
	s.ClearRect(100000, 100000, 10, 10, term.StyleDefault)
	for y := range grid {
		for x := range grid[y] {
			want := '.'
			if x >= 5 && x < 25 && y >= 4 && y < 14 {
				want = ' '
			}
			if got := grid[y][x].Ch; got != want {
				t.Errorf("grid cell (%d,%d) = %q, want %q", x, y, got, want)
			}
		}
	}
}
