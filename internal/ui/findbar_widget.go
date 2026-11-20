package ui

import (
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

type FindMatch struct {
	Line, Col, Len int
}

type FindBarWidget struct {
	BaseWidget
	Input             *widgets.InputWidget
	Matches           []FindMatch
	Current           int
	Options           SearchOptions
	Borders           *term.BorderSet
	OnSearch          func(query string, opts SearchOptions) []FindMatch
	OnNavigate        func(match FindMatch)
	OnEditorTabChange func()
	OnDismiss         func()
	Debounce          Debouncer
	focused           bool
	OnToggleReplace   func()
	findBarLayout
}

func NewFindBarWidget() *FindBarWidget {
	f := &FindBarWidget{focused: true}
	f.Input = newPanelInput()
	f.Input.Config.Prefix = ""
	f.Input.Config.Placeholder = i18n.Text("search.search")
	f.Input.Config.OnChange = func(string) {
		f.Current = 0
		f.search()
	}
	f.Input.Actions = []widgets.InputAction{
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

func (f *FindBarWidget) Focusable() bool { return true }

func (f *FindBarWidget) FocusedInput() *widgets.InputWidget {
	if f.focused {
		return f.Input
	}
	return nil
}

func (f *FindBarWidget) Focus() {
	f.focused = true
}

func (f *FindBarWidget) CursorPosition() (int, int, bool) {
	if !f.focused {
		return 0, 0, false
	}
	return f.Input.CursorPosition()
}

func (f *FindBarWidget) Render(surface Surface) {
	f.syncActions()
	f.findBarLayout = renderFindBar(surface, f.Borders, f.Input.Text(), f.Current, len(f.Matches), false, f.OnToggleReplace != nil)
	renderFindBarInput(surface, f.Input, f.inputX, f.barRect.Y+1, f.inputW)
}

func (f *FindBarWidget) search() {
	f.Debounce.Schedule(func() {
		f.doSearch()
	})
}

func (f *FindBarWidget) doSearch() {
	if f.OnSearch != nil {
		f.Matches = f.OnSearch(f.Input.Text(), f.Options)
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
	if f.OnSearch == nil || f.Input.Text() == "" {
		return
	}
	f.Matches = f.OnSearch(f.Input.Text(), f.Options)
	f.Current = max(min(f.Current, len(f.Matches)-1), 0)
}

func (f *FindBarWidget) Query() string { return f.Input.Text() }

func (f *FindBarWidget) CurrentMatch() int { return f.Current }

func (f *FindBarWidget) NavigateMatch(delta int) {
	if len(f.Matches) == 0 {
		return
	}
	f.Current = (f.Current + delta + len(f.Matches)) % len(f.Matches)
	f.navigate()
}

func (f *FindBarWidget) navigate() {
	if f.OnNavigate != nil && len(f.Matches) > 0 {
		f.OnNavigate(f.Matches[f.Current])
	}
}

func (f *FindBarWidget) HandleEvent(ev tcell.Event) EventResult {
	if result := handleInputDrag(ev, f.Input); result != EventIgnored {
		return result
	}
	switch tev := ev.(type) {
	case *tcell.EventMouse:
		if tev.Buttons()&tcell.Button1 != 0 {
			mx, my := tev.Position()
			barX, barY, barW, barH := f.barRect.X, f.barRect.Y, f.barRect.W, f.barRect.H
			r := f.GetRect()
			localX := mx - r.X
			localY := my - r.Y
			if localY >= barY && localY < barY+barH && localX >= barX && localX < barX+barW {
				f.focused = true
				if f.btnToggle.Contains(mx, my) && f.OnToggleReplace != nil {
					f.OnToggleReplace()
					return EventConsumed
				}
				row := barY + 1
				if localY == row && localX >= f.inputX && localX < f.inputX+f.inputW {
					return f.Input.HandleEvent(tev)
				}
				if f.btnPrev.Contains(mx, my) {
					f.NavigateMatch(-1)
				}
				if f.btnNext.Contains(mx, my) {
					f.NavigateMatch(1)
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
		if kev.Modifiers()&tcell.ModShift != 0 {
			f.NavigateMatch(-1)
		} else {
			f.NavigateMatch(1)
		}
		return EventConsumed
	case tcell.KeyUp:
		f.NavigateMatch(-1)
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

func (f *FindBarWidget) CancelPointerCapture() bool { return cancelInputDrags(f.Input) }

func (f *FindBarWidget) OwnsPointerCapture() bool { return ownsInputDrag(f.Input) }
