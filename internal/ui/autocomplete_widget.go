package ui

import (
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

type CompletionKind int

const (
	CompletionFunction CompletionKind = iota
	CompletionMethod
	CompletionVariable
	CompletionConstant
	CompletionType
	CompletionField
	CompletionKeyword
	CompletionSnippet
	CompletionModule
)

func (k CompletionKind) Symbol() rune { return '■' }

func (k CompletionKind) Style() term.Style {
	switch k {
	case CompletionFunction:
		return term.StyleSyntaxFunction
	case CompletionMethod:
		return term.StyleSyntaxBuiltin
	case CompletionVariable:
		return term.StyleSyntaxVariable
	case CompletionConstant:
		return term.StyleSyntaxNumber
	case CompletionType:
		return term.StyleSyntaxType
	case CompletionField:
		return term.StyleSyntaxTag
	case CompletionKeyword:
		return term.StyleSyntaxKeyword
	case CompletionSnippet:
		return term.StyleSyntaxString
	case CompletionModule:
		return term.StyleSyntaxComment
	default:
		return term.StyleSyntaxVariable
	}
}

type AdditionalEdit struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	NewText   string
}

type CompletionItem struct {
	Label           string
	Detail          string
	InsertText      string
	FilterText      string
	SortText        string
	Kind            CompletionKind
	AdditionalEdits []AdditionalEdit
}

func FilterCompletions(items []CompletionItem, prefix string) []CompletionItem {
	if prefix == "" {
		return items
	}
	lowerPrefix := strings.ToLower(prefix)
	var filtered []CompletionItem
	for _, it := range items {
		ft := it.FilterText
		if ft == "" {
			ft = it.Label
		}
		if strings.HasPrefix(strings.ToLower(ft), lowerPrefix) {
			filtered = append(filtered, it)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		si := filtered[i].SortText
		if si == "" {
			si = filtered[i].Label
		}
		sj := filtered[j].SortText
		if sj == "" {
			sj = filtered[j].Label
		}
		return si < sj
	})
	return filtered
}

type AutocompleteWidget struct {
	BaseWidget
	Items            []CompletionItem
	Selected         int
	scrollTop        int
	AnchorX          int
	AnchorY          int
	MaxVisible       int
	Borders          *term.BorderSet
	OnSelect         func(item CompletionItem)
	OnDismiss        func()
	firstEvent       bool
	scrollbar        widgets.VerticalScrollbar
	scrollbarCapture scrollbarCaptureState
}

const defaultMaxVisible = 10

func NewAutocompleteWidget(items []CompletionItem, x, y int) *AutocompleteWidget {
	return &AutocompleteWidget{
		Items:      items,
		AnchorX:    x,
		AnchorY:    y,
		MaxVisible: defaultMaxVisible,
		firstEvent: true,
	}
}

func (a *AutocompleteWidget) Focusable() bool { return false }

func (a *AutocompleteWidget) SetItems(items []CompletionItem) {
	a.Items = items
	if a.Selected >= len(items) {
		a.Selected = 0
	}
	a.scrollTop = 0
	a.ensureVisible()
}

func (a *AutocompleteWidget) visibleCount() int {
	if len(a.Items) < a.MaxVisible {
		return len(a.Items)
	}
	return a.MaxVisible
}

func (a *AutocompleteWidget) menuWidth() int {
	maxW := 0
	for _, it := range a.Items {
		w := len([]rune(it.Label)) + 5
		if it.Detail != "" {
			w += len([]rune(it.Detail)) + 2
		}
		if w > maxW {
			maxW = w
		}
	}
	if maxW < 12 {
		maxW = 12
	}
	if maxW > 60 {
		maxW = 60
	}
	return maxW
}

func (a *AutocompleteWidget) ensureVisible() {
	vis := a.visibleCount()
	if vis == 0 {
		return
	}
	if a.Selected < a.scrollTop {
		a.scrollTop = a.Selected
	}
	if a.Selected >= a.scrollTop+vis {
		a.scrollTop = a.Selected - vis + 1
	}
}

