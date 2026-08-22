package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type DropdownConfig struct {
	Label   string      `json:"label,omitempty"`
	Entries []MenuEntry `json:"entries,omitempty"`
	Style   term.Style  `json:"-"`
	Box     *BoxModel

	OnMenu func(entries []MenuEntry, screenX, screenY int)
}

type DropdownWidget struct {
	BaseWidget
	Config DropdownConfig
	button *ButtonWidget
}

func NewDropdownWidget(config DropdownConfig) *DropdownWidget {
	if config.Label == "" {
		config.Label = "⋮"
	}
	btn := NewButtonWidget(ButtonConfig{
		Label: config.Label,
		Style: config.Style,
	})
	if config.Box != nil {
		btn.Box = *config.Box
	}
	return &DropdownWidget{Config: config, button: btn}
}

func (d *DropdownWidget) UpdateConfig(config DropdownConfig) {
	if config.Label == "" {
		config.Label = "⋮"
	}
	d.Config = config
	d.button.Config.Label = config.Label
	d.button.Config.Style = config.Style
	d.button.SetLabel(config.Label)
	if config.Box != nil {
		d.button.Box = *config.Box
	}
}

func (d *DropdownWidget) Height() int { return d.button.Height() }
func (d *DropdownWidget) Width() int  { return d.button.Width() }

func (d *DropdownWidget) Render(surface Surface) {
	d.button.SetRect(d.GetRect())
	d.button.Render(surface)
}

func (d *DropdownWidget) HandleEvent(ev tcell.Event) EventResult {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		if e.Buttons()&tcell.Button1 != 0 {
			mx, my := e.Position()
			r := d.GetRect()
			if mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H {
				if d.Config.OnMenu != nil && len(d.Config.Entries) > 0 {
					d.Config.OnMenu(d.Config.Entries, r.X, r.Y+r.H)
					return EventConsumed
				}
			}
		}
	}
	return EventIgnored
}
