package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/gitpod-io/xterm-go"
)

func TestTerminalRenderReplacesFullwidthRuneOverScrollbar(t *testing.T) {
	const (
		cols = 8
		rows = 3
		w    = 6
		h    = 2
	)
	// contentW is w-1 once scrollback shows the scrollbar. A width-2 rune
	// starting at that last content column would paint into the scrollbar.
	termSess, err := terminal.New("/bin/cat", cols, rows, 100, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer termSess.Close()

	line := "abc 中
"
	extra := "scrollback
"
	termSess.Snapshot(func(xt *xterm.Terminal) {
		xt.WriteString(extra + extra + line + line)
	})

	tw := NewTerminalWidget(termSess, &TerminalColorPalette{})
	tw.SetRect(Rect{W: w, H: h})

	assertClippedWideLine(t, tw, "live view")

	tw.scrollOffset = 1
	assertClippedWideLine(t, tw, "scrollback")
}

func assertClippedWideLine(t *testing.T, tw *TerminalWidget, path string) {
	t.Helper()
	const w, h = 6, 2
	surface := newTestSurface(w, h)
	tw.Render(surface)
	found := false
	for y := 0; y < h; y++ {
		if surface.cells[y][2].Ch != 'c' {
			continue
		}
		found = true
		if got := surface.cells[y][4].Ch; got != ' ' {
			t.Errorf("%s row %d last content column = %q, want space instead of a fullwidth rune", path, y, got)
		}
		if got := surface.cells[y][5].Ch; got == '中' {
			t.Errorf("%s row %d scrollbar column = fullwidth rune", path, y)
		}
	}
	if !found {
		t.Fatalf("%s did not render the wide-rune line", path)
	}
}

func newTestSurface(w, h int) *RenderSurface {
	cells := make([][]term.Cell, h)
	for y := range cells {
		cells[y] = make([]term.Cell, w)
	}
	return NewRenderSurface(cells, Rect{W: w, H: h})
}
