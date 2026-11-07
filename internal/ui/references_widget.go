package ui

import (
	"fmt"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type ReferenceItem struct {
	File string
	Line int
	Col  int
	Text string
}

type ReferencesWidget struct {
	BaseWidget
	Items      []ReferenceItem
	selected   int
	scrollTop  int
	OnNavigate func(file string, line, col int)
}

func NewReferencesWidget() *ReferencesWidget {
	return &ReferencesWidget{}
}

func (r *ReferencesWidget) Focusable() bool { return true }

func (r *ReferencesWidget) SetItems(items []ReferenceItem) {
	r.Items = items
	r.selected = 0
	r.scrollTop = 0
}

func (r *ReferencesWidget) Render(surface Surface) {
	w, h := surface.Size()

	if len(r.Items) == 0 {
		msg := i18n.Text("references.noReferences")
		x := 1
		for _, ch := range msg {
			if x >= w {
				break
			}
			surface.SetCell(x, 0, term.Cell{Ch: ch, Style: term.StyleMuted})
			x++
		}
		return
	}

	if r.scrollTop > r.selected {
		r.scrollTop = r.selected
	}
	if r.selected >= r.scrollTop+h {
		r.scrollTop = r.selected - h + 1
	}

	for y := 0; y < h; y++ {
		idx := r.scrollTop + y
		if idx >= len(r.Items) {
			break
		}
		item := r.Items[idx]

		style := term.StyleDefault
		if idx == r.selected {
			style = term.StyleSidebarSelected
		}

		for x := 0; x < w; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		}

		x := 1

		loc := fmt.Sprintf("%s:%d:%d", filepath.Base(item.File), item.Line+1, item.Col+1)
		for _, ch := range loc {
			if x >= w {
				break
			}
			surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
			x++
		}
		x++

		textStyle := style
		if idx != r.selected {
			textStyle = term.StyleMuted
		}
		for _, ch := range item.Text {
			if x >= w-1 {
				break
			}
			surface.SetCell(x, y, term.Cell{Ch: ch, Style: textStyle})
			x++
		}
	}
}

func (r *ReferencesWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		switch tev.Key() {
		case tcell.KeyUp:
			if r.selected > 0 {
				r.selected--
			}
			return EventConsumed
		case tcell.KeyDown:
			if r.selected < len(r.Items)-1 {
				r.selected++
			}
			return EventConsumed
		case tcell.KeyEnter:
			if r.selected < len(r.Items) && r.OnNavigate != nil {
				item := r.Items[r.selected]
				r.OnNavigate(item.File, item.Line, item.Col)
			}
			return EventConsumed
		}
	case *tcell.EventMouse:
		btn := tev.Buttons()
		_, my := tev.Position()
		rect := r.GetRect()
		row := my - rect.Y
		idx := r.scrollTop + row
		if btn&tcell.Button1 != 0 && idx >= 0 && idx < len(r.Items) {
			r.selected = idx
			if r.OnNavigate != nil {
				item := r.Items[r.selected]
				r.OnNavigate(item.File, item.Line, item.Col)
			}
			return EventConsumed
		}
		if btn&tcell.WheelUp != 0 {
			if r.scrollTop > 0 {
				r.scrollTop--
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			if r.scrollTop < len(r.Items)-1 {
				r.scrollTop++
			}
			return EventConsumed
		}
	}
	return EventIgnored
}
