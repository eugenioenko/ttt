package ui

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type terminalPaddingProbe struct {
	captureStubWidget
}

func (p *terminalPaddingProbe) Render(s Surface) {
	s.Fill(term.Cell{Ch: 'X', Style: term.StyleDefault})
}

func (p *terminalPaddingProbe) CursorPosition() (int, int, bool) {
	r := p.GetRect()
	return r.X, r.Y, true
}

func TestTerminalPanelPadding(t *testing.T) {
	for _, tt := range []struct {
		name           string
		width, padding int
		left, cols     int
	}{
		{"zero", 20, 0, 5, 15},
		{"default", 20, 1, 6, 13},
		{"custom", 20, 2, 7, 11},
		{"negative", 20, -1, 5, 15},
		{"oversized", 20, int(^uint(0) >> 1), 11, 3},
		{"narrow", 9, 2, 6, 2},
		{"odd narrow", 8, 2, 5, 3},
		{"two columns", 7, 1, 5, 2},
		{"one column", 6, 1, 5, 1},
		{"no content", 5, 1, 5, 0},
		{"clipped border", 4, 1, 4, 0},
		{"clipped strip", 3, 1, 3, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tp := NewTerminalPanelWidget()
			tp.Padding = tt.padding
			probe := &terminalPaddingProbe{}
			tp.AddTerminal(probe)
			r := Rect{X: 3, Y: 2, W: tt.width, H: 3}
			tp.SetRect(r)
			grid := makeGrid(r.X+r.W+1, r.Y+r.H+1)
			tp.Render(NewRenderSurface(grid, r))
			want := Rect{X: r.X + tt.left, Y: r.Y, W: tt.cols, H: r.H}
			if got := probe.GetRect(); got != want {
				t.Fatalf("content rect = %+v, want %+v", got, want)
			}
			if _, _, visible := tp.CursorPosition(); visible != (tt.cols > 0) {
				t.Fatalf("cursor visible = %v, content width = %d", visible, tt.cols)
			}
			for y := r.Y; y < r.Y+r.H; y++ {
				for x := min(VerticalTabBarWidth, r.W); x < r.W; x++ {
					ch := ' '
					if x >= tt.left && x < tt.left+tt.cols {
						ch = 'X'
					}
					if got := grid[y][r.X+x].Ch; got != ch {
						t.Fatalf("cell (%d,%d) = %q, want %q", x, y, got, ch)
					}
				}
				if grid[y][r.X+r.W].Ch != '.' {
					t.Fatal("render crossed the panel boundary")
				}
			}
			divider := -1
			if r.W > 4 {
				divider = r.X + 4
				if grid[r.Y][divider].Ch != '│' {
					t.Fatal("reported divider does not match the rendered border")
				}
			}
			if got := tp.DividerScreenX(); got != divider {
				t.Fatalf("divider = %d, want %d", got, divider)
			}
		})
	}
}

func TestTerminalPanelPaddingMouseRouting(t *testing.T) {
	for _, padding := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(padding), func(t *testing.T) {
			tp := NewTerminalPanelWidget()
			tp.Padding = padding
			probe := &terminalPaddingProbe{}
			tp.AddTerminal(probe)
			tp.AddTerminal(&terminalPaddingProbe{})
			tp.SetActive(0)
			tp.SetRect(Rect{X: 3, Y: 2, W: 20, H: 3})
			tp.Render(NewRenderSurface(makeGrid(25, 6), tp.GetRect()))
			r := probe.GetRect()
			for _, x := range []int{r.X - 1, r.X + r.W} {
				want := EventConsumed
				if x >= tp.GetRect().X+tp.GetRect().W {
					want = EventIgnored
				}
				if got := tp.HandleEvent(tcell.NewEventMouse(x, r.Y, tcell.Button1, 0)); got != want {
					t.Fatalf("click at %d = %v, want %v", x, got, want)
				}
			}
			if probe.events != 0 {
				t.Fatal("padding or tab strip forwarded a mouse event to the terminal")
			}
			for _, x := range []int{tp.GetRect().X, r.X - 1, r.X, tp.GetRect().X + tp.GetRect().W - 1} {
				for _, button := range []tcell.ButtonMask{tcell.WheelUp, tcell.WheelDown, tcell.WheelLeft, tcell.WheelRight} {
					if got := tp.HandleEvent(tcell.NewEventMouse(x, r.Y, button, 0)); got != EventCaptured {
						t.Fatalf("wheel at %d = %v, want forwarded event", x, got)
					}
				}
			}
			if probe.events != 16 {
				t.Fatalf("forwarded wheel events = %d, want 16", probe.events)
			}
			probe.events = 0
			tp.HandleEvent(tcell.NewEventMouse(r.X, r.Y, tcell.Button1, 0))
			if probe.events != 1 {
				t.Fatal("first content cell did not receive the click")
			}
			probe.owns = true
			tp.HandleEvent(tcell.NewEventMouse(r.X+r.W, r.Y, tcell.Button1, 0))
			tp.HandleEvent(tcell.NewEventMouse(4, r.Y+1, tcell.Button1, 0))
			tp.HandleEvent(tcell.NewEventMouse(r.X+r.W, r.Y, tcell.ButtonNone, 0))
			if probe.events != 4 || tp.ActiveIndex() != 0 {
				t.Fatal("captured drag or release was lost outside the content")
			}
			probe.owns = false
			tp.HandleEvent(tcell.NewEventMouse(4, r.Y+1, tcell.Button1, 0))
			if tp.ActiveIndex() != 1 {
				t.Fatal("tab strip click did not switch terminals")
			}
		})
	}
}
