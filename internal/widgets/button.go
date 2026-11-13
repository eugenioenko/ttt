package widgets

import (
	"strings"
	"unicode"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

// ButtonVariant is how a button is drawn while it is not focused.
type ButtonVariant int

const (
	// ButtonLink draws the bare label, as core dialogs do.
	ButtonLink ButtonVariant = iota
	ButtonFilled
	ButtonBracket
	ButtonInverse
	ButtonMuted
)

// ButtonSpan is one styled run of a button's text. Style is a theme style
// name resolved by the caller (plugin layer); zero keeps the button's own
// style for that run.
type ButtonSpan struct {
	Text  string
	Style term.Style
}

type ButtonConfig struct {
	Label   string        `json:"label"`
	Command string        `json:"command"`
	Style   term.Style    `json:"-"`
	Variant ButtonVariant `json:"-"`
	// Spans, when non-empty, replaces the plain label with styled runs.
	// Label stays as the plain-text fallback and is ignored while spans
	// are set.
	Spans []ButtonSpan `json:"-"`
	Box   *BoxModel

	OnClick   func()
	OnCommand func(command string)
}

type ButtonWidget struct {
	BaseWidget
	Config     ButtonConfig
	Disabled   bool
	label      string
	spans      []ButtonSpan
	accelIndex int
	accelRune  rune
	focused    bool
}

func NewButtonWidget(config ButtonConfig) *ButtonWidget {
	b := &ButtonWidget{Config: config, accelIndex: -1}
	if config.Box != nil {
		b.Box = *config.Box
	} else {
		b.Box.PaddingLeft = 1
		b.Box.PaddingRight = 1
	}
	b.spans = append([]ButtonSpan(nil), config.Spans...)
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
	b.spans = nil
	b.accelIndex = -1
	b.accelRune = 0
}

// SetSpans replaces the styled runs and drops the plain label, keeping the
// button focused state and callbacks untouched.
func (b *ButtonWidget) SetSpans(spans []ButtonSpan) {
	b.spans = append([]ButtonSpan(nil), spans...)
	b.label = ""
	b.accelIndex = -1
	b.accelRune = 0
}

// HasSpans reports whether the button renders styled runs instead of the
// plain label.
func (b *ButtonWidget) HasSpans() bool { return len(b.spans) > 0 }

func (b *ButtonWidget) Height() int { return 1 + b.BoxOverheadH() }
func (b *ButtonWidget) Width() int {
	if b.HasSpans() {
		w := 0
		for _, s := range b.spans {
			w += textwidth.String(s.Text)
		}
		if b.Config.Variant == ButtonBracket {
			w += 2
		}
		return w + b.BoxOverheadW()
	}
	return textwidth.String(b.text()) + b.BoxOverheadW()
}

func (b *ButtonWidget) text() string {
	if b.Config.Variant == ButtonBracket {
		return "[" + b.label + "]"
	}
	return b.label
}

func (b *ButtonWidget) style() term.Style {
	switch {
	case b.Disabled:
		return term.StyleMuted
	case b.focused:
		return term.StyleButtonFocused
	case b.Config.Style != 0:
		return b.Config.Style
	}
	switch b.Config.Variant {
	case ButtonFilled:
		return term.StyleButtonFilled
	case ButtonInverse:
		return term.StyleButtonInverse
	case ButtonMuted:
		return term.StyleMuted
	}
	return term.StyleButton
}

func (b *ButtonWidget) Focusable() bool   { return true }
func (b *ButtonWidget) SetFocused(f bool) { b.focused = f }
func (b *ButtonWidget) IsFocused() bool   { return b.focused }

func (b *ButtonWidget) Render(surface Surface) {
	style := b.style()

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

	if b.HasSpans() {
		b.renderSpans(inner, w)
		return
	}

	x := 0
	i := 0
	accel := b.accelIndex
	if accel >= 0 && b.Config.Variant == ButtonBracket {
		accel++
	}
	for _, ch := range b.text() {
		rw := textwidth.Rune(ch)
		if x+rw > w {
			// A fullwidth rune that would cross the last column becomes a
			// space: the terminal paints it across two columns regardless of
			// clipping, so it would bleed past the button.
			inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
			break
		}
		cell := term.Cell{Ch: ch, Style: style}
		if i == accel {
			cell.Underline = true
		}
		inner.SetCell(x, 0, cell)
		x += rw
		i++
	}
}

// renderSpans draws the styled runs. Focused and disabled buttons keep their
// own background: span styles apply only to an unfocused, enabled button.
func (b *ButtonWidget) renderSpans(inner Surface, w int) {
	base := b.style()
	spanStyle := func(s term.Style) term.Style {
		if b.Disabled || b.focused || s == 0 {
			return base
		}
		return s
	}
	reserve := 0
	if b.Config.Variant == ButtonBracket {
		reserve = 1
	}
	x := 0
	if reserve == 1 && w > 0 {
		inner.SetCell(0, 0, term.Cell{Ch: '[', Style: base})
		x = 1
	}
	for _, span := range b.spans {
		st := spanStyle(span.Style)
		for _, ch := range span.Text {
			rw := textwidth.Rune(ch)
			if x+rw > w-reserve {
				if x < w-reserve {
					inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: base})
				}
				if reserve == 1 && w > 0 {
					inner.SetCell(w-1, 0, term.Cell{Ch: ']', Style: base})
				}
				return
			}
			inner.SetCell(x, 0, term.Cell{Ch: ch, Style: st})
			x += rw
		}
	}
	if reserve == 1 && x < w {
		inner.SetCell(x, 0, term.Cell{Ch: ']', Style: base})
	}
}

func (b *ButtonWidget) HandleEvent(ev tcell.Event) EventResult {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		if e.Buttons()&tcell.Button1 != 0 {
			mx, my := e.Position()
			r := b.GetRect()
			if mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H {
				b.trigger()
				return EventConsumed
			}
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
