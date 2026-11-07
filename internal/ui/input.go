package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type InputHolder interface {
	FocusedInput() *widgets.InputWidget
}

func newPanelInput() *widgets.InputWidget {
	input := widgets.NewInputWidget(widgets.InputConfig{PrefixStyle: term.StyleInput})
	input.SetFocused(true)
	return input
}

func renderPanelInput(surface Surface, input *widgets.InputWidget, x, y, width int) {
	sub := surface.Sub(Rect{X: x, Y: y, W: width, H: 1})
	ox, oy := sub.Origin()
	w, h := sub.Size()
	input.SetRect(Rect{X: ox, Y: oy, W: w, H: h})
	input.Render(sub)
}
