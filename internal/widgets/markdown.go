package widgets

import (
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/markdown"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type MarkdownWidget struct {
	BaseWidget
	MaxWidth          int
	FillStyle         term.Style
	FullWidthDividers bool
	rawText           string
	lines             []markdown.Line
	wrapWidth         int
	wrapped           []markdown.Line

	doc             *markdown.Document
	focusedDetailID int
	focused         bool

	sel      selection.Selection
	selEnd   selection.Position
	dragging bool

	scrollParent *ScrollViewWidget
}

func NewMarkdownWidget() *MarkdownWidget {
	return &MarkdownWidget{MaxWidth: 80}
}

func (m *MarkdownWidget) Focusable() bool {
	return m.doc != nil && len(m.doc.Details()) > 0
}

func (m *MarkdownWidget) SetFocused(f bool) {
	m.focused = f
	if f && m.doc != nil {
		if !m.isDetailVisible(m.focusedDetailID) {
			if visible := m.doc.VisibleDetailIDs(); len(visible) > 0 {
				m.focusedDetailID = visible[0]
			} else {
				m.focusedDetailID = 0
			}
		}
	}
}

func (m *MarkdownWidget) isDetailVisible(id int) bool {
	if id <= 0 || m.doc == nil {
		return false
	}
	for _, vis := range m.doc.VisibleDetailIDs() {
		if vis == id {
			return true
		}
	}
	return false
}

func (m *MarkdownWidget) IsFocused() bool {
	return m.focused
}

func (m *MarkdownWidget) Document() *markdown.Document {
	return m.doc
}

func (m *MarkdownWidget) FocusedDetailID() int {
	return m.focusedDetailID
}

func (m *MarkdownWidget) SetScrollParent(sv *ScrollViewWidget) {
	m.scrollParent = sv
}

func (m *MarkdownWidget) SetMaxWidth(maxWidth int) {
	if m.MaxWidth == maxWidth {
		return
	}
	m.MaxWidth = maxWidth
	m.wrapWidth = 0
	m.wrapped = nil
}

func (m *MarkdownWidget) SetContent(text string) {
	if text == m.rawText && m.doc != nil {
		return
	}
	m.rawText = text
	m.doc = markdown.Parse(text)
	m.lines = m.doc.Render()
	m.wrapWidth = 0
	m.wrapped = nil
	m.sel.Clear()
	m.dragging = false
	m.focusedDetailID = 0
}

func (m *MarkdownWidget) Height() int { return 0 }
func (m *MarkdownWidget) Width() int  { return 0 }

func (m *MarkdownWidget) ScrollSize() (int, int) {
	w, h := m.GetRect().W, 0
	if m.scrollParent != nil {
		r := m.scrollParent.GetRect()
		w, h = r.W-m.scrollParent.BoxOverheadW(), r.H-m.scrollParent.BoxOverheadH()
	}
	if w <= 0 {
		w = 80
	}
	wrapW := max(1, w-m.BoxOverheadW())
	for {
		m.rewrap(wrapW)
		contentW := 0
		for _, line := range m.wrapped {
			if line.Kind == markdown.KindDivider {
				contentW = max(contentW, wrapW)
			} else {
				contentW = max(contentW, textwidth.String(line.Text()))
			}
		}
		contentW += m.BoxOverheadW()
		contentH := len(m.wrapped) + m.BoxOverheadH()
		if m.scrollParent == nil || h <= 0 {
			return contentW, contentH
		}
		viewW, _ := m.scrollParent.viewportSize(w, h, contentW, contentH)
		nextWidth := max(1, viewW-m.BoxOverheadW())
		if nextWidth >= wrapW {
			return contentW, contentH
		}
		wrapW = nextWidth
	}
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
		if !line.Kind.Wrappable() || textwidth.String(line.Text()) <= width {
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
		if w := textwidth.String(line.Text()); w > maxLineW {
			maxLineW = w
		}
	}
	return maxLineW, len(m.wrapped)
}

