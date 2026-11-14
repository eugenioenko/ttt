package ui

import (
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

type ReplaceBarWidget struct {
	BaseWidget
	SearchInput  *widgets.InputWidget
	ReplaceInput *widgets.InputWidget
	Matches      []FindMatch
	Current      int
	Options      SearchOptions
	Borders      *term.BorderSet
	OnSearch     func(query string, opts SearchOptions) []FindMatch
	OnNavigate   func(match FindMatch)
	OnReplace    func(match FindMatch, replacement string)
	OnReplaceAll func(query, replacement string, opts SearchOptions)
	OnDismiss    func()
	focused      bool
	focusRow     int
	OnToggleFind func()
	findBarLayout
}

func NewReplaceBarWidget() *ReplaceBarWidget {
	r := &ReplaceBarWidget{focused: true}
	r.SearchInput = newPanelInput()
	r.SearchInput.Config.Prefix = ""
	r.SearchInput.Config.Placeholder = i18n.Text("search.search")
	r.SearchInput.Config.OnChange = func(string) {
		r.Current = 0
		r.search()
	}
	r.SearchInput.Actions = []widgets.InputAction{
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

	r.ReplaceInput = newPanelInput()
	r.ReplaceInput.Config.Prefix = ""
	r.ReplaceInput.Config.Placeholder = i18n.Text("general.replace")
	r.ReplaceInput.Actions = []widgets.InputAction{
		{Label: string(icons.Rune(icons.Replace)), OnClick: r.replaceCurrent},
		{Label: i18n.Text("search.all"), OnClick: func() {
			if r.OnReplaceAll != nil {
				r.OnReplaceAll(r.SearchInput.Text(), r.ReplaceInput.Text(), r.Options)
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

func (r *ReplaceBarWidget) Focusable() bool { return true }

func (r *ReplaceBarWidget) Focus() {
	r.focused = true
}

func (r *ReplaceBarWidget) FocusedInput() *widgets.InputWidget {
	if !r.focused {
		return nil
	}
	if r.focusRow == 1 {
		return r.ReplaceInput
	}
	return r.SearchInput
}

func (r *ReplaceBarWidget) CursorPosition() (int, int, bool) {
	if !r.focused {
		return 0, 0, false
	}
	return r.FocusedInput().CursorPosition()
}

func (r *ReplaceBarWidget) Render(surface Surface) {
	r.syncActions()
	if icons.ActiveIsNerd() {
		r.ReplaceInput.Actions[1].Label = string(icons.Rune(icons.ReplaceAll))
	} else {
		r.ReplaceInput.Actions[1].Label = i18n.Text("search.all")
	}
	r.findBarLayout = renderFindBar(surface, r.Borders, r.SearchInput.Text(), r.Current, len(r.Matches), true, r.OnToggleFind != nil)
	renderFindBarInput(surface, r.SearchInput, r.inputX, r.barRect.Y+1, r.inputW)
	renderFindBarInput(surface, r.ReplaceInput, r.inputX, r.barRect.Y+2, max(r.barRect.X+r.barRect.W-1-r.inputX, 0))
}

func (r *ReplaceBarWidget) search() {
	if r.OnSearch != nil {
		r.Matches = r.OnSearch(r.SearchInput.Text(), r.Options)
		if len(r.Matches) > 0 {
			if r.Current >= len(r.Matches) {
				r.Current = 0
			}
			r.navigate()
		}
	}
}

// Refresh re-runs the query against content that changed under the bar,
// keeping the current match index where it still exists, without navigating.
func (r *ReplaceBarWidget) Refresh() {
	if r.OnSearch == nil || r.SearchInput.Text() == "" {
		return
	}
	r.Matches = r.OnSearch(r.SearchInput.Text(), r.Options)
	r.Current = max(min(r.Current, len(r.Matches)-1), 0)
}

func (r *ReplaceBarWidget) Query() string { return r.SearchInput.Text() }

func (r *ReplaceBarWidget) CurrentMatch() int { return r.Current }

func (r *ReplaceBarWidget) NavigateMatch(delta int) {
	if len(r.Matches) == 0 {
		return
	}
	r.Current = (r.Current + delta + len(r.Matches)) % len(r.Matches)
	r.navigate()
}

func (r *ReplaceBarWidget) replaceCurrent() {
	r.Refresh()
	if len(r.Matches) == 0 || r.OnReplace == nil {
		return
	}
	match, replacement := r.Matches[r.Current], r.ReplaceInput.Text()
	r.OnReplace(match, replacement)
	if r.OnSearch == nil {
		return
	}
	r.Matches = r.OnSearch(r.SearchInput.Text(), r.Options)
	r.Current = 0
	end := match.Col + len([]rune(replacement))
	for i, m := range r.Matches {
		if m.Line > match.Line || m.Line == match.Line && m.Col >= end {
			r.Current = i
			break
		}
	}
	r.navigate()
}

func (r *ReplaceBarWidget) navigate() {
	if r.OnNavigate != nil && len(r.Matches) > 0 {
		r.OnNavigate(r.Matches[r.Current])
	}
}

func (r *ReplaceBarWidget) HandleEvent(ev tcell.Event) EventResult {
	if result := handleInputDrag(ev, r.SearchInput, r.ReplaceInput); result != EventIgnored {
		return result
	}
	switch tev := ev.(type) {
	case *tcell.EventMouse:
		if tev.Buttons()&tcell.Button1 != 0 {
			mx, my := tev.Position()
			rect := r.GetRect()
			localX := mx - rect.X
			localY := my - rect.Y
			barX, barY, barW, barH := r.barRect.X, r.barRect.Y, r.barRect.W, r.barRect.H
			if localX < barX || localX >= barX+barW || localY < barY || localY >= barY+barH {
				r.focused = false
				return EventIgnored
			}
			r.focused = true
			if r.btnToggle.Contains(mx, my) && r.OnToggleFind != nil {
				r.OnToggleFind()
				return EventConsumed
			}

			if r.btnPrev.Contains(mx, my) {
				r.NavigateMatch(-1)
				return EventConsumed
			}
			if r.btnNext.Contains(mx, my) {
				r.NavigateMatch(1)
				return EventConsumed
			}
			if r.btnClose.Contains(mx, my) {
				if r.OnDismiss != nil {
					r.OnDismiss()
				}
				return EventConsumed
			}

			findRow := barY + 1
			replRow := barY + 2

			if localY == findRow && localX >= r.inputX && localX < barX+barW-1 {
				r.focusRow = 0
				if result := r.SearchInput.HandleEvent(tev); result != EventIgnored {
					return result
				}
				return EventConsumed
			}
			if localY == replRow && localX >= r.inputX && localX < barX+barW-1 {
				r.focusRow = 1
				return r.ReplaceInput.HandleEvent(tev)
			}

			return EventConsumed
		}
		return EventIgnored
	case *tcell.EventKey:
		return r.handleKey(tev)
	}
	return EventIgnored
}

func (r *ReplaceBarWidget) handleKey(kev *tcell.EventKey) EventResult {
	if !r.focused && kev.Key() != tcell.KeyEscape {
		return EventIgnored
	}

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
			if kev.Modifiers()&tcell.ModShift != 0 {
				r.NavigateMatch(-1)
			} else {
				r.NavigateMatch(1)
			}
		} else {
			r.replaceCurrent()
		}
		return EventConsumed
	case tcell.KeyUp:
		r.NavigateMatch(-1)
		return EventConsumed
	case tcell.KeyDown:
		r.NavigateMatch(1)
		return EventConsumed
	}

	if kev.Modifiers()&tcell.ModAlt != 0 && kev.Key() == tcell.KeyRune {
		switch term.KeyRune(kev) {
		case 'r':
			if r.OnReplaceAll != nil {
				r.OnReplaceAll(r.SearchInput.Text(), r.ReplaceInput.Text(), r.Options)
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

	return EventIgnored
}

func (r *ReplaceBarWidget) CancelPointerCapture() bool {
	return cancelInputDrags(r.SearchInput, r.ReplaceInput)
}

func (r *ReplaceBarWidget) OwnsPointerCapture() bool {
	return ownsInputDrag(r.SearchInput, r.ReplaceInput)
}
