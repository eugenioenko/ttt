package ui

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

type FindMatch struct {
	Line, Col, Len int
}

type FindBarWidget struct {
	BaseWidget
	Input      *InputWidget
	Matches    []FindMatch
	Current    int
	Options    SearchOptions
	Borders    *term.BorderSet
	OnSearch   func(query string, opts SearchOptions) []FindMatch
	OnNavigate func(match FindMatch)
	OnDismiss  func()
	Debounce   Debouncer
	focused    bool
	btnPrev    HitRegion
	btnNext    HitRegion
	btnClose   HitRegion
}

func NewFindBarWidget() *FindBarWidget {
	f := &FindBarWidget{focused: true}
	f.Input = NewInputWidget()
	f.Input.Placeholder = "Search"
	f.Input.OnChange = func(string) {
		f.Current = 0
		f.search()
	}
	f.Input.Actions = []InputAction{
		{Label: "Aa", OnClick: func() {
			f.Options.CaseSensitive = !f.Options.CaseSensitive
			f.syncActions()
			f.Current = 0
			f.search()
		}},
		{Label: ".*", OnClick: func() {
			f.Options.UseRegex = !f.Options.UseRegex
			f.syncActions()
			f.Current = 0
			f.search()
		}},
	}
	return f
}

func (f *FindBarWidget) syncActions() {
	if len(f.Input.Actions) >= 2 {
		f.Input.Actions[0].Active = f.Options.CaseSensitive
		f.Input.Actions[1].Active = f.Options.UseRegex
	}
}

func (f *FindBarWidget) barLayout() (barX, barY, barW, barH int) {
	r := f.GetRect()
	barW = 40
	if barW > r.W-4 {
		barW = r.W - 4
	}
	barX = r.W - barW - 1
	barY = 4
	barH = 3
	return
}

func (f *FindBarWidget) Focusable() bool { return true }

func (f *FindBarWidget) Focus() {
	f.focused = true
}

func (f *FindBarWidget) CursorPosition() (int, int, bool) {
	if !f.focused {
		return 0, 0, false
	}
	r := f.GetRect()
	barX, barY, barW, _ := f.barLayout()
	row := barY + 1
	cx := f.Input.CursorX(barX + 1)
	if cx >= barX+barW-1 {
		cx = barX + barW - 2
	}
	return r.X + cx, r.Y + row, true
}

func (f *FindBarWidget) Render(surface Surface) {
	sw, _ := surface.Size()

	barW := 40
	if barW > sw-4 {
		barW = sw - 4
	}
	barX := sw - barW - 1
	barY := 4
	barH := 3

	b := term.SingleBorderSet()
	if f.Borders != nil {
		b = *f.Borders
	}
	surface.DrawBorder(barX, barY, barW, barH, b, term.StyleBorder)

	row := barY + 1

	info := ""
	hasMatches := f.Input.Text != "" && len(f.Matches) > 0
	if f.Input.Text != "" {
		info = fmt.Sprintf(" %d/%d", f.currentDisplay(), len(f.Matches))
	}
	navButtons := ""
	if hasMatches {
		navButtons = " ▲ ▼"
	}
	closeBtn := " ✕ "
	suffixW := len([]rune(info)) + len([]rune(navButtons)) + len([]rune(closeBtn))

	inputW := barW - 2 - suffixW
	if inputW < 4 {
		inputW = 4
	}
	f.Input.Render(surface, barX+1, row, inputW)

	cx := barX + 1 + inputW
	for _, ch := range info {
		if cx < barX+barW-1 {
			surface.SetCell(cx, row, term.Cell{Ch: ch, Style: term.StyleMuted})
			cx++
		}
	}

	navStart := cx
	for _, ch := range navButtons {
		if cx < barX+barW-1 {
			surface.SetCell(cx, row, term.Cell{Ch: ch, Style: term.StyleMuted})
			cx++
		}
	}
	f.btnPrev = HitRegion{}
	f.btnNext = HitRegion{}
	if hasMatches {
		f.btnPrev = HitRegion{X: navStart + 1, Y: row, W: 1}
		f.btnNext = HitRegion{X: navStart + 3, Y: row, W: 1}
	}
	closeStart := cx
	for _, ch := range closeBtn {
		if cx < barX+barW-1 {
			surface.SetCell(cx, row, term.Cell{Ch: ch, Style: term.StyleMuted})
			cx++
		}
	}
	f.btnClose = HitRegion{X: closeStart + 1, Y: row, W: 1}
}

