package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type ParagraphWidget struct {
	BaseWidget
	Text  string
	Style term.Style

	lastWidth int
	lines     []string
}

func NewParagraphWidget(text string) *ParagraphWidget {
	return &ParagraphWidget{Text: text}
}

func (p *ParagraphWidget) Height() int {
	if p.lastWidth > 0 {
		return len(p.lines) + p.BoxOverheadH()
	}
	return 0
}

func (p *ParagraphWidget) Width() int { return 0 }

func (p *ParagraphWidget) HeightAtWidth(w int) int {
	return len(wrapText(p.Text, max(w-p.BoxOverheadW(), 1))) + p.BoxOverheadH()
}

func (p *ParagraphWidget) HeightForWidth(w int) int {
	return len(wrapText(p.Text, w)) + p.BoxOverheadH()
}

func (p *ParagraphWidget) Render(surface Surface) {
	inner := p.RenderBox(surface)
	w, h := inner.Size()
	if w <= 0 || h <= 0 {
		return
	}

	if w != p.lastWidth {
		p.lastWidth = w
		p.lines = wrapText(p.Text, w)
	}

	style := p.Style
	if style == 0 {
		style = term.StyleDefault
	}

	for i, line := range p.lines {
		if i >= h {
			break
		}
		inner.DrawText(0, i, line, w, style)
	}
}

func (p *ParagraphWidget) HandleEvent(ev tcell.Event) EventResult {
	return EventIgnored
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return nil
	}

	runes := []rune(text)
	var lines []string
	start := 0

	for start < len(runes) {
		if start+width >= len(runes) {
			lines = append(lines, string(runes[start:]))
			break
		}

		end := start + width
		lastSpace := -1
		for i := end - 1; i > start; i-- {
			if runes[i] == ' ' {
				lastSpace = i
				break
			}
		}

		if lastSpace > start {
			lines = append(lines, string(runes[start:lastSpace]))
			start = lastSpace + 1
		} else {
			lines = append(lines, string(runes[start:end]))
			start = end
		}
	}

	if len(lines) == 0 {
		lines = []string{""}
	}

	return lines
}
