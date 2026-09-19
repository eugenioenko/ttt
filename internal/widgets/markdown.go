package widgets

import (
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/markdown"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type MarkdownWidget struct {
	BaseWidget
	MaxWidth  int
	FillStyle term.Style
	rawText   string
	lines     []markdown.Line
	wrapWidth int
	wrapped   []markdown.Line

	sel      selection.Selection
	selEnd   selection.Position
	dragging bool

	scrollParent *ScrollViewWidget
}

func NewMarkdownWidget() *MarkdownWidget {
	return &MarkdownWidget{MaxWidth: 80}
}

func (m *MarkdownWidget) SetScrollParent(sv *ScrollViewWidget) {
	m.scrollParent = sv
}

func (m *MarkdownWidget) SetContent(text string) {
	if text == m.rawText {
		return
	}
	m.rawText = text
	m.lines = markdown.Render(text)
	m.wrapWidth = 0
	m.wrapped = nil
	m.sel.Clear()
	m.dragging = false
}

func (m *MarkdownWidget) Height() int { return 0 }
func (m *MarkdownWidget) Width() int  { return 0 }

func (m *MarkdownWidget) ScrollSize() (int, int) {
	var w int
	if m.scrollParent != nil {
		w = m.scrollParent.GetRect().W
	}
	if w <= 0 {
		r := m.GetRect()
		w = r.W
	}
	if w <= 0 {
		w = 80
	}
	wrapW := w - m.Box.PaddingLeft - m.Box.PaddingRight
	if wrapW < 1 {
		wrapW = 1
	}
	m.rewrap(wrapW)
	contentW := 0
	for _, line := range m.wrapped {
		if line.Kind == markdown.KindDivider {
			continue
		}
		if lw := len([]rune(line.Text())); lw > contentW {
			contentW = lw
		}
	}
	contentW += m.Box.PaddingLeft + m.Box.PaddingRight
	h := len(m.wrapped) + m.Box.PaddingTop + m.Box.PaddingBottom
	return contentW, h
}

func (m *MarkdownWidget) rewrap(width int) {
	if m.MaxWidth > 0 && width > m.MaxWidth {
		width = m.MaxWidth
	}
	if width == m.wrapWidth && m.wrapped != nil {
		return
	}
	m.wrapWidth = width
	m.wrapped = nil
	for _, line := range m.lines {
		if !line.Kind.Wrappable() || len([]rune(line.Text())) <= width {
			m.wrapped = append(m.wrapped, line)
			continue
		}
		m.wrapped = append(m.wrapped, markdown.WrapLine(line, width)...)
	}
}

func (m *MarkdownWidget) wrappedTextLines() []string {
	lines := make([]string, len(m.wrapped))
	for i, l := range m.wrapped {
		lines[i] = l.Text()
	}
	return lines
}

func (m *MarkdownWidget) ContentSize(width int) (maxLineW, lineCount int) {
	m.rewrap(width)
	for _, line := range m.wrapped {
		if w := len([]rune(line.Text())); w > maxLineW {
			maxLineW = w
		}
	}
	return maxLineW, len(m.wrapped)
}

func (m *MarkdownWidget) Render(surface Surface) {
	surface = m.RenderBox(surface)
	w, h := surface.Size()
	if w <= 0 || h <= 0 {
		return
	}
	if m.FillStyle != 0 {
		surface.Fill(term.Cell{Ch: ' ', Style: m.FillStyle})
	}

	m.rewrap(w)

	for y := 0; y < h && y < len(m.wrapped); y++ {
		line := m.wrapped[y]
		if line.Kind == markdown.KindDivider {
			// The span carries the rule character: a heavier one under a level-1
			// heading than under a level-2, which is what separates them.
			ch := '─'
			if len(line.Spans) > 0 {
				for _, r := range line.Spans[0].Text {
					ch = r
					break
				}
			}
			for x := 0; x < w; x++ {
				surface.SetCell(x, y, term.Cell{Ch: ch, Style: term.StyleBorder})
			}
			continue
		}
		x := 0
		for _, span := range line.Spans {
			for _, ch := range span.Text {
				if x >= w {
					break
				}
				cell := term.Cell{Ch: ch, Style: span.Style}
				if m.sel.Contains(y, x, m.selEnd.Line, m.selEnd.Col) {
					cell.BgStyle = term.StyleSelection
				}
				surface.SetCell(x, y, cell)
				x++
			}
		}
		if m.sel.Active {
			for pad := x; pad < w; pad++ {
				if m.sel.Contains(y, pad, m.selEnd.Line, m.selEnd.Col) {
					surface.SetCell(pad, y, term.Cell{Ch: ' ', BgStyle: term.StyleSelection})
				}
			}
		}
	}
}

func (m *MarkdownWidget) mouseToContent(mx, my int) selection.Position {
	if m.scrollParent == nil {
		return selection.Position{}
	}
	pr := m.scrollParent.GetRect()

	contentY := my - pr.Y + m.scrollParent.scrollY
	contentX := mx - pr.X + m.scrollParent.scrollX

	if contentY < 0 {
		contentY = 0
	}
	if contentX < 0 {
		contentX = 0
	}
	if contentY >= len(m.wrapped) {
		contentY = len(m.wrapped) - 1
		if contentY < 0 {
			contentY = 0
		}
	}
	if contentY < len(m.wrapped) {
		lineLen := len([]rune(m.wrapped[contentY].Text()))
		if contentX > lineLen {
			contentX = lineLen
		}
	}
	return selection.Position{Line: contentY, Col: contentX}
}

func (m *MarkdownWidget) HandleEvent(ev tcell.Event) EventResult {
	switch e := ev.(type) {
	case *tcell.EventKey:
		if e.Key() == tcell.KeyCtrlC && m.sel.Active {
			text := m.sel.Text(m.wrappedTextLines(), m.selEnd.Line, m.selEnd.Col)
			if text != "" {
				clipboard.Set(text)
			}
			return EventConsumed
		}
	case *tcell.EventMouse:
		if m.scrollParent == nil {
			return EventIgnored
		}
		btn := e.Buttons()
		mx, my := e.Position()

		pr := m.scrollParent.GetRect()

		inside := mx >= pr.X && mx < pr.X+pr.W &&
			my >= pr.Y && my < pr.Y+pr.H

		if btn&tcell.Button1 != 0 {
			pos := m.mouseToContent(mx, my)
			if m.dragging {
				m.selEnd = pos
				return EventConsumed
			}
			if inside {
				m.sel.Start(pos.Line, pos.Col)
				m.selEnd = pos
				m.dragging = true
				return EventConsumed
			}
		}

		if btn == tcell.ButtonNone && m.dragging {
			m.dragging = false
			if m.sel.Anchor == m.selEnd {
				m.sel.Clear()
			}
			return EventConsumed
		}
	}
	return EventIgnored
}
