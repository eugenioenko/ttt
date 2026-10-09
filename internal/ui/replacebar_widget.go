package ui

import (
	"fmt"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

type ReplaceBarWidget struct {
	BaseWidget
	SearchInput  *InputWidget
	ReplaceInput *InputWidget
	Matches      []FindMatch
	Current      int
	Options      SearchOptions
	Borders      *term.BorderSet
	OnSearch     func(query string, opts SearchOptions) []FindMatch
	OnNavigate   func(match FindMatch)
	OnReplace    func(match FindMatch, replacement string)
	OnReplaceAll func(query, replacement string, opts SearchOptions)
	OnDismiss    func()
	focusRow     int
	btnPrev      HitRegion
	btnNext      HitRegion
	btnClose     HitRegion
}

func NewReplaceBarWidget() *ReplaceBarWidget {
	r := &ReplaceBarWidget{}
	r.SearchInput = NewInputWidget()
	r.SearchInput.Placeholder = "Search"
	r.SearchInput.OnChange = func(string) {
		r.Current = 0
		r.search()
	}
	r.SearchInput.Actions = []InputAction{
		{Label: "Aa", OnClick: func() {
			r.Options.CaseSensitive = !r.Options.CaseSensitive
			r.syncActions()
			r.Current = 0
			r.search()
		}},
		{Label: ".*", OnClick: func() {
			r.Options.UseRegex = !r.Options.UseRegex
			r.syncActions()
			r.Current = 0
			r.search()
		}},
	}

	r.ReplaceInput = NewInputWidget()
	r.ReplaceInput.Placeholder = "Replace"
	r.ReplaceInput.Actions = []InputAction{
		{Label: "⟳", OnClick: func() {
			if len(r.Matches) > 0 && r.OnReplace != nil {
				r.OnReplace(r.Matches[r.Current], r.ReplaceInput.Text)
				r.search()
			}
		}},
		{Label: "All", OnClick: func() {
			if r.OnReplaceAll != nil {
				r.OnReplaceAll(r.SearchInput.Text, r.ReplaceInput.Text, r.Options)
				r.search()
			}
		}},
	}

	return r
}

func (r *ReplaceBarWidget) syncActions() {
	if len(r.SearchInput.Actions) >= 2 {
		r.SearchInput.Actions[0].Active = r.Options.CaseSensitive
		r.SearchInput.Actions[1].Active = r.Options.UseRegex
	}
}

func (r *ReplaceBarWidget) barLayout() (barX, barY, barW, barH int) {
	rect := r.GetRect()
	barW = 40
	if barW > rect.W-4 {
		barW = rect.W - 4
	}
	barX = rect.W - barW - 1
	barY = 4
	barH = 4
	return
}

func (r *ReplaceBarWidget) Focusable() bool { return true }

func (r *ReplaceBarWidget) CursorPosition() (int, int, bool) {
	rect := r.GetRect()
	barX, barY, barW, _ := r.barLayout()
	row := barY + 1 + r.focusRow
	inp := r.SearchInput
	if r.focusRow == 1 {
		inp = r.ReplaceInput
	}
	cx := inp.CursorX(barX + 1)
	if cx >= barX+barW-1 {
		cx = barX + barW - 2
	}
	return rect.X + cx, rect.Y + row, true
}

func (r *ReplaceBarWidget) Render(surface Surface) {
	sw, _ := surface.Size()

	barW := 40
	if barW > sw-4 {
		barW = sw - 4
	}
	barX := sw - barW - 1
	barY := 4
	barH := 4

	b := term.SingleBorderSet()
	if r.Borders != nil {
		b = *r.Borders
	}
	surface.DrawBorder(barX, barY, barW, barH, b, term.StyleBorder)

	findRow := barY + 1
	replRow := barY + 2

	info := ""
	if r.SearchInput.Text != "" {
		info = fmt.Sprintf(" %d/%d", r.currentDisplay(), len(r.Matches))
	}
	navButtons := " ▲ ▼ ✕"
	suffixW := len([]rune(info)) + len([]rune(navButtons))

	searchInputW := barW - 2 - suffixW
	if searchInputW < 4 {
		searchInputW = 4
	}
	r.SearchInput.Render(surface, barX+1, findRow, searchInputW)

	cx := barX + 1 + searchInputW
	for _, ch := range info {
		if cx < barX+barW-1 {
			surface.SetCell(cx, findRow, term.Cell{Ch: ch, Style: term.StyleMuted})
			cx++
		}
	}

	navStart := cx
	for _, ch := range navButtons {
		if cx < barX+barW-1 {
			surface.SetCell(cx, findRow, term.Cell{Ch: ch, Style: term.StyleMuted})
			cx++
		}
	}
	r.btnPrev = HitRegion{X: navStart + 1, Y: findRow, W: 1}
	r.btnNext = HitRegion{X: navStart + 3, Y: findRow, W: 1}
	r.btnClose = HitRegion{X: navStart + 5, Y: findRow, W: 1}

	replInputW := barW - 2
	r.ReplaceInput.Render(surface, barX+1, replRow, replInputW)
}

func (r *ReplaceBarWidget) currentDisplay() int {
	if len(r.Matches) == 0 {
		return 0
	}
	return r.Current + 1
}

func (r *ReplaceBarWidget) search() {
	if r.OnSearch != nil {
		r.Matches = r.OnSearch(r.SearchInput.Text, r.Options)
		if len(r.Matches) > 0 {
			if r.Current >= len(r.Matches) {
				r.Current = 0
			}
			r.navigate()
		}
	}
}

// SetMatches must not navigate: it runs after the user edits, and moving the
// cursor would yank it away from where they are typing.
func (r *ReplaceBarWidget) SetMatches(matches []FindMatch) {
	r.Matches = matches
	r.Current = min(r.Current, max(len(matches)-1, 0))
}

func (r *ReplaceBarWidget) navigate() {
	if r.OnNavigate != nil && len(r.Matches) > 0 {
		r.OnNavigate(r.Matches[r.Current])
	}
}

func (r *ReplaceBarWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventMouse:
		if tev.Buttons()&tcell.Button1 != 0 {
			mx, my := tev.Position()
			barX, barY, barW, _ := r.barLayout()
			rect := r.GetRect()
			localX := mx - rect.X
			localY := my - rect.Y

			findRow := barY + 1
			replRow := barY + 2

			if localY == findRow && localX >= barX+1 && localX < barX+1+barW-2 {
				r.focusRow = 0
				r.SearchInput.HandleClick(mx, my)
				return EventConsumed
			}
			if localY == replRow && localX >= barX+1 && localX < barX+1+barW-2 {
				r.focusRow = 1
				r.ReplaceInput.HandleClick(mx, my)
				return EventConsumed
			}

			if r.btnPrev.Contains(mx, my) {
				if len(r.Matches) > 0 {
					r.Current = (r.Current - 1 + len(r.Matches)) % len(r.Matches)
					r.navigate()
				}
				return EventConsumed
			}
			if r.btnNext.Contains(mx, my) {
				if len(r.Matches) > 0 {
					r.Current = (r.Current + 1) % len(r.Matches)
					r.navigate()
				}
				return EventConsumed
			}
			if r.btnClose.Contains(mx, my) {
				if r.OnDismiss != nil {
					r.OnDismiss()
				}
				return EventConsumed
			}
		}
		return EventConsumed
	case *tcell.EventKey:
		return r.handleKey(tev)
	}
	return EventConsumed
}

