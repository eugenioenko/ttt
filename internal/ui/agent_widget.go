package ui

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

// AgentMessage is one entry in the embedded coding-agent transcript.
type AgentMessage struct {
	Role string
	Text string
}

// AgentWidget is a small host surface for coding-agent conversations. The
// transport is deliberately injected so the widget remains UI-only.
type AgentWidget struct {
	widgets.BaseWidget
	Input    *widgets.InputWidget
	Messages []AgentMessage
	OnSubmit func(string)
	focused  bool
}

// NewAgentWidget creates an empty Agent conversation panel.
func NewAgentWidget(onSubmit func(string)) *AgentWidget {
	w := &AgentWidget{OnSubmit: onSubmit}
	w.Input = widgets.NewInputWidget(widgets.InputConfig{Placeholder: "Ask Codex…", Bordered: true, Multiline: true, MaxRows: 4, Autofocus: true})
	w.Input.Config.OnSubmit = func(string) { w.Submit() }
	return w
}
func (w *AgentWidget) Height() int                      { return 0 }
func (w *AgentWidget) Width() int                       { return 0 }
func (w *AgentWidget) Focusable() bool                  { return true }
func (w *AgentWidget) IsFocused() bool                  { return w.focused }
func (w *AgentWidget) SetFocused(f bool)                { w.focused = f; w.Input.SetFocused(f) }
func (w *AgentWidget) CursorPosition() (int, int, bool) { return w.Input.CursorPosition() }

func (w *AgentWidget) Render(s widgets.Surface) {
	width, height := s.Size()
	s.Fill(term.Cell{Ch: ' ', Style: term.StyleDefault})
	s.DrawText(1, 0, "Codex Agent", width-1, term.StyleActiveTab)
	row := 2
	for _, msg := range w.Messages {
		prefix := "You: "
		style := term.StyleDefault
		if msg.Role != "user" {
			prefix, style = "Codex: ", term.StyleMuted
		}
		for _, line := range strings.Split(msg.Text, "\n") {
			if row >= height-6 {
				break
			}
			s.DrawText(1, row, prefix+line, width-1, style)
			prefix = "      "
			row++
		}
	}
	inputH := w.Input.Height()
	if inputH < 1 {
		inputH = 1
	}
	boxBottom := max(height-inputH-2, 2)
	if width > 2 && boxBottom > 2 {
		for x := 1; x < width-1; x++ {
			s.SetCell(x, 1, term.Cell{Ch: '─', Style: term.StyleBorder})
			s.SetCell(x, boxBottom, term.Cell{Ch: '─', Style: term.StyleBorder})
		}
		for y := 2; y < boxBottom; y++ {
			s.SetCell(1, y, term.Cell{Ch: '│', Style: term.StyleBorder})
			s.SetCell(width-2, y, term.Cell{Ch: '│', Style: term.StyleBorder})
		}
	}
	w.Input.SetRect(widgets.Rect{X: w.GetRect().X + 1, Y: w.GetRect().Y + max(height-inputH-1, 1), W: max(width-2, 1), H: inputH})
	w.Input.Render(s.Sub(widgets.Rect{X: 1, Y: max(height-inputH-1, 1), W: max(width-2, 1), H: inputH}))
}
func (w *AgentWidget) HandleEvent(ev tcell.Event) widgets.EventResult { return w.Input.HandleEvent(ev) }
func (w *AgentWidget) Submit() {
	text := strings.TrimSpace(w.Input.Text())
	if text == "" || w.OnSubmit == nil {
		return
	}
	w.Input.SetText("")
	w.Messages = append(w.Messages, AgentMessage{Role: "user", Text: text})
	w.OnSubmit(text)
}
