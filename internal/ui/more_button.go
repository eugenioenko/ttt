package ui

import (
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

type MoreButtonWidget struct {
	BaseWidget
	OnClick   func(screenX, screenY int)
	pressedIn bool
}

func NewMoreButtonWidget() *MoreButtonWidget {
	return &MoreButtonWidget{}
}

func (m *MoreButtonWidget) Focusable() bool { return false }

func (m *MoreButtonWidget) Render(surface Surface) {
	surface.SetCell(0, 0, term.Cell{Ch: ' ', Style: term.StyleInactiveTab})
	surface.SetCell(1, 0, term.Cell{Ch: icons.Rune(icons.More), Style: term.StyleInactiveTab})
	surface.SetCell(2, 0, term.Cell{Ch: ' ', Style: term.StyleInactiveTab})
}

func (m *MoreButtonWidget) HandleEvent(ev tcell.Event) EventResult {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}
	r := m.GetRect()
	mx, my := mev.Position()
	inside := mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H

	btn := mev.Buttons()
	if btn&tcell.Button1 != 0 && inside && !m.pressedIn {
		m.pressedIn = true
		return EventConsumed
	}
	if btn == tcell.ButtonNone && m.pressedIn {
		m.pressedIn = false
		if inside && m.OnClick != nil {
			m.OnClick(r.X+1, r.Y+r.H)
		}
		return EventConsumed
	}
	return EventIgnored
}
