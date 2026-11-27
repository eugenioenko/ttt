package ui

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type findBarLayout struct {
	barRect                               Rect
	inputX, inputW                        int
	btnToggle, btnPrev, btnNext, btnClose HitRegion
}

const minFindBarWidth = 24

func findBarAnchor(fn func() Rect) Rect {
	if fn == nil {
		return Rect{}
	}
	return fn()
}

func findBarRect(surface Surface, anchor Rect, expanded bool) Rect {
	sw, sh := surface.Size()
	h := 3
	if expanded {
		h++
	}
	if anchor.W > 0 && anchor.H > 0 {
		ox, oy := surface.Origin()
		left := max(anchor.X-ox, 0)
		right := min(anchor.X-ox+anchor.W, sw)
		if aw := right - left; aw > 0 {
			w := min(40, max(aw, min(sw, minFindBarWidth)))
			x := max(right-w, 0)
			y := max(min(anchor.Y-oy, sh-h), 0)
			return Rect{X: x, Y: y, W: w, H: h}
		}
	}
	w := max(min(40, sw-4), 0)
	return Rect{X: sw - w - 1, Y: 4, W: w, H: h}
}

func renderFindBar(surface Surface, borders *term.BorderSet, anchor Rect, query string, current, matches int, expanded, toggle bool) findBarLayout {
	l := findBarLayout{barRect: findBarRect(surface, anchor, expanded)}
	r := l.barRect
	w := r.W
	drawFloatingBar(surface, borders, r)
	ox, oy := surface.Origin()
	row := r.Y + 1
	gutterW := min(3, max(w-3, 0))
	l.inputX = r.X + 1 + gutterW
	if toggle && gutterW > 0 {
		icon := icons.ChevronRight
		if expanded {
			icon = icons.ChevronDown
		}
		end := surface.DrawText(r.X+1+gutterW/2, row, string(icons.Rune(icon)), l.inputX, term.StyleMuted)
		l.btnToggle = HitRegion{X: ox + r.X + 1, Y: oy + row, W: end - (r.X + 1)}
	}
	info := ""
	if query != "" {
		display := 0
		if matches > 0 {
			display = current + 1
		}
		info = fmt.Sprintf(" %d/%d", display, matches)
	}
	nav := ""
	if matches > 0 || expanded {
		nav = " ▲ ▼"
	}
	available := max(r.X+w-1-l.inputX, 0)
	suffixBudget := available - min(4, available)
	closeLabel := " ✕ "
	if textwidth.String(info+nav+closeLabel) > suffixBudget {
		info = ""
	}
	if textwidth.String(nav+closeLabel) > suffixBudget {
		nav = ""
	}
	if textwidth.String(closeLabel) > suffixBudget {
		closeLabel = ""
	}
	l.inputW = available - textwidth.String(info+nav+closeLabel)
	x := surface.DrawText(l.inputX+l.inputW, row, info, r.X+w-1, term.StyleMuted)
	if nav != "" {
		x, l.btnPrev = drawFindBarButton(surface, x, row, "▲", r.X+w-1)
		x, l.btnNext = drawFindBarButton(surface, x, row, "▼", r.X+w-1)
	}
	if closeLabel != "" {
		_, l.btnClose = drawFindBarButton(surface, x, row, "✕", r.X+w-1)
	}
	return l
}

func drawFindBarButton(surface Surface, x, y int, label string, limit int) (int, HitRegion) {
	x = surface.DrawText(x, y, " ", limit, term.StyleMuted)
	start := x
	x = surface.DrawText(x, y, label, limit, term.StyleMuted)
	ox, oy := surface.Origin()
	return x, HitRegion{X: ox + start, Y: oy + y, W: x - start}
}

func renderFindBarInput(surface Surface, input *widgets.InputWidget, x, y, width int) {
	actions := input.Actions
	if width < input.ActionsWidth()+4 {
		input.Actions = nil
	}
	renderPanelInput(surface, input, x, y, width)
	input.Actions = actions
}

func drawFloatingBar(surface Surface, borders *term.BorderSet, r Rect) {
	b := term.SingleBorderSet()
	if borders != nil {
		b = *borders
	}
	surface.DrawBorder(r.X, r.Y, r.W, r.H, b, term.StyleBorder)
	surface.ClearRect(r.X+1, r.Y+1, max(r.W-2, 0), r.H-2, term.StyleInput)
}
