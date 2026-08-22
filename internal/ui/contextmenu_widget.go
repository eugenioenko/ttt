package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"

	"github.com/gdamore/tcell/v3"
)

type ContextMenuItem struct {
	Label    string
	Shortcut string
	Command  string
	IsSep    bool
	Checked  int // 0 = no indicator, 1 = unchecked, 2 = checked
	Submenu  []ContextMenuItem
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
	Items          []ContextMenuItem
	Selected       int
	AnchorX        int
	AnchorY        int
	Borders        *term.BorderSet
	OnExec         func(command string)
	OnDismiss      func()
	OnNavigate     func(dir int)
	OnMouseOutside func(ev tcell.Event)
	Submenu        *ContextMenuWidget
	parent         *ContextMenuWidget
	submenuIndex   int
	firstEvent     bool
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

func (c *ContextMenuWidget) hasSubmenus() bool {
	for _, item := range c.Items {
		if len(item.Submenu) > 0 {
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
		lr := textwidth.String(it.Label)
		if lr > maxLabel {
			maxLabel = lr
		}
		sr := textwidth.String(it.Shortcut)
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
	if c.hasSubmenus() {
		w += 2
	}
	if w < 15 {
		w = 15
	}
	return w
}

func (c *ContextMenuWidget) Render(surface Surface) {
	sw, sh := surface.Size()
	c.renderAt(surface, c.AnchorX, c.AnchorY, sw, sh)
}

func (c *ContextMenuWidget) renderAt(surface Surface, x, y, sw, sh int) {
	if sw <= 0 || sh <= 0 {
		c.storeRect(0, 0, 0, 0)
		return
	}

	menuW := c.menuWidth()
	menuH := len(c.Items) + 2
	if menuW > sw {
		menuW = sw
	}
	if menuH > sh {
		menuH = sh
	}

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
	c.storeRect(x, y, menuW, menuH)
	if menuW < 2 || menuH < 2 {
		return
	}

	b := term.SingleBorderSet()
	if c.Borders != nil {
		b = *c.Borders
	}
	bs := term.StyleBorder
	surface.DrawBorder(x, y, menuW, menuH, b, bs)

	for i, it := range c.Items {
		row := y + 1 + i
		if row >= y+menuH-1 {
			break
		}
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
		if it.Checked != 0 {
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
			sx := x + menuW - 2 - textwidth.String(it.Shortcut)
			surface.DrawText(sx, row, it.Shortcut, x+menuW-1, shortStyle)
		}
		if len(it.Submenu) > 0 && menuW >= 3 {
			surface.SetCell(x+menuW-2, row, term.Cell{Ch: '›', Style: style})
		}
	}

	if c.Submenu == nil {
		return
	}
	childW := c.Submenu.menuWidth()
	childH := len(c.Submenu.Items) + 2
	childX := x + menuW - 1
	if childX+childW > sw {
		childX = x - childW + 1
	}
	childY := y + 1 + c.submenuIndex
	if childY+childH > sh {
		childY = sh - childH
	}
	c.Submenu.renderAt(surface, childX, childY, sw, sh)
}

func (c *ContextMenuWidget) storeRect(x, y, w, h int) {
	c.SetRect(Rect{X: x, Y: y, W: w, H: h})
}

func (c *ContextMenuWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		active := c.activeMenu()
		switch tev.Key() {
		case tcell.KeyEscape:
			if c.OnDismiss != nil {
				c.OnDismiss()
			}
			return EventConsumed
		case tcell.KeyUp:
			active.moveSelection(-1)
			return EventConsumed
		case tcell.KeyDown:
			active.moveSelection(1)
			return EventConsumed
		case tcell.KeyLeft:
			if active.parent != nil {
				active.parent.Submenu = nil
			} else if c.OnNavigate != nil {
				c.OnNavigate(-1)
			}
			return EventConsumed
		case tcell.KeyRight:
			if active.openSelectedSubmenu() {
				return EventConsumed
			}
			if active.parent == nil && c.OnNavigate != nil {
				c.OnNavigate(1)
			}
			return EventConsumed
		case tcell.KeyEnter:
			if active.Selected >= 0 && active.Selected < len(active.Items) && !active.Items[active.Selected].IsSep {
				if active.openSelectedSubmenu() {
					return EventConsumed
				}
				if c.OnExec != nil && active.Items[active.Selected].Command != "" {
					c.OnExec(active.Items[active.Selected].Command)
				}
			}
			return EventConsumed
		}
	case *tcell.EventMouse:
		btn := tev.Buttons()
		mx, my := tev.Position()
		if c.firstEvent {
			c.firstEvent = false
			return EventConsumed
		}

		menu := c.menuAt(mx, my)
		if btn&tcell.Button1 != 0 {
			if menu == nil {
				if c.OnDismiss != nil {
					c.OnDismiss()
				}
				return EventConsumed
			}
			itemIdx := my - menu.GetRect().Y - 1
			if itemIdx >= 0 && itemIdx < len(menu.Items) && !menu.Items[itemIdx].IsSep {
				menu.selectItem(itemIdx)
				if menu.openSelectedSubmenu() {
					return EventConsumed
				}
				if c.OnExec != nil && menu.Items[menu.Selected].Command != "" {
					c.OnExec(menu.Items[menu.Selected].Command)
				}
			}
			return EventConsumed
		}

		if btn == tcell.ButtonNone {
			if menu == nil {
				c.Selected = -1
				if c.OnMouseOutside != nil {
					c.OnMouseOutside(ev)
				}
				return EventConsumed
			}
			itemIdx := my - menu.GetRect().Y - 1
			if itemIdx >= 0 && itemIdx < len(menu.Items) && !menu.Items[itemIdx].IsSep {
				menu.selectItem(itemIdx)
				menu.openSelectedSubmenu()
			}
			return EventConsumed
		}
	}
	return EventConsumed
}

func (c *ContextMenuWidget) activeMenu() *ContextMenuWidget {
	if c.Submenu != nil {
		return c.Submenu.activeMenu()
	}
	return c
}

func (c *ContextMenuWidget) menuAt(x, y int) *ContextMenuWidget {
	if c.Submenu != nil {
		if menu := c.Submenu.menuAt(x, y); menu != nil {
			return menu
		}
	}
	r := c.GetRect()
	if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
		return c
	}
	return nil
}

func (c *ContextMenuWidget) selectItem(index int) {
	if c.Selected != index {
		c.Selected = index
		c.Submenu = nil
	}
}

func (c *ContextMenuWidget) openSelectedSubmenu() bool {
	if c.Selected < 0 || c.Selected >= len(c.Items) {
		return false
	}
	items := c.Items[c.Selected].Submenu
	if len(items) == 0 {
		return false
	}
	if c.Submenu != nil && c.submenuIndex == c.Selected {
		return true
	}
	child := NewContextMenuWidget(items, 0, 0)
	child.Borders = c.Borders
	child.parent = c
	child.firstEvent = false
	c.Submenu = child
	c.submenuIndex = c.Selected
	return true
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
			c.Submenu = nil
			return
		}
	}
}
