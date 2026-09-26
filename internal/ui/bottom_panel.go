package ui

import (
	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/term"
)

type BottomPanelWidget struct {
	BaseWidget
	TabbedPanel
	Visible bool
	Borders *term.BorderSet
}

func NewBottomPanelWidget(borders *term.BorderSet) *BottomPanelWidget {
	bp := &BottomPanelWidget{
		TabbedPanel: NewTabbedPanel(),
		Visible:     true,
		Borders:     borders,
	}
	bp.InitTabClick()
	return bp
}

func (bp *BottomPanelWidget) Focusable() bool { return true }

func (bp *BottomPanelWidget) Render(surface Surface) {
	w, h := surface.Size()
	r := bp.GetRect()

	if h < 3 {
		return
	}

	bp.RenderTabs(surface, Rect{X: r.X, Y: r.Y, W: r.W, H: 1})
	bp.RenderDivider(surface, 1, w, bp.Borders)

	contentH := h - 2
	active := bp.ActiveWidget()
	if active != nil && contentH > 0 {
		active.SetRect(Rect{X: r.X, Y: r.Y + 2, W: r.W, H: contentH})
		contentSurface := surface.Sub(Rect{X: 0, Y: 2, W: w, H: contentH})
		active.Render(contentSurface)
		if divider, ok := active.(dividerXProvider); ok {
			b := term.SingleBorderSet()
			if bp.Borders != nil {
				b = *bp.Borders
			}
			x := divider.DividerScreenX() - r.X
			if x >= 0 && x < w {
				surface.SetCell(x, 1, term.Cell{Ch: b.TopTee, Style: term.StyleBorder})
			}
		}
	}
}

func (bp *BottomPanelWidget) DividerScreenX() int {
	if divider, ok := bp.ActiveWidget().(dividerXProvider); ok {
		return divider.DividerScreenX()
	}
	return -1
}

func (bp *BottomPanelWidget) DividerScreenY() int {
	r := bp.GetRect()
	if r.H < 3 {
		return -1
	}
	return r.Y + 1
}

func (bp *BottomPanelWidget) HandleEvent(ev tcell.Event) EventResult {
	if mev, ok := ev.(*tcell.EventMouse); ok {
		_, my := mev.Position()
		r := bp.GetRect()
		if my == r.Y {
			if bp.Tabs.HandleEvent(ev) == EventConsumed {
				return EventConsumed
			}
			return EventIgnored
		}
	}
	active := bp.ActiveWidget()
	if active != nil {
		return active.HandleEvent(ev)
	}
	return EventIgnored
}
