package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"log/slog"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
)

type tabSpan struct {
	start, end int
	label      string
	active     bool
}

type Tab struct {
	Name     string
	Dirty    bool
	Active   bool
	Closable bool
}

type TabBarWidget struct {
	BaseWidget
	Tabs            []Tab
	Borders         *term.BorderSet
	ScrollOffset    int
	MoreButton      *MoreButtonWidget
	OnTabClick      func(index int)
	OnTabClose      func(index int)
	OnTabRightClick func(index, screenX, screenY int)
	tabSpans        []tabSpan
}

func NewTabBarWidget() *TabBarWidget {
	return &TabBarWidget{}
}

func (t *TabBarWidget) SetTabs(tabs []Tab) {
	t.Tabs = tabs
}

func (t *TabBarWidget) tabLabel(tab Tab) string {
	name := filepath.Base(tab.Name)
	label := " "
	if tab.Dirty {
		label += "● "
	}
	label += name
	if tab.Active && tab.Closable {
		label += " x"
	}
	label += " "
	return label
}

func (t *TabBarWidget) Render(surface *RenderSurface) {
	w, _ := surface.Size()

	b := term.SingleBorderSet()
	if t.Borders != nil {
		b = *t.Borders
	}
	bs := term.StyleBorder

	// Compute spans: active tab needs 2 extra cols for │ │ side borders
	spans := make([]tabSpan, len(t.Tabs))
	pos := 0
	activeIdx := -1
	for i, tab := range t.Tabs {
		label := t.tabLabel(tab)
		labelW := len([]rune(label))
		spanW := labelW
		if tab.Active {
			spanW += 2
		}
		spans[i] = tabSpan{start: pos, end: pos + spanW, label: label, active: tab.Active}
		if tab.Active {
			activeIdx = i
		}
		pos += spanW
	}
	t.tabSpans = spans

	// Scroll to keep active tab visible
	if activeIdx >= 0 {
		s := spans[activeIdx]
		if s.end-t.ScrollOffset > w {
			t.ScrollOffset = s.end - w
		}
		if s.start < t.ScrollOffset {
			t.ScrollOffset = s.start
		}
	}
	if t.ScrollOffset < 0 {
		t.ScrollOffset = 0
	}

	// Row 0: top of active tab ┌───┐, spaces elsewhere
	for x := 0; x < w; x++ {
		surface.SetCell(x, 0, term.Cell{Ch: ' '})
	}
	if activeIdx >= 0 {
		s := spans[activeIdx]
		sx := s.start - t.ScrollOffset
		ex := s.end - t.ScrollOffset
		if sx >= 0 && sx < w {
			surface.SetCell(sx, 0, term.Cell{Ch: b.TopLeft, Style: bs})
		}
		for x := sx + 1; x < ex-1; x++ {
			if x >= 0 && x < w {
				surface.SetCell(x, 0, term.Cell{Ch: b.Horizontal, Style: bs})
			}
		}
		if ex-1 > sx && ex-1 >= 0 && ex-1 < w {
			surface.SetCell(ex-1, 0, term.Cell{Ch: b.TopRight, Style: bs})
		}
	}

	// Row 1: tab labels, active tab has │ sides
	for x := 0; x < w; x++ {
		surface.SetCell(x, 1, term.Cell{Ch: ' '})
	}
	for i, s := range spans {
		sx := s.start - t.ScrollOffset
		ex := s.end - t.ScrollOffset
		dirty := t.Tabs[i].Dirty
		if s.active {
			if sx >= 0 && sx < w {
				surface.SetCell(sx, 1, term.Cell{Ch: b.Vertical, Style: bs})
			}
			for ci, ch := range []rune(s.label) {
				style := term.StyleActiveTab
				if dirty && ch == '●' {
					style = term.StyleWarning
				}
				x := sx + 1 + ci
				if x >= 0 && x < w {
					surface.SetCell(x, 1, term.Cell{Ch: ch, Style: style})
				}
			}
			if ex-1 >= 0 && ex-1 < w {
				surface.SetCell(ex-1, 1, term.Cell{Ch: b.Vertical, Style: bs})
			}
		} else {
			for ci, ch := range []rune(s.label) {
				style := term.StyleInactiveTab
				if dirty && ch == '●' {
					style = term.StyleWarning
				}
				x := sx + ci
				if x >= 0 && x < w {
					surface.SetCell(x, 1, term.Cell{Ch: ch, Style: style})
				}
			}
		}
	}

	// Row 2: baseline ─┘  └─, horizontal line with gap for active tab
	for x := 0; x < w; x++ {
		surface.SetCell(x, 2, term.Cell{Ch: b.Horizontal, Style: bs})
	}
	if activeIdx >= 0 {
		s := spans[activeIdx]
		sx := s.start - t.ScrollOffset
		ex := s.end - t.ScrollOffset
		if sx >= 0 && sx < w {
			surface.SetCell(sx, 2, term.Cell{Ch: b.BottomRight, Style: bs})
		}
		for x := sx + 1; x < ex-1; x++ {
			if x >= 0 && x < w {
				surface.SetCell(x, 2, term.Cell{Ch: ' '})
			}
		}
		if ex-1 > sx && ex-1 >= 0 && ex-1 < w {
			surface.SetCell(ex-1, 2, term.Cell{Ch: b.BottomLeft, Style: bs})
		}
	}

	if t.MoreButton != nil && w >= 5 {
		r := t.GetRect()
		t.MoreButton.SetRect(Rect{X: r.X + w - 4, Y: r.Y + 1, W: 3, H: 1})
		moreSurface := surface.Sub(Rect{X: w - 4, Y: 1, W: 3, H: 1})
		t.MoreButton.Render(moreSurface)
	}
}

func (t *TabBarWidget) HandleEvent(ev tcell.Event) EventResult {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}
	r := t.GetRect()
	mx, my := mev.Position()
	btn := mev.Buttons()

	slog.Debug("tabBar", "mx", mx, "my", my, "btn", btn, "rect", r, "hasMore", t.MoreButton != nil)

	if t.MoreButton != nil {
		if t.MoreButton.HandleEvent(ev) == EventConsumed {
			return EventConsumed
		}
	}

	if my < r.Y || my >= r.Y+r.H || mx < r.X || mx >= r.X+r.W {
		slog.Debug("tabBar", "action", "outOfBounds")
		return EventIgnored
	}

	if btn&tcell.Button2 != 0 && t.OnTabRightClick != nil {
		localX := mx - r.X + t.ScrollOffset
		for i, s := range t.tabSpans {
			if localX >= s.start && localX < s.end {
				t.OnTabRightClick(i, mx, my)
				return EventConsumed
			}
		}
	}

	if btn&tcell.Button1 == 0 {
		return EventIgnored
	}

	localX := mx - r.X + t.ScrollOffset
	for i, s := range t.tabSpans {
		if localX >= s.start && localX < s.end {
			if s.active && t.OnTabClose != nil {
				closeX := s.end - 3
				if localX == closeX {
					t.OnTabClose(i)
					return EventConsumed
				}
			}
			if t.OnTabClick != nil {
				t.OnTabClick(i)
			}
			return EventConsumed
		}
	}
	return EventIgnored
}
