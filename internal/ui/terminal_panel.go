package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type TerminalPanelWidget struct {
	BaseWidget
	TabBar  *VerticalTabBar
	widgets []Widget
	active  int
	Borders *term.BorderSet
}

func NewTerminalPanelWidget() *TerminalPanelWidget {
	tp := &TerminalPanelWidget{
		TabBar: NewVerticalTabBar(),
	}
	tp.TabBar.OnSelect = func(index int) {
		tp.SetActive(index)
	}
	return tp
}

func (tp *TerminalPanelWidget) Focusable() bool { return true }

func (tp *TerminalPanelWidget) CursorPosition() (int, int, bool) {
	if w := tp.ActiveWidget(); w != nil {
		if cp, ok := w.(CursorProvider); ok {
			return cp.CursorPosition()
		}
	}
	return 0, 0, false
}

func (tp *TerminalPanelWidget) WantsRawKeys() bool {
	if w := tp.ActiveWidget(); w != nil {
		if rk, ok := w.(RawKeyConsumer); ok {
			return rk.WantsRawKeys()
		}
	}
	return false
}

func (tp *TerminalPanelWidget) SetFocused(focused bool) {
	if w := tp.ActiveWidget(); w != nil {
		if setter, ok := w.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(focused)
		}
	}
}

// AddTerminal and RemoveTerminal focus the widget they make active, as SetActive
// does. Leaving that to a following Root.SetFocus only worked while SetFocus
// blindly re-asserted focus on the already-focused panel.
func (tp *TerminalPanelWidget) AddTerminal(w Widget) {
	tp.blurActive()
	tp.widgets = append(tp.widgets, w)
	tp.active = len(tp.widgets) - 1
	tp.focusActive()
	tp.syncTabBar()
}

func (tp *TerminalPanelWidget) RemoveTerminal(index int) {
	if index < 0 || index >= len(tp.widgets) {
		return
	}
	tp.widgets = append(tp.widgets[:index], tp.widgets[index+1:]...)
	if tp.active >= len(tp.widgets) {
		tp.active = len(tp.widgets) - 1
	}
	if tp.active < 0 {
		tp.active = 0
	}
	tp.focusActive()
	tp.syncTabBar()
}

func (tp *TerminalPanelWidget) SetActive(index int) {
	if index >= 0 && index < len(tp.widgets) && tp.active != index {
		tp.blurActive()
		tp.active = index
		tp.focusActive()
		tp.syncTabBar()
	}
}

func (tp *TerminalPanelWidget) blurActive()  { tp.setActiveFocus(false) }
func (tp *TerminalPanelWidget) focusActive() { tp.setActiveFocus(true) }

func (tp *TerminalPanelWidget) setActiveFocus(focused bool) {
	if w := tp.ActiveWidget(); w != nil {
		if setter, ok := w.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(focused)
		}
	}
}

func (tp *TerminalPanelWidget) ActiveIndex() int {
	return tp.active
}

func (tp *TerminalPanelWidget) ActiveWidget() Widget {
	if tp.active >= 0 && tp.active < len(tp.widgets) {
		return tp.widgets[tp.active]
	}
	return nil
}

func (tp *TerminalPanelWidget) Count() int {
	return len(tp.widgets)
}

func (tp *TerminalPanelWidget) DividerScreenX() int {
	r := tp.GetRect()
	if len(tp.widgets) == 0 || r.W <= VerticalTabBarWidth {
		return -1
	}
	return r.X + VerticalTabBarWidth - 2
}

func (tp *TerminalPanelWidget) syncTabBar() {
	tp.TabBar.Count = len(tp.widgets)
	tp.TabBar.Active = tp.active
}

func (tp *TerminalPanelWidget) Render(surface Surface) {
	w, h := surface.Size()
	r := tp.GetRect()

	if len(tp.widgets) == 0 {
		msg := "No terminals. Press + to create one."
		x := 1
		for _, ch := range msg {
			if x >= w {
				break
			}
			surface.SetCell(x, 0, term.Cell{Ch: ch, Style: term.StyleMuted})
			x++
		}
		return
	}

	stripW := VerticalTabBarWidth
	contentW := w - stripW
	if contentW <= 0 {
		return
	}

	tp.TabBar.Borders = tp.Borders
	tp.TabBar.SetRect(Rect{X: r.X, Y: r.Y, W: stripW, H: h})
	tp.TabBar.Render(surface.Sub(Rect{X: 0, Y: 0, W: stripW, H: h}))

	active := tp.widgets[tp.active]
	active.SetRect(Rect{X: r.X + stripW, Y: r.Y, W: contentW, H: h})
	active.Render(surface.Sub(Rect{X: stripW, Y: 0, W: contentW, H: h}))
}

func (tp *TerminalPanelWidget) HandleEvent(ev tcell.Event) EventResult {
	if mev, ok := ev.(*tcell.EventMouse); ok {
		if mev.Buttons()&tcell.Button1 != 0 && !tp.activeOwnsPointer() {
			mx, _ := mev.Position()
			r := tp.GetRect()
			if mx-r.X < VerticalTabBarWidth {
				return tp.TabBar.HandleEvent(ev)
			}
		}
	}

	if w := tp.ActiveWidget(); w != nil {
		return w.HandleEvent(ev)
	}
	return EventIgnored
}

func (tp *TerminalPanelWidget) activeOwnsPointer() bool {
	owner, ok := tp.ActiveWidget().(widgets.PointerCaptureOwner)
	return ok && owner.OwnsPointerCapture()
}
