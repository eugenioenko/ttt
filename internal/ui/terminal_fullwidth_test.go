package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/xterm-go"
	"github.com/gdamore/tcell/v3"
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

func TestTerminalPanelPaddingKeepsContentCoordinatesAndScrollbar(t *testing.T) {
	const cols, rows = 13, 3
	dev, err := terminal.New("/bin/cat", cols, rows, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	tw := NewTerminalWidget(dev, &TerminalColorPalette{})
	tp := NewTerminalPanelWidget()
	tp.AddTerminal(tw)
	tp.SetRect(Rect{X: 2, Y: 1, W: 20, H: rows})
	grid := makeGrid(24, 5)
	surface := NewRenderSurface(grid, tp.GetRect())
	dev.Snapshot(func(xt *xterm.Terminal) { xt.WriteString("漢xx") })
	tp.Render(surface)
	r := tw.GetRect()
	if x, y, visible := tp.CursorPosition(); !visible || x != r.X+4 || y != r.Y {
		t.Fatalf("cursor = (%d,%d,%v), want (%d,%d,true)", x, y, visible, r.X+4, r.Y)
	}
	if col, row := tw.mousePTYCoords(r.X+3, r.Y+1); col != 4 || row != 2 {
		t.Fatalf("PTY coordinates = (%d,%d), want (4,2)", col, row)
	}
	tp.HandleEvent(tcell.NewEventMouse(r.X, r.Y, tcell.Button1, 0))
	tp.HandleEvent(tcell.NewEventMouse(r.X+3, r.Y, tcell.Button1, 0))
	tp.HandleEvent(tcell.NewEventMouse(r.X+3, r.Y, tcell.ButtonNone, 0))
	if got := tw.selectedText(); got != "漢x" {
		t.Fatalf("selected text = %q, want 漢x", got)
	}
	tw.ClearSelection()
	dev.Snapshot(func(xt *xterm.Terminal) {
		xt.WriteString(strings.Repeat("\r\n", rows*2) + "\x1b[H漢" + strings.Repeat("x", cols-2))
	})
	for _, offset := range []int{0, 1} {
		tw.scrollOffset = offset
		tp.Render(surface)
		if tw.scrollbar.X != r.X+r.W-1 {
			t.Fatalf("scrollbar x = %d, want %d", tw.scrollbar.X, r.X+r.W-1)
		}
		for y := r.Y; y < r.Y+r.H; y++ {
			if grid[y][r.X-1].Ch != ' ' || grid[y][r.X+r.W].Ch != ' ' {
				t.Fatalf("padding overwritten on row %d, scroll offset %d", y, offset)
			}
		}
	}
}