func (m *MarkdownWidget) Render(surface Surface) {
	outer := surface
	surface = m.RenderBox(surface)
	w, h := surface.Size()
	if w <= 0 || h <= 0 {
		return
	}
	if m.FillStyle != 0 {
		surface.Fill(term.Cell{Ch: ' ', Style: m.FillStyle})
	}

	// The virtual surface can be wider than the viewport because of code blocks.
	if m.scrollParent == nil || m.wrapWidth == 0 {
		m.rewrap(w)
	}

	for y := 0; y < h && y < len(m.wrapped); y++ {
		line := m.wrapped[y]
		if line.Kind == markdown.KindDivider {
			if m.FullWidthDividers {
				outerW, _ := outer.Size()
				dividerY := y + m.Box.MarginTop + m.Box.PaddingTop
				if m.Box.BorderTop {
					dividerY++
				}
				startX, endX := m.Box.MarginLeft, outerW-m.Box.MarginRight
				if m.Box.BorderLeft {
					startX++
				}
				if m.Box.BorderRight {
					endX--
				}
				for x := startX; x < endX; x++ {
					outer.SetCell(x, dividerY, term.Cell{Ch: '─', Style: term.StyleBorder})
				}
				continue
			}
			for x := 0; x < w; x++ {
				surface.SetCell(x, y, term.Cell{Ch: '─', Style: term.StyleBorder})
			}
			continue
		}
		x, col := 0, 0
		for _, span := range line.Spans {
			for _, ch := range span.Text {
				if x >= w {
					break
				}
				width := textwidth.Rune(ch)
				cell := term.Cell{Ch: ch, Style: span.Style}
				if x+width > w {
					cell.Ch = ' '
					width = 1
				}
				if m.sel.Contains(y, col, m.selEnd.Line, m.selEnd.Col) {
					cell.BgStyle = term.StyleSelection
				} else if m.focused && line.Kind == markdown.KindSummary && line.DetailID == m.focusedDetailID {
					cell.BgStyle = term.StyleSidebarSelected
				}
				surface.SetCell(x, y, cell)
				x += width
				col++
			}
		}
		if m.sel.Active {
			for pad := x; pad < w; pad++ {
				if m.sel.Contains(y, col+pad-x, m.selEnd.Line, m.selEnd.Col) {
					surface.SetCell(pad, y, term.Cell{Ch: ' ', BgStyle: term.StyleSelection})
				}
			}
		} else if m.focused && line.Kind == markdown.KindSummary && line.DetailID == m.focusedDetailID {
			for pad := x; pad < w; pad++ {
				surface.SetCell(pad, y, term.Cell{Ch: ' ', BgStyle: term.StyleSidebarSelected})
			}
		}
	}
}

func (m *MarkdownWidget) isMouseInside(mx, my int) bool {
	if m.scrollParent != nil {
		pr := m.scrollParent.GetRect()
		return mx >= pr.X && mx < pr.X+pr.W && my >= pr.Y && my < pr.Y+pr.H
	}
	r := m.GetRect()
	return mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H
}

func (m *MarkdownWidget) mouseToContent(mx, my int) selection.Position {
	if len(m.wrapped) == 0 {
		w := m.rect.W - m.BoxOverheadW()
		if w <= 0 {
			w = 80
		}
		m.rewrap(w)
	}
	ox, oy := m.contentOrigin()
	contentY := my - oy
	contentX := mx - ox

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
	col := 0
	if contentY < len(m.wrapped) {
		x := 0
		for _, ch := range m.wrapped[contentY].Text() {
			width := textwidth.Rune(ch)
			if x+width > contentX {
				break
			}
			x += width
			col++
		}
	}
	return selection.Position{Line: contentY, Col: col}
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
		if m.doc != nil && len(m.doc.Details()) > 0 {
			switch e.Key() {
			case tcell.KeyTab:
				if m.focusNextDetail(true) {
					return EventConsumed
				}
			case tcell.KeyBacktab:
				if m.focusPrevDetail(true) {
					return EventConsumed
				}
			case tcell.KeyEscape:
				if m.focusedDetailID > 0 {
					m.focusedDetailID = 0
					return EventConsumed
				}
			case tcell.KeyEnter, tcell.KeyRune:
				if m.focusedDetailID > 0 && (e.Key() == tcell.KeyEnter || term.KeyRune(e) == ' ') {
					m.toggleDetail(m.focusedDetailID)
					return EventConsumed
				}
			}
		}
	case *tcell.EventMouse:
		mx, my := e.Position()
		inside := m.isMouseInside(mx, my)
		btn := e.Buttons()

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
			isClick := m.sel.Anchor == m.selEnd
			if isClick {
				m.sel.Clear()
				if m.doc != nil && m.selEnd.Line >= 0 && m.selEnd.Line < len(m.wrapped) {
					line := m.wrapped[m.selEnd.Line]
					if line.Kind == markdown.KindSummary && line.DetailID > 0 {
						m.focusedDetailID = line.DetailID
						m.toggleDetail(line.DetailID)
						return EventConsumed
					}
				}
			}
			return EventConsumed
		}
	}
	return EventIgnored
}

