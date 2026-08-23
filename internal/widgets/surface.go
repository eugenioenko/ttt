package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type Rect struct {
	X, Y, W, H int
}

type EventResult int

const (
	EventIgnored EventResult = iota
	EventConsumed
	EventDismissed
	EventCaptured
)

type Widget interface {
	Height() int
	Width() int
	SetRect(r Rect)
	GetRect() Rect
	Render(surface Surface)
	HandleEvent(ev tcell.Event) EventResult
	SetBoxModel(bm BoxModel)
}

type FocusableWidget interface {
	Widget
	Focusable() bool
	SetFocused(focused bool)
	IsFocused() bool
}

type ContainerWidget interface {
	WidgetChildren() []Widget
}

type CursorPositioner interface {
	CursorPosition() (x, y int, visible bool)
}

type PointerCaptureCanceler interface {
	CancelPointerCapture() bool
}

type PointerCaptureOwner interface {
	OwnsPointerCapture() bool
}

type PointerInteractionInvalidator interface {
	InvalidatePointerInteraction() bool
}

type PointerCaptureInvalidationSetter interface {
	SetPointerCaptureInvalidated(func())
}

func CancelPointerCapture(w Widget) bool {
	if canceler, ok := w.(PointerCaptureCanceler); ok {
		return canceler.CancelPointerCapture()
	}
	return false
}

func InvalidatePointerInteraction(w Widget) bool {
	if invalidator, ok := w.(PointerInteractionInvalidator); ok {
		return invalidator.InvalidatePointerInteraction()
	}
	return CancelPointerCapture(w)
}

func SetPointerCaptureInvalidated(w Widget, invalidated func()) {
	if setter, ok := w.(PointerCaptureInvalidationSetter); ok {
		setter.SetPointerCaptureInvalidated(invalidated)
	}
}

func hasFocusedChild(w Widget) bool {
	if fw, ok := w.(FocusableWidget); ok && fw.IsFocused() {
		return true
	}
	if cw, ok := w.(ContainerWidget); ok {
		for _, child := range cw.WidgetChildren() {
			if hasFocusedChild(child) {
				return true
			}
		}
	}
	return false
}

type PopupRenderer interface {
	HasPopup() bool
	PopupRect() Rect
	RenderPopup(surface Surface)
}

// PopupBounder is implemented by widgets whose popup should be constrained to a
// container's rect. Containers that render popups call it before PopupRect.
type PopupBounder interface {
	SetPopupBounds(r Rect)
}

// widgetBorders returns the widget's configured border set, falling back to the
// rounded default.
func widgetBorders(b BoxModel) term.BorderSet {
	if b.Borders.Horizontal != 0 {
		return b.Borders
	}
	return term.RoundedBorderSet()
}

type HeightForWidther interface {
	HeightForWidth(w int) int
}

// ContentHeighter reports the natural content height of a grow widget inside a scroll view.
type ContentHeighter interface {
	ContentHeight() int
}

type ScrollableWidget interface {
	Widget
	ScrollSize() (w, h int)
}

type Surface interface {
	Size() (w, h int)
	Origin() (x, y int)
	SetCell(x, y int, c term.Cell)
	DrawText(x, y int, text string, maxW int, style term.Style) int
	DrawBorder(x, y, w, h int, b term.BorderSet, style term.Style)
	ClearRect(x, y, w, h int, style term.Style)
	Fill(c term.Cell)
	Sub(r Rect) Surface
}

type BoxModel struct {
	BorderTop    bool `json:"borderTop,omitempty"`
	BorderBottom bool `json:"borderBottom,omitempty"`
	BorderLeft   bool `json:"borderLeft,omitempty"`
	BorderRight  bool `json:"borderRight,omitempty"`

	PaddingTop    int `json:"paddingTop,omitempty"`
	PaddingBottom int `json:"paddingBottom,omitempty"`
	PaddingLeft   int `json:"paddingLeft,omitempty"`
	PaddingRight  int `json:"paddingRight,omitempty"`

	MarginTop    int `json:"marginTop,omitempty"`
	MarginBottom int `json:"marginBottom,omitempty"`
	MarginLeft   int `json:"marginLeft,omitempty"`
	MarginRight  int `json:"marginRight,omitempty"`

	Borders term.BorderSet `json:"-"`
	Style   term.Style     `json:"-"`
}

type BaseWidget struct {
	Box  BoxModel
	rect Rect
}

func (b *BaseWidget) SetRect(r Rect)          { b.rect = r }
func (b *BaseWidget) GetRect() Rect           { return b.rect }
func (b *BaseWidget) SetBoxModel(bm BoxModel) { b.Box = bm }

func (b *BaseWidget) contentOrigin() (int, int) {
	x := b.rect.X + b.Box.MarginLeft + b.Box.PaddingLeft
	y := b.rect.Y + b.Box.MarginTop + b.Box.PaddingTop
	if b.Box.BorderLeft {
		x++
	}
	if b.Box.BorderTop {
		y++
	}
	return x, y
}