func (f *FindBarWidget) currentDisplay() int {
	if len(f.Matches) == 0 {
		return 0
	}
	return f.Current + 1
}

func (f *FindBarWidget) search() {
	f.Debounce.Schedule(func() {
		f.doSearch()
	})
}

func (f *FindBarWidget) doSearch() {
	if f.OnSearch != nil {
		f.Matches = f.OnSearch(f.Input.Text, f.Options)
		if len(f.Matches) > 0 {
			if f.Current >= len(f.Matches) {
				f.Current = 0
			}
			f.navigate()
		}
	}
}

// Refresh re-runs the query against content that changed under the bar,
// keeping the current match index where it still exists, without navigating.
func (f *FindBarWidget) Refresh() {
	if f.OnSearch == nil || f.Input.Text == "" {
		return
	}
	f.Matches = f.OnSearch(f.Input.Text, f.Options)
	f.Current = max(min(f.Current, len(f.Matches)-1), 0)
}

func (f *FindBarWidget) navigate() {
	if f.OnNavigate != nil && len(f.Matches) > 0 {
		f.OnNavigate(f.Matches[f.Current])
	}
}

func (f *FindBarWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventMouse:
		if tev.Buttons()&tcell.Button1 != 0 {
			mx, my := tev.Position()
			barX, barY, barW, barH := f.barLayout()
			r := f.GetRect()
			localX := mx - r.X
			localY := my - r.Y
			if localY >= barY && localY < barY+barH && localX >= barX && localX < barX+barW {
				f.focused = true
				row := barY + 1
				if localY == row && localX >= barX+1 && localX < barX+1+barW-2 {
					f.Input.HandleClick(mx, my)
				}
				if f.btnPrev.Contains(mx, my) {
					if len(f.Matches) > 0 {
						f.Current = (f.Current - 1 + len(f.Matches)) % len(f.Matches)
						f.navigate()
					}
				}
				if f.btnNext.Contains(mx, my) {
					if len(f.Matches) > 0 {
						f.Current = (f.Current + 1) % len(f.Matches)
						f.navigate()
					}
				}
				if f.btnClose.Contains(mx, my) {
					if f.OnDismiss != nil {
						f.OnDismiss()
					}
				}
				return EventConsumed
			}
			f.focused = false
		}
		return EventIgnored
	case *tcell.EventKey:
		return f.handleKey(tev)
	}
	return EventIgnored
}

func (f *FindBarWidget) handleKey(kev *tcell.EventKey) EventResult {
	if !f.focused {
		if kev.Key() == tcell.KeyEscape {
			if f.OnDismiss != nil {
				f.OnDismiss()
			}
			return EventConsumed
		}
		return EventIgnored
	}

	switch kev.Key() {
	case tcell.KeyEscape:
		if f.OnDismiss != nil {
			f.OnDismiss()
		}
		return EventConsumed
	case tcell.KeyEnter, tcell.KeyDown:
		if len(f.Matches) > 0 {
			if kev.Modifiers()&tcell.ModShift != 0 {
				f.Current = (f.Current - 1 + len(f.Matches)) % len(f.Matches)
			} else {
				f.Current = (f.Current + 1) % len(f.Matches)
			}
			f.navigate()
		}
		return EventConsumed
	case tcell.KeyUp:
		if len(f.Matches) > 0 {
			f.Current = (f.Current - 1 + len(f.Matches)) % len(f.Matches)
			f.navigate()
		}
		return EventConsumed
	}

	if kev.Modifiers()&tcell.ModAlt != 0 && kev.Key() == tcell.KeyRune {
		switch term.KeyRune(kev) {
		case 'c':
			f.Options.CaseSensitive = !f.Options.CaseSensitive
			f.syncActions()
			f.Current = 0
			f.search()
			return EventConsumed
		case 'r':
			f.Options.UseRegex = !f.Options.UseRegex
			f.syncActions()
			f.Current = 0
			f.search()
			return EventConsumed
		}
	}

	if f.Input.HandleEvent(tcell.NewEventKey(kev.Key(), kev.Str(), kev.Modifiers())) == EventConsumed {
		return EventConsumed
	}

	return EventIgnored
}
