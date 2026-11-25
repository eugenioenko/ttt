package ui

import (
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

type ToolbarCorner int

const (
	ToolbarTopRight ToolbarCorner = iota
	ToolbarTopLeft
	ToolbarBottomRight
	ToolbarBottomLeft
)

type DebugToolbar struct {
	Corner       ToolbarCorner
	EditorBounds func() Rect
	Borders      *term.BorderSet
	CanStep      func() bool
	Attached     bool
	OnCommand    func(string)
	OnMenu       func(int, int)
	rect         Rect
	hits         [5]HitRegion
}

func (b *DebugToolbar) buttons() [5]icons.Name {
	stop := icons.DebugStop
	if b.Attached {
		stop = icons.DebugDetach
	}
	return [5]icons.Name{icons.DebugStepOver, icons.DebugStepIn, icons.DebugStepOut, stop, icons.More}
}

func (b *DebugToolbar) render(surface Surface) {
	buttons := b.buttons()
	width := 3
	for _, name := range buttons {
		width += textwidth.Rune(icons.Rune(name)) + 2
	}
	bounds := b.EditorBounds()
	if bounds.W < width+2 || bounds.H < 3 {
		b.rect = Rect{}
		return
	}
	xPos, yPos := bounds.X+bounds.W-width-1, bounds.Y
	if b.Corner == ToolbarTopLeft || b.Corner == ToolbarBottomLeft {
		xPos = bounds.X + 1
	}
	if b.Corner == ToolbarBottomLeft || b.Corner == ToolbarBottomRight {
		yPos = bounds.Y + max(0, bounds.H-4)
	}
	b.rect = Rect{X: xPos, Y: yPos, W: width, H: 3}
	drawFloatingBar(surface, b.Borders, b.rect)
	x, y := b.rect.X+1, b.rect.Y+1
	ox, oy := surface.Origin()
	for i, name := range buttons {
		style := term.StyleInputAction
		if i < 3 && !b.CanStep() {
			style = term.StyleMuted
		}
		end := surface.DrawText(x, y, " "+string(icons.Rune(name))+" ", b.rect.X+b.rect.W-1, style)
		b.hits[i] = HitRegion{X: ox + x, Y: oy + y, W: end - x}
		x = end
		if i == 2 {
			x++
		}
	}
}

func (b *DebugToolbar) click(x, y int) {
	if hit := b.hits[4]; hit.Contains(x, y) {
		b.OnMenu(hit.X, hit.Y+1)
		return
	}
	commands := [4]string{"debug.stepOver", "debug.stepIn", "debug.stepOut", "debug.stop"}
	if b.Attached {
		commands[3] = "debug.detach"
	}
	for i, cmd := range commands {
		if b.hits[i].Contains(x, y) && (i >= 3 || b.CanStep()) {
			b.OnCommand(cmd)
			return
		}
	}
}
