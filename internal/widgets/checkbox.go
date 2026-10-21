package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type CheckboxConfig struct {
	Label    string
	Checked  bool
	Style    term.Style `json:"-"`
	OnChange func(checked bool)
}

type CheckboxWidget struct {
	BaseWidget
	Config  CheckboxConfig
	focused bool
}

func NewCheckboxWidget(config CheckboxConfig) *CheckboxWidget {
	return &CheckboxWidget{Config: config}
}

func (c *CheckboxWidget) Height() int { return 1 + c.BoxOverheadH() }
func (c *CheckboxWidget) Width() int  { return 0 }

func (c *CheckboxWidget) Focusable() bool   { return true }
func (c *CheckboxWidget) SetFocused(f bool) { c.focused = f }
func (c *CheckboxWidget) IsFocused() bool   { return c.focused }

func (c *CheckboxWidget) Render(surface Surface) {
	inner := c.RenderBox(surface)
	w, _ := inner.Size()
	if w < 4 {
		return
	}

	style := c.Config.Style
	if style == 0 {
		style = term.StyleDefault
	}

	// Focus shows in the brackets alone, matching the input widget's "❯" prefix.
	// The mark and label keep their own colour so the checked state stays legible.
	bracket := term.StyleBorder
	if c.focused {
		bracket = term.StyleBorderActive
	}

	for x := range w {
		inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
	}

	mark := ' '
	if c.Config.Checked {
		mark = 'x'
	}
	inner.SetCell(1, 0, term.Cell{Ch: '[', Style: bracket})
	inner.SetCell(2, 0, term.Cell{Ch: mark, Style: style})
	inner.SetCell(3, 0, term.Cell{Ch: ']', Style: bracket})

	inner.DrawText(5, 0, c.Config.Label, w, style)
}

func (c *CheckboxWidget) toggle() {
	c.Config.Checked = !c.Config.Checked
	if c.Config.OnChange != nil {
		c.Config.OnChange(c.Config.Checked)
	}
}

func (c *CheckboxWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		if !c.focused {
			return EventIgnored
		}
		if tev.Key() == tcell.KeyEnter || (tev.Key() == tcell.KeyRune && term.KeyRune(tev) == ' ') {
			c.toggle()
			return EventConsumed
		}
	case *tcell.EventMouse:
		if tev.Buttons()&tcell.Button1 != 0 {
			mx, my := tev.Position()
			r := c.GetRect()
			if mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H {
				c.toggle()
				return EventConsumed
			}
		}
	}
	return EventIgnored
}
