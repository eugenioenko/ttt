package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/xterm-go"
)

// A fullwidth rune must never be drawn in the last column of the content
// area: the terminal paints it across two columns regardless of clipping, so
// it bleeds over the scrollbar to its right (#636).
func TestTerminalRenderClipsWideRuneAtTheScrollbar(t *testing.T) {
	const cols, rows = 20, 5

	dev, err := terminal.New("/bin/cat", cols, rows, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()

	// Every line ends with a fullwidth rune in the last content column. The
	// scrollbar takes the last column of the widget, so the content is
	// columns-1 wide and the wide rune lands at columns-2.
	var sb strings.Builder
	for i := 0; i < rows*2; i++ {
		sb.WriteString(strings.Repeat("x", cols-2) + "漢\r\n")
	}
	dev.Snapshot(func(xt *xterm.Terminal) { xt.WriteString(sb.String()) })

	tw := NewTerminalWidget(dev, &TerminalColorPalette{})
	tw.SetRect(Rect{W: cols, H: rows})
	cells := make([][]term.Cell, rows)
	for y := range cells {
		cells[y] = make([]term.Cell, cols)
	}
	surface := NewRenderSurface(cells, Rect{W: cols, H: rows})

	// Live view.
	tw.Render(surface)
	if got := cells[0][cols-2].Ch; got != ' ' {
		t.Errorf("live view: last content column = %U, want a space", got)
	}

	// Scrollback view.
	tw.scrollOffset = 1
	tw.Render(surface)
	if got := cells[0][cols-2].Ch; got != ' ' {
		t.Errorf("scrollback: last content column = %U, want a space", got)
	}
}

// A fullwidth rune anywhere else in the content area is still drawn.
func TestTerminalRenderKeepsWideRuneInsideTheContent(t *testing.T) {
	const cols, rows = 20, 5

	dev, err := terminal.New("/bin/cat", cols, rows, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()

	dev.Snapshot(func(xt *xterm.Terminal) {
		for i := 0; i < rows*2; i++ {
			xt.WriteString("漢" + strings.Repeat("x", cols-4) + "\r\n")
		}
	})

	tw := NewTerminalWidget(dev, &TerminalColorPalette{})
	tw.SetRect(Rect{W: cols, H: rows})
	cells := make([][]term.Cell, rows)
	for y := range cells {
		cells[y] = make([]term.Cell, cols)
	}
	surface := NewRenderSurface(cells, Rect{W: cols, H: rows})

	tw.Render(surface)
	if got := cells[0][0].Ch; got != '漢' {
		t.Errorf("first column = %U, want the wide rune", got)
	}
}
