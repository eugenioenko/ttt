package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type DialogButton struct {
	Label   string
	Handler func()
}

type DialogWidget struct {
	BaseWidget
	Title     string
	Content   Widget
	Buttons   []DialogButton
	BoxWidth  int
	Borders   term.BorderSet
	OnDismiss func()

	boxX   int
	boxY   int
	boxW   int
	boxH   int
	footer *HStackWidget
}

func (d *DialogWidget) WidgetChildren() []Widget {
	var out []Widget
	if d.Content != nil {
		out = append(out, d.Content)
	}
	if d.footer != nil {
		out = append(out, d.footer)
	}
	return out
}

func NewDialogWidget(width int) *DialogWidget {
	d := &DialogWidget{
		BoxWidth: width,
		Borders:  term.DoubleBorderSet(),
	}
	if d.BoxWidth <= 0 {
		d.BoxWidth = 40
	}
	return d
}

func (d *DialogWidget) SetContent(w Widget) {
	d.Content = w
}

func (d *DialogWidget) Build() {
	if len(d.Buttons) > 0 {
		children := make([]Widget, len(d.Buttons))
		for i, btn := range d.Buttons {
			handler := btn.Handler
			children[i] = NewButtonWidget(ButtonConfig{
				Label:   btn.Label,
				OnClick: handler,
			})
		}
		d.footer = NewHStackWidget(children...)
		d.footer.Align = "right"
		d.footer.Gap = 1
		d.footer.Box.PaddingLeft = 1
		d.footer.Box.PaddingRight = 1
	}
}

func (d *DialogWidget) Height() int { return 0 }
func (d *DialogWidget) Width() int  { return d.BoxWidth }

func (d *DialogWidget) contentHeight(availW int) int {
	if d.Content != nil {
		if hfw, ok := d.Content.(HeightForWidther); ok {
			h := hfw.HeightForWidth(availW)
			if h > 0 {
				return h
			}
		}
		h := d.Content.Height()
		if h > 0 {
			return h
		}
	}
	return 1
}

func (d *DialogWidget) Render(surface Surface) {
	sw, sh := surface.Size()
	if sw <= 4 || sh <= 4 {
		return
	}

	d.boxW = d.BoxWidth
	if d.boxW > sw-4 {
		d.boxW = sw - 4
	}

	titleH := 0
	if d.Title != "" {
		titleH = 1
	}
	contentH := d.contentHeight(d.boxW - 4)
	btnH := 0
	if d.footer != nil {
		btnH = 1
	}

	gaps := 0
	if titleH > 0 && contentH > 0 {
		gaps++
	}
	if contentH > 0 && btnH > 0 {
		gaps++
	}

	d.boxH = 2 + titleH + gaps + contentH + btnH
	d.boxX = (sw - d.boxW) / 2
	d.boxY = 2

	if d.boxY+d.boxH > sh {
		contentH = max(0, contentH-(d.boxY+d.boxH-sh))
		d.boxH = sh - d.boxY
	}

	surface.ClearRect(d.boxX, d.boxY, d.boxW, d.boxH, term.StylePaletteItem)
	surface.DrawBorder(d.boxX, d.boxY, d.boxW, d.boxH, d.Borders, term.StyleBorder)

	innerX := d.boxX + 1
	innerW := d.boxW - 2
	y := d.boxY + 1

	if d.Title != "" {
		tx := innerX + 1
		for _, ch := range d.Title {
			if tx >= innerX+innerW-1 {
				break
			}
			surface.SetCell(tx, y, term.Cell{Ch: ch, Style: term.StylePaletteItem, Bold: true})
			tx++
		}
		y += titleH
	}

	if d.Content != nil && contentH > 0 {
		if d.Title != "" {
			y++
		}
		cx := innerX + 1
		cw := innerW - 2
		d.Content.SetRect(Rect{X: d.boxX + 2, Y: y, W: cw, H: contentH})
		contentSurface := surface.Sub(Rect{X: cx, Y: y, W: cw, H: contentH})
		d.Content.Render(contentSurface)
		y += contentH
		if d.footer != nil {
			y++
		}
	}

	if d.footer != nil {
		d.footer.SetRect(Rect{X: innerX, Y: y, W: innerW, H: 1})
		footerSurface := surface.Sub(Rect{X: innerX, Y: y, W: innerW, H: 1})
		d.footer.Render(footerSurface)
	}
}

func (d *DialogWidget) moveFocusButton(forward bool) {
	children := d.footer.Children
	if len(children) == 0 {
		return
	}
	cur := -1
	for i, c := range children {
		if fw, ok := c.(FocusableWidget); ok && fw.IsFocused() {
			cur = i
			break
		}
	}
	if cur < 0 {
		cur = 0
	}
	next := cur
	if forward && cur < len(children)-1 {
		next = cur + 1
	} else if !forward && cur > 0 {
		next = cur - 1
	}
	if next != cur {
		if fw, ok := children[cur].(FocusableWidget); ok {
			fw.SetFocused(false)
		}
		if fw, ok := children[next].(FocusableWidget); ok {
			fw.SetFocused(true)
		}
	}
}

func (d *DialogWidget) HandleEvent(ev tcell.Event) EventResult {
	switch e := ev.(type) {
	case *tcell.EventKey:
		if e.Key() == tcell.KeyEscape {
			if d.OnDismiss != nil {
				d.OnDismiss()
			}
			return EventConsumed
		}
		if d.footer != nil && (e.Key() == tcell.KeyLeft || e.Key() == tcell.KeyRight) {
			d.moveFocusButton(e.Key() == tcell.KeyRight)
			return EventConsumed
		}
		if d.footer != nil {
			if d.footer.HandleEvent(e) == EventConsumed {
				return EventConsumed
			}
		}
		if d.Content != nil {
			d.Content.HandleEvent(e)
		}
	case *tcell.EventMouse:
		if d.footer != nil {
			d.footer.HandleEvent(e)
		}
		if d.Content != nil {
			d.Content.HandleEvent(e)
		}
	}
	return EventConsumed
}