func (b *BaseWidget) BoxOverheadH() int {
	h := b.Box.MarginTop + b.Box.MarginBottom + b.Box.PaddingTop + b.Box.PaddingBottom
	if b.Box.BorderTop {
		h++
	}
	if b.Box.BorderBottom {
		h++
	}
	return h
}

func (b *BaseWidget) BoxOverheadW() int {
	w := b.Box.MarginLeft + b.Box.MarginRight + b.Box.PaddingLeft + b.Box.PaddingRight
	if b.Box.BorderLeft {
		w++
	}
	if b.Box.BorderRight {
		w++
	}
	return w
}

func (b *BaseWidget) RenderBox(surface Surface) Surface {
	w, h := surface.Size()

	borderStyle := b.Box.Style
	if borderStyle == 0 {
		borderStyle = term.StyleBorder
	}
	bs := b.Box.Borders

	mx := b.Box.MarginLeft
	my := b.Box.MarginTop
	mw := w - b.Box.MarginLeft - b.Box.MarginRight
	mh := h - b.Box.MarginTop - b.Box.MarginBottom
	if mw <= 0 || mh <= 0 {
		return surface.Sub(Rect{X: 0, Y: 0, W: 0, H: 0})
	}

	bTop, bBottom, bLeft, bRight := 0, 0, 0, 0
	if b.Box.BorderTop {
		bTop = 1
	}
	if b.Box.BorderBottom {
		bBottom = 1
	}
	if b.Box.BorderLeft {
		bLeft = 1
	}
	if b.Box.BorderRight {
		bRight = 1
	}

	if b.Box.BorderTop {
		for x := mx + bLeft; x < mx+mw-bRight; x++ {
			surface.SetCell(x, my, term.Cell{Ch: bs.Horizontal, Style: borderStyle})
		}
	}
	if b.Box.BorderBottom {
		for x := mx + bLeft; x < mx+mw-bRight; x++ {
			surface.SetCell(x, my+mh-1, term.Cell{Ch: bs.Horizontal, Style: borderStyle})
		}
	}
	if b.Box.BorderLeft {
		for y := my + bTop; y < my+mh-bBottom; y++ {
			surface.SetCell(mx, y, term.Cell{Ch: bs.Vertical, Style: borderStyle})
		}
	}
	if b.Box.BorderRight {
		for y := my + bTop; y < my+mh-bBottom; y++ {
			surface.SetCell(mx+mw-1, y, term.Cell{Ch: bs.Vertical, Style: borderStyle})
		}
	}

	if b.Box.BorderTop && b.Box.BorderLeft {
		surface.SetCell(mx, my, term.Cell{Ch: bs.TopLeft, Style: borderStyle})
	}
	if b.Box.BorderTop && b.Box.BorderRight {
		surface.SetCell(mx+mw-1, my, term.Cell{Ch: bs.TopRight, Style: borderStyle})
	}
	if b.Box.BorderBottom && b.Box.BorderLeft {
		surface.SetCell(mx, my+mh-1, term.Cell{Ch: bs.BottomLeft, Style: borderStyle})
	}
	if b.Box.BorderBottom && b.Box.BorderRight {
		surface.SetCell(mx+mw-1, my+mh-1, term.Cell{Ch: bs.BottomRight, Style: borderStyle})
	}

	paddedX := mx + bLeft
	paddedY := my + bTop
	paddedW := mw - bLeft - bRight
	paddedH := mh - bTop - bBottom

	if paddedW <= 0 || paddedH <= 0 {
		return surface.Sub(Rect{X: 0, Y: 0, W: 0, H: 0})
	}

	innerX := paddedX + b.Box.PaddingLeft
	innerY := paddedY + b.Box.PaddingTop
	innerW := paddedW - b.Box.PaddingLeft - b.Box.PaddingRight
	innerH := paddedH - b.Box.PaddingTop - b.Box.PaddingBottom

	if innerW <= 0 || innerH <= 0 {
		return surface.Sub(Rect{X: 0, Y: 0, W: 0, H: 0})
	}
	return surface.Sub(Rect{X: innerX, Y: innerY, W: innerW, H: innerH})
}

func (b *BaseWidget) BorderedInterior(surface Surface) Surface {
	w, h := surface.Size()
	mx := b.Box.MarginLeft
	my := b.Box.MarginTop
	mw := w - b.Box.MarginLeft - b.Box.MarginRight
	mh := h - b.Box.MarginTop - b.Box.MarginBottom
	if mw <= 0 || mh <= 0 {
		return surface.Sub(Rect{X: 0, Y: 0, W: 0, H: 0})
	}
	bTop, bLeft, bRight, bBottom := 0, 0, 0, 0
	if b.Box.BorderTop {
		bTop = 1
	}
	if b.Box.BorderBottom {
		bBottom = 1
	}
	if b.Box.BorderLeft {
		bLeft = 1
	}
	if b.Box.BorderRight {
		bRight = 1
	}
	px := mx + bLeft
	py := my + bTop
	pw := mw - bLeft - bRight
	ph := mh - bTop - bBottom
	if pw <= 0 || ph <= 0 {
		return surface.Sub(Rect{X: 0, Y: 0, W: 0, H: 0})
	}
	return surface.Sub(Rect{X: px, Y: py, W: pw, H: ph})
}
