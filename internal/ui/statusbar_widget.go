package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
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
		start := x
		x += s.drawText(surface, x, seg.Text, style)
		if seg.OnClick != nil {
			// The span comes from what was drawn, not a second measurement, so
			// a clipped segment can never claim columns it does not occupy.
			s.spans = append(s.spans, statusBarSpan{r.X + start, r.X + x, seg.OnClick})
		}
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

	rx := w - textwidth.String(rightStr)
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
			start := pos
			pos += s.drawText(surface, pos, seg.Text, style)
			if seg.OnClick != nil {
				s.spans = append(s.spans, statusBarSpan{r.X + start, r.X + pos, seg.OnClick})
			}
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
		actionW := textwidth.String(actionLabel)
		actionX := rightX - actionW
		if actionX > x+2 {
			s.actionSpan = statusBarSpan{r.X + actionX, r.X + actionX + actionW, nil}
			s.drawText(surface, actionX, actionLabel, style)
			rightX = actionX
		}
		if s.Status.SecondaryLabel != "" && s.Status.SecondaryAction != nil {
			secLabel := " [" + s.Status.SecondaryLabel + "] "
			secW := textwidth.String(secLabel)
			secX := rightX - secW
			if secX > x+2 {
				s.secondarySpan = statusBarSpan{r.X + secX, r.X + secX + secW, nil}
				s.drawText(surface, secX, secLabel, style)
			}
		}
	} else {
		okLabel := " [OK] "
		okW := textwidth.String(okLabel)
		okX := rightX - okW
		if okX > x+2 {
			s.okSpan = statusBarSpan{r.X + okX, r.X + okX + okW, nil}
			s.drawText(surface, okX, okLabel, style)
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

// drawText draws text at x and returns the number of columns consumed, which is
// more than the rune count when the text contains fullwidth characters.
func (s *StatusBarWidget) drawText(surface Surface, x int, text string, style term.Style) int {
	w, _ := surface.Size()
	n := 0
	for _, ch := range text {
		cw := textwidth.Rune(ch)
		if x+n+cw > w {
			break
		}
		surface.SetCell(x+n, 0, term.Cell{Ch: ch, Style: style})
		n += cw
	}
	return n
}
