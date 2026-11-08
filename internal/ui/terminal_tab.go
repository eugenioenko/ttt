package ui

import (
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/xterm-go"
	"github.com/gdamore/tcell/v3"
)

// TerminalTabWidget hosts a terminal as the content of an editor tab.
type TerminalTabWidget struct {
	BaseWidget
	Terminal *TerminalWidget
	PaddingX int
	Icon     string
	// OnClose runs when the editor group discards the tab. It must release the
	// terminal without closing the tab again: the group is mid-close.
	OnClose func()
}

func (t *TerminalTabWidget) TabIcon(mode string) string {
	if t.Icon != "" {
		return t.Icon
	}
	return icons.Get(mode, icons.PanelApps)
}

func NewTerminalTabWidget(tw *TerminalWidget) *TerminalTabWidget {
	return &TerminalTabWidget{Terminal: tw, PaddingX: 1}
}

func (t *TerminalTabWidget) Focusable() bool { return true }

func (t *TerminalTabWidget) SetFocused(focused bool) { t.Terminal.SetFocused(focused) }

func (t *TerminalTabWidget) WantsRawKeys() bool { return t.Terminal.WantsRawKeys() }

func (t *TerminalTabWidget) OwnsPointerCapture() bool { return t.Terminal.OwnsPointerCapture() }

func (t *TerminalTabWidget) Close() {
	if t.OnClose != nil {
		t.OnClose()
	}
}

// ContentRect is the grid the PTY should be sized to, from the last layout.
func (t *TerminalTabWidget) ContentRect() Rect {
	return terminalContentRect(t.GetRect(), t.PaddingX, t.Terminal.ScrollbarVisible())
}

func (t *TerminalTabWidget) CursorPosition() (int, int, bool) {
	x, y, visible := t.Terminal.CursorPosition()
	return x, y, visible && t.Terminal.GetRect().Contains(x, y)
}

func (t *TerminalTabWidget) Render(surface Surface) {
	renderTerminalArea(surface, t.GetRect(), t.Terminal, t.PaddingX)
}

func (t *TerminalTabWidget) HandleEvent(ev tcell.Event) EventResult {
	if mev, ok := ev.(*tcell.EventMouse); ok && !t.Terminal.OwnsPointerCapture() {
		mx, my := mev.Position()
		if !t.GetRect().Contains(mx, my) {
			return EventIgnored
		}
		if mev.Buttons()&(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) == 0 {
			if result, stop := terminalMouseOutsideContent(t.Terminal, mev); stop {
				return result
			}
		}
	}
	return t.Terminal.HandleEvent(ev)
}

func (t *TerminalTabWidget) PointerShapeAt(mx, my int) string {
	if shape := t.Terminal.PointerShapeAt(mx, my); shape != "" {
		return shape
	}
	if t.GetRect().Contains(mx, my) {
		return term.PointerDefault
	}
	return ""
}

func terminalContentRect(area Rect, paddingX int, scrollbar bool) Rect {
	available := max(area.W, 0)
	if scrollbar {
		available = max(available-1, 0)
	}
	// Preserve xterm's minimum width; clamp before doubling to avoid overflow.
	padding := min(max(paddingX, 0), max(available-xterm.MinimumCols, 0)/2)
	return Rect{
		X: area.X + padding,
		Y: area.Y,
		W: available - 2*padding,
		H: max(area.H, 0),
	}
}

// renderTerminalArea draws tw with its padding and scrollbar into surface,
// which covers the screen rect area.
func renderTerminalArea(surface Surface, area Rect, tw *TerminalWidget, paddingX int) {
	if tw.Term != nil {
		tw.Term.AckUpdate()
	}
	showScrollbar := tw.ScrollbarVisible()
	content := terminalContentRect(area, paddingX, showScrollbar)

	blank := term.Cell{Ch: ' ', Style: term.StyleDefault}
	if tw.Palette != nil {
		blank.Direct = true
		blank.Fg = tw.Palette.Fg
		blank.Bg = tw.Palette.Bg
	}
	renderPaddedContent(surface, area, content, tw, blank)
	tw.scrollbar.Height = 0
	if showScrollbar && content.W > 0 && content.H > 0 {
		tw.renderScrollbar(surface.Sub(Rect{X: area.W - 1, W: 1, H: area.H}))
	}
}

func renderPaddedContent(surface Surface, area, content Rect, w Widget, blank term.Cell) {
	contentX := content.X - area.X
	surface.Sub(Rect{W: contentX, H: area.H}).Fill(blank)
	rightX := contentX + content.W
	surface.Sub(Rect{X: rightX, W: max(area.W-rightX, 0), H: area.H}).Fill(blank)
	w.SetRect(content)
	if content.W > 0 && content.H > 0 {
		w.Render(surface.Sub(Rect{X: contentX, W: content.W, H: content.H}))
	}
}

// terminalMouseOutsideContent settles a click that lands in the padding around
// w, so the terminal never sees coordinates outside its grid. stop reports
// whether the event was settled.
func terminalMouseOutsideContent(w Widget, mev *tcell.EventMouse) (result EventResult, stop bool) {
	mx, my := mev.Position()
	if tw, ok := w.(*TerminalWidget); ok && tw.scrollbar.Visible() && mx == tw.scrollbar.X {
		return EventIgnored, false
	}
	if w.GetRect().Contains(mx, my) {
		return EventIgnored, false
	}
	if mev.Buttons()&tcell.Button1 != 0 {
		return EventConsumed, true
	}
	return EventIgnored, true
}