// FocusNextDetail advances keyboard focus to the next visible summary without wrapping.
// Returns true if focus advanced, false if already at the last visible summary.
func (m *MarkdownWidget) FocusNextDetail() bool {
	return m.focusNextDetail(false)
}

// FocusPrevDetail moves keyboard focus to the previous visible summary without wrapping.
// Returns true if focus moved, false if already at the first visible summary.
func (m *MarkdownWidget) FocusPrevDetail() bool {
	return m.focusPrevDetail(false)
}

// FocusFirstDetail sets keyboard focus to the first visible summary.
func (m *MarkdownWidget) FocusFirstDetail() bool {
	if m.doc == nil {
		return false
	}
	visible := m.doc.VisibleDetailIDs()
	if len(visible) == 0 {
		return false
	}
	m.focusedDetailID = visible[0]
	m.ensureDetailVisible(m.focusedDetailID)
	return true
}

// FocusLastDetail sets keyboard focus to the last visible summary.
func (m *MarkdownWidget) FocusLastDetail() bool {
	if m.doc == nil {
		return false
	}
	visible := m.doc.VisibleDetailIDs()
	if len(visible) == 0 {
		return false
	}
	m.focusedDetailID = visible[len(visible)-1]
	m.ensureDetailVisible(m.focusedDetailID)
	return true
}

func (m *MarkdownWidget) focusNextDetail(wrap bool) bool {
	if m.doc == nil {
		return false
	}
	visible := m.doc.VisibleDetailIDs()
	if len(visible) == 0 {
		return false
	}
	currentIdx := -1
	for i, id := range visible {
		if id == m.focusedDetailID {
			currentIdx = i
			break
		}
	}
	if currentIdx < 0 {
		m.focusedDetailID = visible[0]
	} else if currentIdx >= len(visible)-1 {
		if !wrap {
			return false
		}
		m.focusedDetailID = visible[0]
	} else {
		m.focusedDetailID = visible[currentIdx+1]
	}
	m.ensureDetailVisible(m.focusedDetailID)
	return true
}

func (m *MarkdownWidget) focusPrevDetail(wrap bool) bool {
	if m.doc == nil {
		return false
	}
	visible := m.doc.VisibleDetailIDs()
	if len(visible) == 0 {
		return false
	}
	currentIdx := -1
	for i, id := range visible {
		if id == m.focusedDetailID {
			currentIdx = i
			break
		}
	}
	if currentIdx < 0 {
		m.focusedDetailID = visible[len(visible)-1]
	} else if currentIdx <= 0 {
		if !wrap {
			return false
		}
		m.focusedDetailID = visible[len(visible)-1]
	} else {
		m.focusedDetailID = visible[currentIdx-1]
	}
	m.ensureDetailVisible(m.focusedDetailID)
	return true
}

func (m *MarkdownWidget) ensureDetailVisible(detailID int) {
	if m.scrollParent == nil {
		return
	}
	y := m.detailRow(detailID)
	if y >= 0 {
		m.scrollParent.EnsureVisible(0, y)
	}
}

func (m *MarkdownWidget) detailRow(detailID int) int {
	for y, line := range m.wrapped {
		if line.Kind == markdown.KindSummary && line.DetailID == detailID {
			return y
		}
	}
	return -1
}

func (m *MarkdownWidget) toggleDetail(detailID int) {
	if m.doc == nil {
		return
	}
	if m.doc.Toggle(detailID) {
		m.lines = m.doc.Render()
		m.wrapWidth = 0
		m.wrapped = nil
		m.sel.Clear()
		m.clampScrollParent()
	}
}

func (m *MarkdownWidget) clampScrollParent() {
	if m.scrollParent == nil {
		return
	}
	r := m.scrollParent.GetRect()
	w, h := r.W-m.scrollParent.BoxOverheadW(), r.H-m.scrollParent.BoxOverheadH()
	contentW, contentH := m.ScrollSize()
	viewW, viewH := m.scrollParent.viewportSize(w, h, contentW, contentH)
	m.scrollParent.clamp(contentW, contentH, viewW, viewH)
}
