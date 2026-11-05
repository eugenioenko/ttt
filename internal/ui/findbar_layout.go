package ui

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

type findBarLayout struct {
	barRect                               Rect
	inputX, inputW                        int
	btnToggle, btnPrev, btnNext, btnClose HitRegion
}

func renderFindBar(surface Surface, borders *term.BorderSet, query string, current, matches int, expanded, toggle bool) findBarLayout {
	sw, _ := surface.Size()
	w := max(min(40, sw-4), 0)
	l := findBarLayout{barRect: Rect{X: sw - w - 1, Y: 4, W: w, H: 3}}
	if expanded {
		l.barRect.H++
	}
	b := term.SingleBorderSet()
	if borders != nil {
		b = *borders
	}
	r := l.barRect
	surface.DrawBorder(r.X, r.Y, r.W, r.H, b, term.StyleBorder)
	surface.ClearRect(r.X+1, r.Y+1, max(w-2, 0), r.H-2, term.StyleInput)
	ox, oy := surface.Origin()
	row := r.Y + 1
	l.inputX = r.X + 4
	if toggle {
		icon := icons.ChevronRight
		if expanded {
			icon = icons.ChevronDown
		}
		end := surface.DrawText(r.X+2, row, string(icons.Rune(icon)), r.X+w-1, term.StyleMuted)
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
	suffix := info + nav + " ✕ "
	l.inputW = max(w-5-textwidth.String(suffix), 0)
	x := surface.DrawText(l.inputX+l.inputW, row, info, r.X+w-1, term.StyleMuted)
	if nav != "" {
		x, l.btnPrev = drawFindBarButton(surface, x, row, "▲", r.X+w-1)
		x, l.btnNext = drawFindBarButton(surface, x, row, "▼", r.X+w-1)
	}
	_, l.btnClose = drawFindBarButton(surface, x, row, "✕", r.X+w-1)
	return l
}

func drawFindBarButton(surface Surface, x, y int, label string, limit int) (int, HitRegion) {
	x = surface.DrawText(x, y, " ", limit, term.StyleMuted)
	start := x
	x = surface.DrawText(x, y, label, limit, term.StyleMuted)
	ox, oy := surface.Origin()
	return x, HitRegion{X: ox + start, Y: oy + y, W: x - start}
}
