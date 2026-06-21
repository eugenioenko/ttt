package ui

import (
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

type ContextMenuItem struct {
	Label    string
	Shortcut string
	Command  string
	IsSep    bool
	Checked  int // 0 = no indicator, 1 = unchecked, 2 = checked
}

const (
	MenuUnchecked = 1
	MenuChecked   = 2
)

func MenuSep() ContextMenuItem {
	return ContextMenuItem{IsSep: true}
}

type ContextMenuWidget struct {
	BaseWidget
	Items      []ContextMenuItem
	Selected   int
	AnchorX    int
	AnchorY    int
	Borders    *term.BorderSet
	OnExec     func(command string)
	OnDismiss  func()
	OnNavigate func(dir int)
	firstEvent bool
}

func NewContextMenuWidget(items []ContextMenuItem, x, y int) *ContextMenuWidget {
	sel := 0
	for i, it := range items {
		if !it.IsSep {
			sel = i
			break
		}
	}
	return &ContextMenuWidget{
		Items:      items,
		Selected:   sel,
		AnchorX:    x,
		AnchorY:    y,
		firstEvent: true,
	}
}

func (c *ContextMenuWidget) Focusable() bool { return true }

func (c *ContextMenuWidget) hasCheckedItems() bool {
	for _, it := range c.Items {
		if it.Checked != 0 {
			return true
		}
	}
	return false
}

func (c *ContextMenuWidget) menuWidth() int {
	maxLabel := 0
	maxShort := 0
	for _, it := range c.Items {
		if it.IsSep {
			continue
		}
		lr := len([]rune(it.Label))
		if lr > maxLabel {
			maxLabel = lr
		}
		sr := len([]rune(it.Shortcut))
		if sr > maxShort {
			maxShort = sr
		}
	}
	w := maxLabel + 6
	if c.hasCheckedItems() {
		w += 2
	}
	if maxShort > 0 {
		w += maxShort + 2
	}
	if w < 15 {
		w = 15
	}
	return w
}

func (c *ContextMenuWidget) Render(surface *RenderSurface) {
	sw, sh := surface.Size()

	menuW := c.menuWidth()
	menuH := len(c.Items) + 2

	x := c.AnchorX
	y := c.AnchorY
	if x+menuW > sw {
		x = sw - menuW
	}
	if x < 0 {
		x = 0
	}
	if y+menuH > sh {
		y = sh - menuH
	}
	if y < 0 {
		y = 0
	}

	b := term.SingleBorderSet()
	if c.Borders != nil {
		b = *c.Borders
	}
	bs := term.StyleBorder
	surface.DrawBorder(x, y, menuW, menuH, b, bs)

	for i, it := range c.Items {
		row := y + 1 + i
		if it.IsSep {
			for bx := x + 1; bx < x+menuW-1; bx++ {
				surface.SetCell(bx, row, term.Cell{Ch: b.Horizontal, Style: bs})
			}
			continue
		}

		style := term.StylePaletteItem
		if i == c.Selected {
			style = term.StylePaletteSelected
		}

		surface.ClearRect(x+1, row, menuW-2, 1, style)
		labelX := x + 2
		if c.hasCheckedItems() {
			if it.Checked == MenuChecked {
				surface.SetCell(x+1, row, term.Cell{Ch: '✓', Style: style})
			}
			labelX = x + 3
		}
		surface.DrawText(labelX, row, it.Label, x+menuW-1, style)

		if it.Shortcut != "" {
			shortStyle := term.StyleMuted
			if i == c.Selected {
				shortStyle = style
			}
			shortRunes := []rune(it.Shortcut)
			sx := x + menuW - 2 - len(shortRunes)
			surface.DrawText(sx, row, it.Shortcut, x+menuW-1, shortStyle)
		}
	}

	c.storeRect(x, y, menuW, menuH)
}

func (c *ContextMenuWidget) storeRect(x, y, w, h int) {
	c.SetRect(Rect{X: x, Y: y, W: w, H: h})
}

func (c *ContextMenuWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		switch tev.Key() {
		case tcell.KeyEscape:
			if c.OnDismiss != nil {
				c.OnDismiss()
			}
			return EventConsumed
		case tcell.KeyUp:
			c.moveSelection(-1)
			return EventConsumed
		case tcell.KeyDown:
			c.moveSelection(1)
			return EventConsumed
		case tcell.KeyLeft:
			if c.OnNavigate != nil {
				c.OnNavigate(-1)
			}
			return EventConsumed
		case tcell.KeyRight:
			if c.OnNavigate != nil {
				c.OnNavigate(1)
			}
			return EventConsumed
		case tcell.KeyEnter:
			if c.Selected >= 0 && c.Selected < len(c.Items) && !c.Items[c.Selected].IsSep {
				if c.OnExec != nil {
					c.OnExec(c.Items[c.Selected].Command)
				}
			}
			return EventConsumed
		}
	case *tcell.EventMouse:
		btn := tev.Buttons()
		mx, my := tev.Position()
		r := c.GetRect()

		if c.firstEvent {
			c.firstEvent = false
			return EventConsumed
		}

		if btn&tcell.Button1 != 0 {
			if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
				if c.OnDismiss != nil {
					c.OnDismiss()
				}
				return EventConsumed
			}
			itemIdx := my - r.Y - 1
			if itemIdx >= 0 && itemIdx < len(c.Items) && !c.Items[itemIdx].IsSep {
				c.Selected = itemIdx
				if c.OnExec != nil {
					c.OnExec(c.Items[c.Selected].Command)
				}
			}
			return EventConsumed
		}

		if btn == tcell.ButtonNone {
			if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
				c.Selected = -1
				return EventConsumed
			}
			itemIdx := my - r.Y - 1
			if itemIdx >= 0 && itemIdx < len(c.Items) && !c.Items[itemIdx].IsSep {
				c.Selected = itemIdx
			}
			return EventConsumed
		}
	}
	return EventConsumed
}

func (c *ContextMenuWidget) moveSelection(dir int) {
	n := len(c.Items)
	if n == 0 {
		return
	}
	next := c.Selected
	for i := 0; i < n; i++ {
		next = (next + dir + n) % n
		if !c.Items[next].IsSep {
			c.Selected = next
			return
		}
	}
}
