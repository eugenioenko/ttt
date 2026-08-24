package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

const hoverMaxVisibleLines = 12
const hoverMaxWidth = 68

type HoverWidget struct {
	BaseWidget
	md       *widgets.MarkdownWidget
	scroll   *widgets.ScrollViewWidget
	AnchorX  int
	AnchorY  int
	OffsetX  int
	OffsetY  int
	Borders  *term.BorderSet
	maxLineW int
	numLines int
}

func NewHoverWidget(text string, x, y int) *HoverWidget {
	md := widgets.NewMarkdownWidget()
	md.MaxWidth = hoverMaxWidth
	md.FillStyle = term.StylePaletteItem
	md.Box.PaddingLeft = 1
	md.Box.PaddingRight = 1
	md.SetContent(text)

	scroll := widgets.NewScrollViewWidget(md)
	md.SetScrollParent(scroll)

	maxW, lineCount := md.ContentSize(hoverMaxWidth)

	return &HoverWidget{
		md:       md,
		scroll:   scroll,
		AnchorX:  x,
		AnchorY:  y,
		maxLineW: maxW,
		numLines: lineCount,
	}
}

func (h *HoverWidget) HasContent() bool {
	return h.numLines > 0
}

func (h *HoverWidget) Focusable() bool { return false }

func (h *HoverWidget) visibleLines() int {
	v := h.numLines
	if v > hoverMaxVisibleLines {
		v = hoverMaxVisibleLines
	}
	return v
}

func (h *HoverWidget) Render(surface Surface) {
	if h.numLines == 0 {
		return
	}
	sw, sh := surface.Size()

	visLines := h.visibleLines()
	hasVScroll := h.numLines > visLines

	contentW := h.maxLineW + h.md.Box.PaddingLeft + h.md.Box.PaddingRight
	maxContentW := sw - 6
	if maxContentW < 20 {
		maxContentW = 20
	}
	if contentW > maxContentW {
		contentW = maxContentW
	}

	menuW := contentW + 2
	if hasVScroll {
		menuW++
	}
	menuH := visLines + 2

	localX := h.AnchorX - h.OffsetX
	localY := h.AnchorY - h.OffsetY

	x := localX
	if x+menuW > sw {
		x = sw - menuW
	}
	if x < 0 {
		x = 0
	}

	spaceAbove := localY
	y := localY - menuH
	if y < 0 {
		if spaceAbove < menuH {
			y = localY + 1
			if y+menuH > sh {
				menuH = sh - y
			}
		} else {
			y = 0
		}
	}

	b := term.SingleBorderSet()
	if h.Borders != nil {
		b = *h.Borders
	}
	surface.DrawBorder(x, y, menuW, menuH, b, term.StyleBorder)

	innerW := menuW - 2
	innerH := menuH - 2
	if innerW <= 0 || innerH <= 0 {
		h.SetRect(Rect{X: h.OffsetX + x, Y: h.OffsetY + y, W: menuW, H: menuH})
		return
	}

	sub := surface.Sub(Rect{X: x + 1, Y: y + 1, W: innerW, H: innerH})
	h.scroll.SetRect(Rect{X: h.OffsetX + x + 1, Y: h.OffsetY + y + 1, W: innerW, H: innerH})
	h.scroll.Render(sub)

	h.SetRect(Rect{X: h.OffsetX + x, Y: h.OffsetY + y, W: menuW, H: menuH})
}

func (h *HoverWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		if result := h.scroll.HandleEvent(ev); result == EventConsumed {
			return EventConsumed
		}
		return EventDismissed
	case *tcell.EventMouse:
		mx, my := tev.Position()
		r := h.GetRect()
		inside := mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H
		btn := tev.Buttons()
		if inside {
			return h.scroll.HandleEvent(ev)
		}
		if btn != tcell.ButtonNone {
			return EventDismissed
		}
	}
	return EventIgnored
}

func (h *HoverWidget) IsDragging() bool {
	return false
}
