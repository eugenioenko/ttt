package widgets

import (
	"strings"
	"unicode"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type ButtonConfig struct {
	Label   string     `json:"label"`
	Command string     `json:"command"`
	Style   term.Style `json:"-"`
	Box     *BoxModel

	OnClick   func()
	OnCommand func(command string)
}

type ButtonWidget struct {
	BaseWidget
	Config                    ButtonConfig
	Disabled                  bool
	label                     string
	accelIndex                int
	accelRune                 rune
	focused                   bool
	pressed                   bool
	mouseButton1Down          bool
	pointerCaptureInvalidated func()
}

func NewButtonWidget(config ButtonConfig) *ButtonWidget {
	b := &ButtonWidget{Config: config, accelIndex: -1}
	if config.Box != nil {
		b.Box = *config.Box
	} else {
		b.Box.PaddingLeft = 1
		b.Box.PaddingRight = 1
	}
	idx := strings.IndexByte(config.Label, '&')
	if idx >= 0 && idx < len(config.Label)-1 {
		runes := []rune(config.Label)
		runeIdx := len([]rune(config.Label[:idx]))
		b.label = string(runes[:runeIdx]) + string(runes[runeIdx+1:])
		b.accelIndex = runeIdx
		b.accelRune = runes[runeIdx+1]
	} else {
		b.label = config.Label
	}
	return b
}

func (b *ButtonWidget) SetLabel(label string) {
	b.label = label
	b.accelIndex = -1
	b.accelRune = 0
}

func (b *ButtonWidget) Height() int { return 1 + b.BoxOverheadH() }
func (b *ButtonWidget) Width() int {
	w := textwidth.String(b.label) + b.BoxOverheadW()
	return w
}

func (b *ButtonWidget) Focusable() bool { return true }
func (b *ButtonWidget) SetFocused(f bool) {
	b.focused = f
	if !f {
		b.CancelPointerCapture()
	}
}

func (b *ButtonWidget) CancelPointerCapture() bool {
	b.mouseButton1Down = false
	if !b.pressed {
		return false
	}
	b.pressed = false
	if b.pointerCaptureInvalidated != nil {
		b.pointerCaptureInvalidated()
	}
	return true
}

func (b *ButtonWidget) SetPointerCaptureInvalidated(invalidated func()) {
	b.pointerCaptureInvalidated = invalidated
}
func (b *ButtonWidget) IsFocused() bool { return b.focused }

func (b *ButtonWidget) Render(surface Surface) {
	style := b.Config.Style
	if style == 0 {
		style = term.StyleButton
	}
	if b.Disabled {
		style = term.StyleMuted
	} else if b.focused {
		style = term.StyleButtonFocused
	}

	padded := b.BorderedInterior(surface)
	pw, _ := padded.Size()
	fillH := 1 + b.Box.PaddingTop + b.Box.PaddingBottom
	for y := 0; y < fillH; y++ {
		for x := 0; x < pw; x++ {
			padded.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		}
	}

	inner := b.RenderBox(surface)
	w, _ := inner.Size()
	if w <= 0 {
		return
	}

	x := 0
	i := 0
	for _, ch := range b.label {
		rw := textwidth.Rune(ch)
		if x+rw > w {
			// A fullwidth rune that would cross the last column becomes a
			// space: the terminal paints it across two columns regardless of
			// clipping, so it would bleed past the button.
			inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
			break
		}
		cell := term.Cell{Ch: ch, Style: style}
		if i == b.accelIndex {
			cell.Underline = true
		}
		inner.SetCell(x, 0, cell)
		x += rw
		i++
	}
}

func (b *ButtonWidget) HandleEvent(ev tcell.Event) EventResult {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		mx, my := e.Position()
		r := b.GetRect()
		inside := mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H
		button1WasDown := b.mouseButton1Down
		b.mouseButton1Down = e.Buttons()&tcell.Button1 != 0
		if e.Buttons()&tcell.Button1 != 0 {
			if button1WasDown {
				if b.pressed {
					return EventCaptured
				}
				return EventIgnored
			}
			if inside {
				b.pressed = true
				return EventCaptured
			}
		}
		if e.Buttons() == tcell.ButtonNone && b.pressed {
			b.pressed = false
			if inside {
				b.trigger()
			}
			return EventConsumed
		}
	case *tcell.EventKey:
		if b.focused {
			if e.Key() == tcell.KeyEnter || (e.Key() == tcell.KeyRune && term.KeyRune(e) == ' ') {
				b.trigger()
				return EventConsumed
			}
		}
		if b.accelRune != 0 && e.Key() == tcell.KeyRune {
			if unicode.ToLower(term.KeyRune(e)) == unicode.ToLower(b.accelRune) {
				b.trigger()
				return EventConsumed
			}
		}
	}
	return EventIgnored
}

func (b *ButtonWidget) trigger() {
	if b.Disabled {
		return
	}
	if b.Config.OnClick != nil {
		b.Config.OnClick()
	}
	if b.Config.OnCommand != nil && b.Config.Command != "" {
		b.Config.OnCommand(b.Config.Command)
	}
}
