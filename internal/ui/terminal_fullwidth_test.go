package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/xterm-go"
	"github.com/gdamore/tcell/v3"
)

// A fullwidth rune must never be drawn in the last column of the content
// area: the terminal paints it across two columns regardless of clipping, so
// it bleeds outside the text surface (#636).
func TestTerminalRenderClipsWideRuneAtTheContentEdge(t *testing.T) {
	const cols, rows = 20, 5

	dev, err := terminal.New("/bin/cat", cols, rows, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()

	// Leave the emulator wider than the clipped surface to place a fullwidth
	// rune at the surface edge without the emulator wrapping it first.
	var sb strings.Builder
	for i := 0; i < rows*2; i++ {
		sb.WriteString(strings.Repeat("x", cols-2) + "漢\r\n")
	}
	dev.Snapshot(func(xt *xterm.Terminal) { xt.WriteString(sb.String()) })

	tw := NewTerminalWidget(dev, &TerminalColorPalette{})
	tw.SetRect(Rect{W: cols - 1, H: rows})
	cells := make([][]term.Cell, rows)
	for y := range cells {
		cells[y] = make([]term.Cell, cols)
	}
	surface := NewRenderSurface(cells, Rect{W: cols - 1, H: rows})

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
		r = tw.GetRect()
		if tw.scrollbar.X != tp.GetRect().X+tp.GetRect().W-1 {
			t.Fatalf("scrollbar x = %d, want %d", tw.scrollbar.X, tp.GetRect().X+tp.GetRect().W-1)
		}
		for y := r.Y; y < r.Y+r.H; y++ {
			if grid[y][r.X-1].Ch != ' ' || grid[y][r.X+r.W].Ch != ' ' {
				t.Fatalf("padding overwritten on row %d, scroll offset %d", y, offset)
			}
		}
	}
}

func TestTerminalPanelPaddingSeparatesTextFromScrollbar(t *testing.T) {
	for _, padding := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(padding), func(t *testing.T) {
			const width, rows = 24, 3
			cols := width - VerticalTabBarWidth - 1 - 2*padding
			dev, err := terminal.New("/bin/cat", cols+1, rows, 1000, nil, "")
			if err != nil {
				t.Fatal(err)
			}
			defer dev.Close()
			tw := NewTerminalWidget(dev, nil)
			tp := NewTerminalPanelWidget()
			tp.PaddingX = padding
			tp.AddTerminal(tw)
			tp.SetRect(Rect{X: 2, Y: 1, W: width, H: rows})
			grid := makeGrid(width+4, rows+2)
			surface := NewRenderSurface(grid, tp.GetRect())
			tp.Render(surface)
			before := tw.GetRect()
			barX := tp.GetRect().X + width - 1
			if before.W != cols+1 || tw.scrollbar.Visible() {
				t.Fatal("empty terminal displayed a scrollbar")
			}
			dev.Resize(cols, rows)
			line := strings.Repeat("X", cols-1) + "Z"
			dev.Snapshot(func(xt *xterm.Terminal) {
				xt.WriteString(strings.Repeat(line+"\r\n", rows*3) + line)
			})
			for _, offset := range []int{0, 1} {
				tw.scrollOffset = offset
				tp.Render(surface)
				r := tw.GetRect()
				if r.W != before.W-1 || r.X != before.X {
					t.Fatalf("scrollbar text rectangle = %+v, hidden = %+v", r, before)
				}
				if !tw.scrollbar.Visible() || tw.scrollbar.X != barX {
					t.Fatal("scrollbar is not at the panel edge")
				}
				for y := r.Y; y < r.Y+r.H; y++ {
					if grid[y][r.X+r.W-1].Ch != 'Z' {
						t.Fatalf("last text column was lost on row %d, scroll offset %d", y, offset)
					}
					for i := 0; i < padding; i++ {
						if grid[y][r.X-1-i].Ch != ' ' || grid[y][r.X+r.W+i].Ch != ' ' {
							t.Fatalf("text is missing padding on row %d, scroll offset %d", y, offset)
						}
					}
					if grid[y][barX].Ch != '█' {
						t.Fatal("scrollbar was not rendered outside the text padding")
					}
				}
			}
			if padding > 0 {
				tp.HandleEvent(tcell.NewEventMouse(barX-1, before.Y, tcell.Button1, 0))
				tp.HandleEvent(tcell.NewEventMouse(barX-1, before.Y, tcell.ButtonNone, 0))
				if tw.HasSelection() || tw.OwnsPointerCapture() {
					t.Fatal("padding click started selection or scrollbar dragging")
				}
			}
			if got := tp.HandleEvent(tcell.NewEventMouse(barX, before.Y, tcell.Button1, 0)); got != EventCaptured {
				t.Fatalf("scrollbar click = %v, want pointer capture", got)
			}
			tp.HandleEvent(tcell.NewEventMouse(before.X, before.Y+rows-1, tcell.Button1, 0))
			tp.HandleEvent(tcell.NewEventMouse(before.X, before.Y+rows-1, tcell.ButtonNone, 0))
			if tw.HasSelection() || tw.OwnsPointerCapture() || tw.IsScrolledUp() {
				t.Fatal("scrollbar drag did not release at the live view")
			}
			dev.Snapshot(func(xt *xterm.Terminal) { xt.WriteString("\x1b[?1049h") })
			dev.Resize(cols+1, rows)
			dev.Snapshot(func(xt *xterm.Terminal) { xt.WriteString("\x1b[H" + strings.Repeat("X", cols) + "A") })
			tp.Render(surface)
			if tw.scrollbar.Visible() || tw.GetRect() != before {
				t.Fatal("alternate screen retained the scrollbar or changed text geometry")
			}
			if grid[before.Y][before.X+before.W-1].Ch != 'A' {
				t.Fatal("alternate screen did not use the reclaimed scrollbar column")
			}
			for x := before.X + before.W; x <= barX; x++ {
				if grid[before.Y][x].Ch != ' ' {
					t.Fatal("alternate screen lost its right padding")
				}
			}
			if padding > 0 {
				tp.HandleEvent(tcell.NewEventMouse(barX, before.Y, tcell.Button1, 0))
				if tw.HasSelection() || tw.OwnsPointerCapture() {
					t.Fatal("hidden scrollbar received the click")
				}
			}
		})
	}
}
