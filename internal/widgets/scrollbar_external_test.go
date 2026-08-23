package widgets_test

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type externalScrollbar interface {
	Render(widgets.Surface, widgets.ScrollbarGeometry, widgets.ScrollRange) (int, bool)
	HandleEvent(tcell.Event) (int, widgets.EventResult)
	CancelPointerCapture() bool
	OwnsPointerCapture() bool
}

type scrollbarSurface struct {
	cells [][]term.Cell
}

func newScrollbarSurface(w, h int) *scrollbarSurface {
	cells := make([][]term.Cell, h)
	for y := range cells {
		cells[y] = make([]term.Cell, w)
	}
	return &scrollbarSurface{cells: cells}
}

func (s *scrollbarSurface) Size() (int, int)              { return len(s.cells[0]), len(s.cells) }
func (s *scrollbarSurface) Origin() (int, int)            { return 0, 0 }
func (s *scrollbarSurface) SetCell(x, y int, c term.Cell) { s.cells[y][x] = c }
func (s *scrollbarSurface) DrawText(x, y int, text string, maxW int, style term.Style) int {
	return x
}
func (s *scrollbarSurface) DrawBorder(x, y, w, h int, b term.BorderSet, style term.Style) {}
func (s *scrollbarSurface) ClearRect(x, y, w, h int, style term.Style)                    {}
func (s *scrollbarSurface) Fill(c term.Cell)                                              {}
func (s *scrollbarSurface) Sub(r widgets.Rect) widgets.Surface                            { return s }

func TestExportedScrollbarContractPreservesOrientationAndGeometry(t *testing.T) {
	tests := []struct {
		name     string
		glyph    rune
		bar      externalScrollbar
		geometry widgets.ScrollbarGeometry
		press    *tcell.EventMouse
		cell     func(*scrollbarSurface) term.Cell
	}{
		{
			name:  "vertical",
			glyph: '█',
			bar:   &widgets.VerticalScrollbar{},
			geometry: widgets.NewScrollbarGeometry(
				widgets.Rect{X: 2, Y: 1, W: 1, H: 5},
				widgets.Rect{X: 12, Y: 21, W: 1, H: 5},
			),
			press: tcell.NewEventMouse(12, 25, tcell.Button1, 0),
			cell:  func(surface *scrollbarSurface) term.Cell { return surface.cells[1][2] },
		},
		{
			name:  "horizontal",
			glyph: '▄',
			bar:   &widgets.HorizontalScrollbar{},
			geometry: widgets.NewScrollbarGeometry(
				widgets.Rect{X: 2, Y: 1, W: 5, H: 1},
				widgets.Rect{X: 12, Y: 21, W: 5, H: 1},
			),
			press: tcell.NewEventMouse(16, 21, tcell.Button1, 0),
			cell:  func(surface *scrollbarSurface) term.Cell { return surface.cells[1][2] },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rangeModel := widgets.NewScrollRange(5, 10, 50)
			if rangeModel.Offset() != 5 || rangeModel.MaxOffset() != 5 || !rangeModel.Visible() {
				t.Fatalf("range = offset %d max %d visible %v", rangeModel.Offset(), rangeModel.MaxOffset(), rangeModel.Visible())
			}
			if rangeModel.ClampOffset(-2) != 0 || rangeModel.ClampOffset(20) != 5 {
				t.Fatal("range clamping did not preserve exact endpoints")
			}
			if tt.geometry.LocalTrack() == tt.geometry.HitTrack() {
				t.Fatal("render-local and absolute hit geometry collapsed")
			}

			surface := newScrollbarSurface(20, 10)
			if offset, invalidated := tt.bar.Render(surface, tt.geometry, widgets.NewScrollRange(5, 10, 0)); offset != 0 || invalidated {
				t.Fatalf("render = offset %d invalidated %v", offset, invalidated)
			}
			if got := tt.cell(surface); got.Ch != tt.glyph || got.Style != term.StyleScrollbarThumb {
				t.Fatalf("first track cell = %+v, want glyph %q thumb style", got, tt.glyph)
			}
			if offset, result := tt.bar.HandleEvent(tt.press); offset != 5 || result != widgets.EventCaptured || !tt.bar.OwnsPointerCapture() {
				t.Fatalf("endpoint press = offset %d result %v owns %v", offset, result, tt.bar.OwnsPointerCapture())
			}
			if _, result := tt.bar.HandleEvent(tcell.NewEventMouse(50, 50, tcell.ButtonNone, 0)); result != widgets.EventConsumed || tt.bar.OwnsPointerCapture() {
				t.Fatalf("owned release = result %v owns %v", result, tt.bar.OwnsPointerCapture())
			}

			tt.bar.Render(surface, tt.geometry, widgets.NewScrollRange(5, 10, 0))
			tt.bar.HandleEvent(tt.press)
			if !tt.bar.CancelPointerCapture() || tt.bar.OwnsPointerCapture() || tt.bar.CancelPointerCapture() {
				t.Fatal("cancellation was not stateful and idempotent")
			}
			tt.bar.Render(surface, tt.geometry, widgets.NewScrollRange(5, 5, 0))
			if _, result := tt.bar.HandleEvent(tt.press); result != widgets.EventIgnored || tt.bar.OwnsPointerCapture() {
				t.Fatalf("hidden press = result %v owns %v", result, tt.bar.OwnsPointerCapture())
			}
		})
	}
}
