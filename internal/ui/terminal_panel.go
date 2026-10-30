package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/xterm-go"
	"github.com/gdamore/tcell/v3"
)

type TerminalPanelWidget struct {
	BaseWidget
	TabBar  *VerticalTabBar
	widgets []Widget
	labels  []string
	active  int
	Borders *term.BorderSet
	Padding int
}

func NewTerminalPanelWidget() *TerminalPanelWidget {
	tp := &TerminalPanelWidget{
		TabBar:  NewVerticalTabBar(),
		Padding: 1,
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
			x, y, visible := cp.CursorPosition()
			r := w.GetRect()
			return x, y, visible && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
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
	tp.AddTerminalLabeled(w, "")
}

// AddTerminalLabeled adds a terminal with an optional short badge in the strip.
func (tp *TerminalPanelWidget) AddTerminalLabeled(w Widget, label string) {
	tp.blurActive()
	tp.widgets = append(tp.widgets, w)
	tp.labels = append(tp.labels, label)
	tp.active = len(tp.widgets) - 1
	tp.focusActive()
	tp.syncTabBar()
}

func (tp *TerminalPanelWidget) RemoveTerminal(index int) {
	if index < 0 || index >= len(tp.widgets) {
		return
	}
	tp.widgets = append(tp.widgets[:index], tp.widgets[index+1:]...)
	if index < len(tp.labels) {
		tp.labels = append(tp.labels[:index], tp.labels[index+1:]...)
	}
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
	if len(tp.widgets) == 0 || r.W <= verticalTabBarBorderX {
		return -1
	}
	return r.X + verticalTabBarBorderX
}

func (tp *TerminalPanelWidget) ContentRect(r Rect) Rect {
	available := max(r.W-VerticalTabBarWidth, 0)
	// Preserve xterm's minimum width; clamp before doubling to avoid overflow.
	padding := min(max(tp.Padding, 0), max(available-xterm.MinimumCols, 0)/2)
	return Rect{
		X: r.X + min(VerticalTabBarWidth, max(r.W, 0)) + padding,
		Y: r.Y,
		W: available - 2*padding,
		H: max(r.H, 0),
	}
}

func (tp *TerminalPanelWidget) syncTabBar() {
	tp.TabBar.Count = len(tp.widgets)
	tp.TabBar.Active = tp.active
	tp.TabBar.Labels = tp.labels
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

	stripW := min(VerticalTabBarWidth, max(w, 0))
	content := tp.ContentRect(Rect{X: r.X, Y: r.Y, W: w, H: h})

	tp.TabBar.Borders = tp.Borders
	tp.TabBar.SetRect(Rect{X: r.X, Y: r.Y, W: stripW, H: h})
	tp.TabBar.Render(surface.Sub(Rect{X: 0, Y: 0, W: stripW, H: h}))
	contentX := content.X - r.X
	blank := term.Cell{Ch: ' ', Style: term.StyleDefault}
	surface.Sub(Rect{X: stripW, W: contentX - stripW, H: h}).Fill(blank)
	rightX := contentX + content.W
	surface.Sub(Rect{X: rightX, W: max(w-rightX, 0), H: h}).Fill(blank)

	active := tp.widgets[tp.active]
	active.SetRect(content)
	if content.W > 0 && content.H > 0 {
		active.Render(surface.Sub(Rect{X: content.X - r.X, Y: 0, W: content.W, H: content.H}))
	}
}

func (tp *TerminalPanelWidget) HandleEvent(ev tcell.Event) EventResult {
	if mev, ok := ev.(*tcell.EventMouse); ok {
		if !tp.activeOwnsPointer() {
			mx, my := mev.Position()
			r := tp.GetRect()
			if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
				return EventIgnored
			}
			wheel := mev.Buttons()&(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) != 0
			if wheel {
				if w := tp.ActiveWidget(); w != nil {
					return w.HandleEvent(ev)
				}
				return EventIgnored
			}
			r = tp.TabBar.GetRect()
			if mev.Buttons()&tcell.Button1 != 0 && mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H {
				return tp.TabBar.HandleEvent(ev)
			}
			w := tp.ActiveWidget()
			if w == nil {
				return EventIgnored
			}
			r = w.GetRect()
			if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
				if mev.Buttons()&tcell.Button1 != 0 {
					return EventConsumed
				}
				return EventIgnored
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
