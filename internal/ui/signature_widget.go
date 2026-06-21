package ui

import (
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

type SignatureHelpWidget struct {
	BaseWidget
	Label            string
	ActiveParamStart int
	ActiveParamEnd   int
	AnchorX          int
	AnchorY          int
	Borders          *term.BorderSet
}

func NewSignatureHelpWidget(label string, paramStart, paramEnd int) *SignatureHelpWidget {
	return &SignatureHelpWidget{
		Label:            label,
		ActiveParamStart: paramStart,
		ActiveParamEnd:   paramEnd,
	}
}

func (s *SignatureHelpWidget) Focusable() bool { return false }

func (s *SignatureHelpWidget) Render(surface *RenderSurface) {
	if s.Label == "" {
		return
	}
	sw, sh := surface.Size()

	runes := []rune(s.Label)
	contentW := len(runes) + 2
	menuW := contentW + 2
	if menuW > sw {
		menuW = sw
		contentW = menuW - 2
	}
	menuH := 3

	x := s.AnchorX
	if x+menuW > sw {
		x = sw - menuW
	}
	if x < 0 {
		x = 0
	}

	y := s.AnchorY - menuH
	if y < 0 {
		y = s.AnchorY + 1
		if y+menuH > sh {
			return
		}
	}

	b := term.SingleBorderSet()
	if s.Borders != nil {
		b = *s.Borders
	}
	surface.DrawBorder(x, y, menuW, menuH, b, term.StyleBorder)

	row := y + 1
	st := term.StylePaletteItem
	stHighlight := term.StylePaletteSelected

	for bx := x + 1; bx < x+menuW-1; bx++ {
		surface.SetCell(bx, row, term.Cell{Ch: ' ', Style: st})
	}

	paramStartRune := s.runeOffset(s.ActiveParamStart)
	paramEndRune := s.runeOffset(s.ActiveParamEnd)

	for i := 0; i < contentW && i < len(runes); i++ {
		style := st
		if paramStartRune < paramEndRune && i >= paramStartRune && i < paramEndRune {
			style = stHighlight
		}
		surface.SetCell(x+1+i, row, term.Cell{Ch: runes[i], Style: style})
	}

	s.SetRect(Rect{X: x, Y: y, W: menuW, H: menuH})
}

func (s *SignatureHelpWidget) runeOffset(byteOffset int) int {
	label := s.Label
	if byteOffset >= len(label) {
		return len([]rune(label))
	}
	if byteOffset <= 0 {
		return 0
	}
	return len([]rune(label[:byteOffset]))
}

func (s *SignatureHelpWidget) HandleEvent(ev tcell.Event) EventResult {
	return EventIgnored
}