func (a *AutocompleteWidget) Render(surface Surface) {
	if len(a.Items) == 0 {
		_, invalidated := a.scrollbar.Render(surface, widgets.ScrollbarGeometry{}, widgets.NewScrollRange(0, 0, 0))
		a.scrollbarCapture.notify(invalidated)
		return
	}
	sw, sh := surface.Size()

	vis := a.visibleCount()
	menuW := a.menuWidth()
	hasScroll := len(a.Items) > vis
	if hasScroll {
		menuW++
	}
	menuH := vis + 2

	x := a.AnchorX
	if x+menuW > sw {
		x = sw - menuW
	}
	if x < 0 {
		x = 0
	}

	spaceBelow := sh - (a.AnchorY + 1)
	spaceAbove := a.AnchorY

	y := a.AnchorY + 1
	if menuH > spaceBelow && menuH <= spaceAbove {
		y = a.AnchorY - menuH
		if y < 0 {
			y = 0
		}
	}

	b := term.SingleBorderSet()
	if a.Borders != nil {
		b = *a.Borders
	}
	surface.DrawBorder(x, y, menuW, menuH, b, term.StyleBorder)

	contentW := menuW - 2
	if hasScroll {
		contentW--
	}

	for i := 0; i < vis; i++ {
		idx := a.scrollTop + i
		if idx >= len(a.Items) {
			break
		}
		it := a.Items[idx]
		row := y + 1 + i

		style := term.StylePaletteItem
		if idx == a.Selected {
			style = term.StylePaletteSelected
		}

		surface.ClearRect(x+1, row, menuW-2, 1, style)
		iconCell := term.Cell{Ch: it.Kind.Symbol(), Style: it.Kind.Style()}
		if idx == a.Selected {
			iconCell.BgStyle = term.StylePaletteSelected
		}
		surface.SetCell(x+2, row, iconCell)
		surface.DrawText(x+4, row, it.Label, x+1+contentW, style)

		if it.Detail != "" {
			detailRunes := []rune(it.Detail)
			detailX := x + 1 + contentW - len(detailRunes)
			labelEnd := x + 4 + len([]rune(it.Label)) + 1
			if detailX < labelEnd {
				detailX = labelEnd
			}
			cell := term.Cell{Style: term.StyleSyntaxComment}
			if idx == a.Selected {
				cell.BgStyle = term.StylePaletteSelected
			}
			for _, ch := range it.Detail {
				if detailX >= x+1+contentW {
					break
				}
				cell.Ch = ch
				surface.SetCell(detailX, row, cell)
				detailX++
			}
		}
	}

	ox, oy := surface.Origin()
	trackLength := 0
	if hasScroll {
		trackLength = vis
	}
	_, invalidated := a.scrollbar.Render(
		surface,
		widgets.NewScrollbarGeometry(
			Rect{X: x + menuW - 2, Y: y + 1, W: 1, H: trackLength},
			Rect{X: ox + x + menuW - 2, Y: oy + y + 1, W: 1, H: trackLength},
		),
		widgets.NewScrollRange(trackLength, len(a.Items), a.scrollTop),
	)
	a.scrollbarCapture.notify(invalidated)

	a.SetRect(Rect{X: ox + x, Y: oy + y, W: menuW, H: menuH})
}

func (a *AutocompleteWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		switch tev.Key() {
		case tcell.KeyEscape:
			if a.OnDismiss != nil {
				a.OnDismiss()
			}
			return EventConsumed
		case tcell.KeyUp:
			a.moveSelection(-1)
			return EventConsumed
		case tcell.KeyDown:
			a.moveSelection(1)
			return EventConsumed
		case tcell.KeyEnter, tcell.KeyTab:
			if a.Selected >= 0 && a.Selected < len(a.Items) {
				if a.OnSelect != nil {
					a.OnSelect(a.Items[a.Selected])
				}
			}
			return EventConsumed
		}
		return EventIgnored

	case *tcell.EventMouse:
		btn := tev.Buttons()
		mx, my := tev.Position()
		r := a.GetRect()

		if a.firstEvent {
			a.firstEvent = false
			return EventConsumed
		}

		if btn&tcell.WheelUp != 0 {
			if a.scrollTop > 0 {
				a.scrollTop--
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			max := len(a.Items) - a.visibleCount()
			if a.scrollTop < max {
				a.scrollTop++
			}
			return EventConsumed
		}

		if newTop, result := a.scrollbar.HandleEvent(ev); result != EventIgnored {
			a.scrollTop = newTop
			return result
		}

		if btn&tcell.Button1 != 0 {
			if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
				if a.OnDismiss != nil {
					a.OnDismiss()
				}
				return EventConsumed
			}
			itemIdx := my - r.Y - 1 + a.scrollTop
			if itemIdx >= 0 && itemIdx < len(a.Items) {
				a.Selected = itemIdx
				if a.OnSelect != nil {
					a.OnSelect(a.Items[a.Selected])
				}
			}
			return EventConsumed
		}

		if btn == tcell.ButtonNone {
			if mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H {
				itemIdx := my - r.Y - 1 + a.scrollTop
				if itemIdx >= 0 && itemIdx < len(a.Items) {
					a.Selected = itemIdx
				}
				return EventConsumed
			}
		}
	}
	return EventIgnored
}

func (a *AutocompleteWidget) CancelPointerCapture() bool {
	return a.scrollbarCapture.cancel(&a.scrollbar)
}

func (a *AutocompleteWidget) InvalidatePointerInteraction() bool {
	return a.CancelPointerCapture()
}

func (a *AutocompleteWidget) OwnsPointerCapture() bool {
	return a.scrollbarCapture.owns(&a.scrollbar)
}

func (a *AutocompleteWidget) SetPointerCaptureInvalidated(invalidated func()) {
	a.scrollbarCapture.invalidated = invalidated
}

func (a *AutocompleteWidget) moveSelection(dir int) {
	n := len(a.Items)
	if n == 0 {
		return
	}
	a.Selected += dir
	if a.Selected < 0 {
		a.Selected = 0
	}
	if a.Selected >= n {
		a.Selected = n - 1
	}
	a.ensureVisible()
}