func (r *ReplaceBarWidget) handleKey(kev *tcell.EventKey) EventResult {
	switch kev.Key() {
	case tcell.KeyEscape:
		if r.OnDismiss != nil {
			r.OnDismiss()
		}
		return EventConsumed
	case tcell.KeyTab, tcell.KeyBacktab:
		r.focusRow = 1 - r.focusRow
		return EventConsumed
	case tcell.KeyEnter:
		if r.focusRow == 0 {
			if len(r.Matches) > 0 {
				if kev.Modifiers()&tcell.ModShift != 0 {
					r.Current = (r.Current - 1 + len(r.Matches)) % len(r.Matches)
				} else {
					r.Current = (r.Current + 1) % len(r.Matches)
				}
				r.navigate()
			}
		} else {
			if len(r.Matches) > 0 && r.OnReplace != nil {
				r.OnReplace(r.Matches[r.Current], r.ReplaceInput.Text)
				r.search()
			}
		}
		return EventConsumed
	case tcell.KeyUp:
		if len(r.Matches) > 0 {
			r.Current = (r.Current - 1 + len(r.Matches)) % len(r.Matches)
			r.navigate()
		}
		return EventConsumed
	case tcell.KeyDown:
		if len(r.Matches) > 0 {
			r.Current = (r.Current + 1) % len(r.Matches)
			r.navigate()
		}
		return EventConsumed
	}

	if kev.Modifiers()&tcell.ModAlt != 0 && kev.Key() == tcell.KeyRune {
		switch term.KeyRune(kev) {
		case 'r':
			if r.OnReplaceAll != nil {
				r.OnReplaceAll(r.SearchInput.Text, r.ReplaceInput.Text, r.Options)
				r.search()
			}
			return EventConsumed
		case 'c':
			r.Options.CaseSensitive = !r.Options.CaseSensitive
			r.syncActions()
			r.Current = 0
			r.search()
			return EventConsumed
		case 'x':
			r.Options.UseRegex = !r.Options.UseRegex
			r.syncActions()
			r.Current = 0
			r.search()
			return EventConsumed
		}
	}

	inp := r.SearchInput
	if r.focusRow == 1 {
		inp = r.ReplaceInput
	}
	if inp.HandleEvent(tcell.NewEventKey(kev.Key(), kev.Str(), kev.Modifiers())) == EventConsumed {
		return EventConsumed
	}

	return EventConsumed
}
