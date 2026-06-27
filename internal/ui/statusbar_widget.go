package ui

import (
	"fmt"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v2"
)

type statusBarSpan struct {
	start, end int
}

type StatusBarWidget struct {
	BaseWidget
	Status        *view.StatusBar
	OnIndentClick func()
	OnEolClick    func()
	indentSpan    statusBarSpan
	eolSpan       statusBarSpan
	okSpan        statusBarSpan
	actionSpan    statusBarSpan
	secondarySpan statusBarSpan
}

func NewStatusBarWidget(status *view.StatusBar) *StatusBarWidget {
	return &StatusBarWidget{Status: status}
}

func (s *StatusBarWidget) Render(surface *RenderSurface) {
	w, _ := surface.Size()
	st := s.Status

	for x := 0; x < w; x++ {
		surface.SetCell(x, 0, term.Cell{Ch: ' ', Style: term.StyleStatusBar})
	}

	if st.IsNotificationActive() {
		s.renderNotification(surface, w)
		return
	}

	s.okSpan = statusBarSpan{}

	x := 0
	x += s.drawText(surface, x, " ", term.StyleStatusBar)

	if st.Branch != "" {
		x += s.drawText(surface, x, st.Branch, term.StyleStatusBar)
		x += s.drawText(surface, x, "  ", term.StyleStatusBar)
	}

	if st.Blame != "" {
		x += s.drawText(surface, x, st.Blame, term.StyleStatusBar)
	}

	if st.ReviewProgress != "" {
		if x > 1 {
			x += s.drawText(surface, x, "  ", term.StyleStatusBar)
		}
		x += s.drawText(surface, x, st.ReviewProgress, term.StyleSuccess)
	}

	type segment struct {
		text string
		id   string
	}
	var right []segment
	posText := fmt.Sprintf("Ln %d, Col %d", st.Line+1, st.Col+1)
	if st.CursorCount > 1 {
		posText += fmt.Sprintf(" (%d cursors)", st.CursorCount)
	}
	right = append(right, segment{posText, "pos"})
	if st.TabSize > 0 {
		indentLabel := "Spaces"
		if st.UseTabs {
			indentLabel = "Tab Size"
		}
		right = append(right, segment{fmt.Sprintf("%s: %d", indentLabel, st.TabSize), "indent"})
	}
	right = append(right, segment{"UTF-8", "encoding"})
	eolLabel := "LF"
	if st.LineEnding == "\r\n" {
		eolLabel = "CRLF"
	}
	right = append(right, segment{eolLabel, "eol"})
	if st.Language != "" {
		lang := st.Language
		if st.LSP {
			lang += " ⊕"
		}
		right = append(right, segment{lang, "lang"})
	}

	rightStr := ""
	for i, seg := range right {
		if i > 0 {
			rightStr += "   "
		}
		rightStr += seg.text
	}
	rightStr += " "

	rx := w - len([]rune(rightStr))
	if rx > x {
		pos := rx
		r := s.GetRect()
		for i, seg := range right {
			if i > 0 {
				s.drawText(surface, pos, "   ", term.StyleStatusBar)
				pos += 3
			}
			segLen := len([]rune(seg.text))
			if seg.id == "indent" {
				s.indentSpan = statusBarSpan{r.X + pos, r.X + pos + segLen}
			}
			if seg.id == "eol" {
				s.eolSpan = statusBarSpan{r.X + pos, r.X + pos + segLen}
			}
			s.drawText(surface, pos, seg.text, term.StyleStatusBar)
			pos += segLen
		}
	}
}

func (s *StatusBarWidget) renderNotification(surface *RenderSurface, w int) {
	r := s.GetRect()
	style := s.Status.NotifyLevel.Style()

	for x := 0; x < w; x++ {
		surface.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
	}

	x := 0
	x += s.drawText(surface, x, " ", style)
	x += s.drawText(surface, x, s.Status.Notification, style)

	s.actionSpan = statusBarSpan{}
	s.secondarySpan = statusBarSpan{}
	s.okSpan = statusBarSpan{}
	rightX := w - 1

	if s.Status.ActionLabel != "" && s.Status.NotifyAction != nil {
		actionLabel := " [" + s.Status.ActionLabel + "] "
		actionX := rightX - len([]rune(actionLabel))
		if actionX > x+2 {
			s.actionSpan = statusBarSpan{r.X + actionX, r.X + actionX + len([]rune(actionLabel))}
			for i, ch := range actionLabel {
				surface.SetCell(actionX+i, 0, term.Cell{Ch: ch, Style: style})
			}
			rightX = actionX
		}
		if s.Status.SecondaryLabel != "" && s.Status.SecondaryAction != nil {
			secLabel := " [" + s.Status.SecondaryLabel + "] "
			secX := rightX - len([]rune(secLabel))
			if secX > x+2 {
				s.secondarySpan = statusBarSpan{r.X + secX, r.X + secX + len([]rune(secLabel))}
				for i, ch := range secLabel {
					surface.SetCell(secX+i, 0, term.Cell{Ch: ch, Style: style})
				}
			}
		}
	} else {
		okLabel := " [OK] "
		okX := rightX - len([]rune(okLabel))
		if okX > x+2 {
			s.okSpan = statusBarSpan{r.X + okX, r.X + okX + len([]rune(okLabel))}
			for i, ch := range okLabel {
				surface.SetCell(okX+i, 0, term.Cell{Ch: ch, Style: style})
			}
		}
	}
}

func (s *StatusBarWidget) HandleEvent(ev tcell.Event) EventResult {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}
	if mev.Buttons()&tcell.Button1 == 0 {
		return EventIgnored
	}
	mx, my := mev.Position()
	r := s.GetRect()
	if my != r.Y {
		return EventIgnored
	}
	if s.Status.IsNotificationActive() {
		if mx >= s.okSpan.start && mx < s.okSpan.end {
			s.Status.DismissNotification()
			return EventConsumed
		}
		if s.actionSpan.start != s.actionSpan.end && mx >= s.actionSpan.start && mx < s.actionSpan.end {
			if s.Status.NotifyAction != nil {
				s.Status.NotifyAction()
			}
			s.Status.DismissNotification()
			return EventConsumed
		}
		if s.secondarySpan.start != s.secondarySpan.end && mx >= s.secondarySpan.start && mx < s.secondarySpan.end {
			if s.Status.SecondaryAction != nil {
				s.Status.SecondaryAction()
			}
			s.Status.DismissNotification()
			return EventConsumed
		}
	}
	if mx >= s.indentSpan.start && mx < s.indentSpan.end && s.OnIndentClick != nil {
		s.OnIndentClick()
		return EventConsumed
	}
	if mx >= s.eolSpan.start && mx < s.eolSpan.end && s.OnEolClick != nil {
		s.OnEolClick()
		return EventConsumed
	}
	return EventIgnored
}

func (s *StatusBarWidget) drawText(surface *RenderSurface, x int, text string, style term.Style) int {
	w, _ := surface.Size()
	n := 0
	for _, ch := range text {
		if x+n >= w {
			break
		}
		surface.SetCell(x+n, 0, term.Cell{Ch: ch, Style: style})
		n++
	}
	return n
}
