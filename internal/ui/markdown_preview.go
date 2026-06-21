package ui

import (
	"github.com/eugenioenko/ttt/internal/markdown"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

const markdownMaxWidth = 80

// MarkdownPreviewWidget renders parsed markdown content as styled text
// in a read-only, scrollable preview tab.
type MarkdownPreviewWidget struct {
	BaseWidget
	FilePath string
	lines    []markdown.Line
	topLine  int
	viewH    int
}

// NewMarkdownPreviewWidget creates a preview widget from the given markdown content.
func NewMarkdownPreviewWidget(path string, content string) *MarkdownPreviewWidget {
	lines := markdown.RenderPreview(content, markdownMaxWidth)
	return &MarkdownPreviewWidget{
		FilePath: path,
		lines:    lines,
	}
}

func (m *MarkdownPreviewWidget) Focusable() bool { return true }

func (m *MarkdownPreviewWidget) Render(surface *RenderSurface) {
	w, h := surface.Size()
	m.viewH = h

	// Calculate left padding to center content if viewport is wider than maxWidth
	padding := 0
	if w > markdownMaxWidth {
		padding = (w - markdownMaxWidth) / 2
	}

	// Clear the surface
	surface.Fill(term.Cell{Ch: ' ', Style: term.StyleDefault})

	for y := 0; y < h; y++ {
		idx := m.topLine + y
		if idx >= len(m.lines) {
			break
		}
		line := m.lines[idx]
		x := padding
		for _, span := range line.Spans {
			for _, ch := range span.Text {
				if x >= w {
					break
				}
				surface.SetCell(x, y, term.Cell{Ch: ch, Style: span.Style})
				x++
			}
		}
	}
}

func (m *MarkdownPreviewWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		switch tev.Key() {
		case tcell.KeyUp:
			if m.topLine > 0 {
				m.topLine--
			}
			return EventConsumed
		case tcell.KeyDown:
			max := m.maxTop()
			if m.topLine < max {
				m.topLine++
			}
			return EventConsumed
		case tcell.KeyPgUp:
			m.topLine -= m.viewH
			if m.topLine < 0 {
				m.topLine = 0
			}
			return EventConsumed
		case tcell.KeyPgDn:
			max := m.maxTop()
			m.topLine += m.viewH
			if m.topLine > max {
				m.topLine = max
			}
			return EventConsumed
		case tcell.KeyHome:
			m.topLine = 0
			return EventConsumed
		case tcell.KeyEnd:
			m.topLine = m.maxTop()
			return EventConsumed
		}

	case *tcell.EventMouse:
		btn := tev.Buttons()
		if btn&tcell.WheelUp != 0 {
			m.topLine -= 3
			if m.topLine < 0 {
				m.topLine = 0
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			max := m.maxTop()
			m.topLine += 3
			if m.topLine > max {
				m.topLine = max
			}
			return EventConsumed
		}
	}
	return EventIgnored
}

func (m *MarkdownPreviewWidget) maxTop() int {
	max := len(m.lines) - m.viewH
	if max < 0 {
		max = 0
	}
	return max
}
