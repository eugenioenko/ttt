package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"

	"github.com/gdamore/tcell/v3"
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

func (s *SignatureHelpWidget) Render(surface Surface) {
	if s.Label == "" {
		return
	}
	sw, sh := surface.Size()

	runes := []rune(s.Label)
	menuW := textwidth.Runes(runes) + 4
	if menuW > sw {
		menuW = sw
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

	// Draw by display width so fullwidth runes advance two columns and a wide
	// rune that would straddle the right border is dropped rather than bled
	// across it. The active-parameter span is a separate highlighted segment,
	// which keeps the highlight on the parameter's own columns.
	limit := x + menuW - 1
	tx := x + 1
	if paramStartRune < paramEndRune {
		tx = surface.DrawText(tx, row, string(runes[:paramStartRune]), limit, st)
		tx = surface.DrawText(tx, row, string(runes[paramStartRune:paramEndRune]), limit, stHighlight)
		surface.DrawText(tx, row, string(runes[paramEndRune:]), limit, st)
	} else {
		surface.DrawText(tx, row, s.Label, limit, st)
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
