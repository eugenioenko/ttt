package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

// CommandLineHeight is the number of rows the framed command line occupies.
const CommandLineHeight = 3

// CommandLineWidget is a framed single-line editor docked directly above the
// status bar. It is rendered as a modal overlay rather than a layout child so
// opening it never resizes the editor underneath (which would make the buffer
// visibly jump).
//
// Being a modal overlay is load-bearing beyond the visuals: Root.handleOverlay
// runs above the plugin KeyInterceptor, so a modal plugin (Vim mode) goes
// silent for free while the command line is open — no focus stashing, no mode
// flags.
type CommandLineWidget struct {
	BaseWidget

	Prefix  string
	Borders *term.BorderSet

	OnChange func(text string)
	OnSubmit func(text string)
	OnCancel func()

	// OnKey, when set, is offered every key before the widget handles it.
	// Returning true consumes the key.
	OnKey func(ev *tcell.EventKey) bool

	input *widgets.InputWidget

	// Layout computed by Render and reused by event handlers, so clicks and the
	// cursor never disagree with what was drawn.
	boxX, boxY, boxW int
	laidOut          bool
}

func NewCommandLineWidget(prefix string) *CommandLineWidget {
	if prefix == "" {
		prefix = ":"
	}
	c := &CommandLineWidget{Prefix: prefix}
	c.input = widgets.NewInputWidget(widgets.InputConfig{
		Prefix: prefix,
		Style:  term.StyleInput,
		OnChange: func(text string) {
			if c.OnChange != nil {
				c.OnChange(text)
			}
		},
	})
	c.input.SetFocused(true)
	return c
}

func (c *CommandLineWidget) Focusable() bool { return true }

func (c *CommandLineWidget) SetFocused(f bool) { c.input.SetFocused(f) }

func (c *CommandLineWidget) Text() string { return c.input.Text() }

// SetText replaces the text. OnChange fires, matching keystroke behaviour so
// incremental consumers (search-as-you-type) see every change.
func (c *CommandLineWidget) SetText(text string) { c.input.SetText(text) }

func (c *CommandLineWidget) Height() int { return CommandLineHeight }

// layout computes the box geometry for a surface of the given size, and reports
// whether there is room to draw at all. The box sits directly above the
// one-row status bar.
func commandLineLayout(w, h int) (x, y, width int, ok bool) {
	if w < 4 || h < CommandLineHeight+1 {
		return 0, 0, 0, false
	}
	return 0, h - CommandLineHeight - 1, w, true
}

func (c *CommandLineWidget) Render(surface Surface) {
	w, h := surface.Size()
	x, y, width, ok := commandLineLayout(w, h)
	if !ok {
		c.laidOut = false
		return
	}
	c.boxX, c.boxY, c.boxW = x, y, width
	c.laidOut = true

	borders := term.RoundedBorderSet()
	if c.Borders != nil {
		borders = *c.Borders
	}

	surface.ClearRect(x, y, width, CommandLineHeight, term.StyleDefault)
	surface.DrawBorder(x, y, width, CommandLineHeight, borders, term.StyleBorderActive)

	innerX := x + 2
	innerY := y + 1
	innerW := width - 4
	if innerW <= 0 {
		return
	}

	ox, oy := surface.Origin()
	c.input.SetRect(Rect{X: ox + innerX, Y: oy + innerY, W: innerW, H: 1})
	c.input.Render(surface.Sub(Rect{X: innerX, Y: innerY, W: innerW, H: 1}))
}

func (c *CommandLineWidget) CursorPosition() (int, int, bool) {
	if !c.laidOut {
		return 0, 0, false
	}
	return c.input.CursorPosition()
}

func (c *CommandLineWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		return c.handleKey(tev)
	case *tcell.EventMouse:
		return c.input.HandleEvent(tev)
	}
	return EventIgnored
}

func (c *CommandLineWidget) handleKey(kev *tcell.EventKey) EventResult {
	// OnKey sees every key first, including Enter and Escape, so a caller can
	// drive the prompt itself -- an incremental search needs the keystrokes as
	// they arrive, not just the final text. Declining falls through to the
	// normal handling below.
	if c.OnKey != nil && c.OnKey(kev) {
		return EventConsumed
	}
	switch kev.Key() {
	case tcell.KeyEnter:
		// The overlay sits above Root's Escape handling, so submit/cancel are
		// ours to own.
		if c.OnSubmit != nil {
			c.OnSubmit(c.input.Text())
		}
		return EventConsumed
	case tcell.KeyEscape:
		if c.OnCancel != nil {
			c.OnCancel()
		}
		return EventConsumed
	}
	return c.input.HandleEvent(kev)
}

// Input exposes the inner line editor. Clipboard keys are handled by the input
// itself, so this exists for tests and callers that need finer control.
func (c *CommandLineWidget) Input() *widgets.InputWidget { return c.input }
