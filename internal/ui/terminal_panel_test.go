package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type captureStubWidget struct {
	BaseWidget
	owns   bool
	events int
}

func (s *captureStubWidget) OwnsPointerCapture() bool { return s.owns }

func (s *captureStubWidget) HandleEvent(tcell.Event) EventResult {
	s.events++
	return EventCaptured
}

func TestTerminalPanelDragIntoTabStripDoesNotSwitchTerminal(t *testing.T) {
	tp := NewTerminalPanelWidget()
	first := &captureStubWidget{}
	second := &captureStubWidget{}
	tp.AddTerminal(first)
	tp.AddTerminal(second)
	tp.SetActive(0)
	tp.SetRect(Rect{X: 0, Y: 0, W: 40, H: 10})

	first.owns = true
	drag := tcell.NewEventMouse(2, 1, tcell.Button1, 0)
	tp.HandleEvent(drag)

	if tp.ActiveIndex() != 0 {
		t.Errorf("active = %d, want 0: drag into the tab strip switched terminals", tp.ActiveIndex())
	}
	if first.events != 1 {
		t.Errorf("active terminal got %d events, want 1", first.events)
	}
}

func TestTerminalPanelEmptyStateDrawTextWidth(t *testing.T) {
	t.Cleanup(func() { i18n.SetCatalog(nil) })

	t.Run("wide glyph advances by display width", func(t *testing.T) {
		catalog, err := i18n.ParseCatalog([]byte(`{"terminalPanel.noTerminalsPressToCreateOne":"界a"}`), nil)
		if err != nil {
			t.Fatal(err)
		}
		i18n.SetCatalog(catalog)

		tp := NewTerminalPanelWidget()
		grid := makeGrid(10, 2)
		tp.SetRect(Rect{X: 0, Y: 0, W: 10, H: 2})
		tp.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 10, H: 2}))

		if grid[0][1].Ch != '界' {
			t.Errorf("grid[0][1].Ch = %q, want '界'", grid[0][1].Ch)
		}
		if grid[0][1].Style != term.StyleMuted {
			t.Errorf("grid[0][1].Style = %v, want term.StyleMuted", grid[0][1].Style)
		}
		if grid[0][2].Ch != '.' {
			t.Errorf("grid[0][2].Ch = %q, want '.' (trailing cell of wide glyph)", grid[0][2].Ch)
		}
		if grid[0][3].Ch != 'a' {
			t.Errorf("grid[0][3].Ch = %q, want 'a' at display column 3", grid[0][3].Ch)
		}
		if grid[0][3].Style != term.StyleMuted {
			t.Errorf("grid[0][3].Style = %v, want term.StyleMuted", grid[0][3].Style)
		}
	})

	t.Run("wide glyph at last available column is replaced with space", func(t *testing.T) {
		catalog, err := i18n.ParseCatalog([]byte(`{"terminalPanel.noTerminalsPressToCreateOne":"界a"}`), nil)
		if err != nil {
			t.Fatal(err)
		}
		i18n.SetCatalog(catalog)

		tp := NewTerminalPanelWidget()
		grid := makeGrid(4, 2)
		tp.SetRect(Rect{X: 0, Y: 0, W: 2, H: 2})
		tp.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 2, H: 2}))

		if grid[0][1].Ch != ' ' {
			t.Errorf("grid[0][1].Ch = %q, want ' ' when wide glyph does not fit", grid[0][1].Ch)
		}
		if grid[0][2].Ch != '.' {
			t.Errorf("grid[0][2].Ch = %q, want '.' beyond clip edge", grid[0][2].Ch)
		}
	})

	t.Run("ascii message", func(t *testing.T) {
		catalog, err := i18n.ParseCatalog([]byte(`{"terminalPanel.noTerminalsPressToCreateOne":"No term"}`), nil)
		if err != nil {
			t.Fatal(err)
		}
		i18n.SetCatalog(catalog)

		tp := NewTerminalPanelWidget()
		grid := makeGrid(12, 2)
		tp.SetRect(Rect{X: 0, Y: 0, W: 12, H: 2})
		tp.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 12, H: 2}))

		want := "No term"
		for i, ch := range want {
			if grid[0][1+i].Ch != ch {
				t.Errorf("grid[0][%d].Ch = %q, want %q", 1+i, grid[0][1+i].Ch, ch)
			}
			if grid[0][1+i].Style != term.StyleMuted {
				t.Errorf("grid[0][%d].Style = %v, want term.StyleMuted", 1+i, grid[0][1+i].Style)
			}
		}
	})
}
