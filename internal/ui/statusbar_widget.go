package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

type statusBarSpan struct {
	start, end int
	onClick    func()
}

type StatusBarWidget struct {
	BaseWidget
	Status        *view.StatusBar
	spans         []statusBarSpan
	okSpan        statusBarSpan
	actionSpan    statusBarSpan
	secondarySpan statusBarSpan
}

func NewStatusBarWidget(status *view.StatusBar) *StatusBarWidget {
	return &StatusBarWidget{Status: status}
}

func (s *StatusBarWidget) Height() int { return 1 }

func (s *StatusBarWidget) Render(surface Surface) {
	w, _ := surface.Size()
	st := s.Status

	for x := 0; x < w; x++ {
		surface.SetCell(x, 0, term.Cell{Ch: ' ', Style: term.StyleStatusBar})
	}

	// EchoText takes over the entire status bar — used by plugins for
	// prompts (isearch, query-replace) that need the full row.
	if st.EchoText != "" {
		s.drawText(surface, 1, st.EchoText, term.StyleStatusBar)
		return
	}

	if st.IsNotificationActive() {
		s.renderNotification(surface, w)
		return
	}

	s.okSpan = statusBarSpan{}
	s.spans = s.spans[:0]
	r := s.GetRect()

	x := 0
	x += s.drawText(surface, x, " ", term.StyleStatusBar)

	for _, seg := range st.LeftSegments() {
		if seg.Text == "" {
			continue
		}
		style := seg.Style
		if style == 0 {
			style = term.StyleStatusBar
		}
		textLen := len([]rune(seg.Text))
		if seg.OnClick != nil {
			s.spans = append(s.spans, statusBarSpan{r.X + x, r.X + x + textLen, seg.OnClick})
		}
		x += s.drawText(surface, x, seg.Text, style)
		x += s.drawText(surface, x, "  ", term.StyleStatusBar)
	}

	rightSegs := st.RightSegments()
	rightStr := ""
	for i, seg := range rightSegs {
		if i > 0 {
			rightStr += "   "
		}
		rightStr += seg.Text
	}
	rightStr += " "

	rx := w - len([]rune(rightStr))
	if rx > x {
		pos := rx
		for i, seg := range rightSegs {
			if i > 0 {
				s.drawText(surface, pos, "   ", term.StyleStatusBar)
				pos += 3
			}
			style := seg.Style
			if style == 0 {
				style = term.StyleStatusBar
			}
			segLen := len([]rune(seg.Text))
			if seg.OnClick != nil {
				s.spans = append(s.spans, statusBarSpan{r.X + pos, r.X + pos + segLen, seg.OnClick})
			}
			s.drawText(surface, pos, seg.Text, style)
			pos += segLen
		}
	}
}

func (s *StatusBarWidget) renderNotification(surface Surface, w int) {
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
			s.actionSpan = statusBarSpan{r.X + actionX, r.X + actionX + len([]rune(actionLabel)), nil}
			for i, ch := range actionLabel {
				surface.SetCell(actionX+i, 0, term.Cell{Ch: ch, Style: style})
			}
			rightX = actionX
		}
		if s.Status.SecondaryLabel != "" && s.Status.SecondaryAction != nil {
			secLabel := " [" + s.Status.SecondaryLabel + "] "
			secX := rightX - len([]rune(secLabel))
			if secX > x+2 {
				s.secondarySpan = statusBarSpan{r.X + secX, r.X + secX + len([]rune(secLabel)), nil}
				for i, ch := range secLabel {
					surface.SetCell(secX+i, 0, term.Cell{Ch: ch, Style: style})
				}
			}
		}
	} else {
		okLabel := " [OK] "
		okX := rightX - len([]rune(okLabel))
		if okX > x+2 {
			s.okSpan = statusBarSpan{r.X + okX, r.X + okX + len([]rune(okLabel)), nil}
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
		return EventIgnored
	}
	for _, span := range s.spans {
		if mx >= span.start && mx < span.end && span.onClick != nil {
			span.onClick()
			return EventConsumed
		}
	}
	return EventIgnored
}

func (s *StatusBarWidget) drawText(surface Surface, x int, text string, style term.Style) int {
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
