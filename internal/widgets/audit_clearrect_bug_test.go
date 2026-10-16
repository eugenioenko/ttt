package widgets

import (
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestClearRectClampsToSurfaceBounds(t *testing.T) {
	surface := newVirtualSurface(20, 10)
	surface.Fill(term.Cell{Ch: 'x'})
	assertClearRectCompletes(t, func() {
		surface.ClearRect(0, 0, 100000, 100000, term.StyleDefault)
	})
	for y := range surface.h {
		for x := range surface.w {
			if got := surface.cells[y][x].Ch; got != ' ' {
				t.Errorf("surface cell (%d,%d) = %q, want space", x, y, got)
			}
		}
	}
}

func TestSubVirtualSurfaceClearRectClampsToParentBounds(t *testing.T) {
	parent := newVirtualSurface(30, 20)
	parent.Fill(term.Cell{Ch: 'x'})
	sub := parent.Sub(Rect{X: 5, Y: 4, W: 100000, H: 100000}).(*subVirtualSurface)
	assertClearRectCompletes(t, func() {
		sub.ClearRect(-10, -10, 100000, 100000, term.StyleDefault)
	})
	for y := range parent.h {
		for x := range parent.w {
			want := 'x'
			if x >= 5 && y >= 4 {
				want = ' '
			}
			if got := parent.cells[y][x].Ch; got != want {
				t.Errorf("parent cell (%d,%d) = %q, want %q", x, y, got, want)
			}
		}
	}
}

func assertClearRectCompletes(t *testing.T, clear func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		clear()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ClearRect did not return within 2 seconds")
	}
}
