package widgets

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type PagerConfig struct {
	Offset   int
	Limit    int
	Total    int
	OnChange func(offset int)
}

type PagerWidget struct {
	BaseWidget
	Config     PagerConfig
	focused    bool
	wasPressed bool
	hits       []pagerHit
}

type pagerHit struct {
	x0, x1 int
	offset int
}

var pagerArrows = [4]string{" « ", " ‹ ", " › ", " » "}

func NewPagerWidget(cfg PagerConfig) *PagerWidget {
	return &PagerWidget{Config: cfg}
}

func (p *PagerWidget) Focusable() bool   { return true }
func (p *PagerWidget) SetFocused(f bool) { p.focused = f }
func (p *PagerWidget) IsFocused() bool   { return p.focused }

func (p *PagerWidget) Height() int { return 1 + p.BoxOverheadH() }

func (p *PagerWidget) Width() int {
	w := textwidth.String(p.rangeText()) + 2 + 4
	for _, a := range pagerArrows {
		w += textwidth.String(a)
	}
	return w + p.BoxOverheadW()
}

func (p *PagerWidget) limit() int {
	if p.Config.Limit <= 0 {
		return max(p.Config.Total, 1)
	}
	return p.Config.Limit
}

func (p *PagerWidget) rangeText() string {
	c := p.Config
	if c.Total <= 0 {
		return "No items"
	}
	first := min(c.Offset+1, c.Total)
	last := min(c.Offset+p.limit(), c.Total)
	return fmt.Sprintf("%d–%d of %d", first, last, c.Total)
}

// targets returns the offsets for first, previous, next and last, with -1 for
// a direction that is not available from the current page.
func (p *PagerWidget) targets() [4]int {
	c, lim := p.Config, p.limit()
	t := [4]int{-1, -1, -1, -1}
	if c.Offset > 0 {
		t[0] = 0
		t[1] = max(c.Offset-lim, 0)
	}
	if c.Offset+lim < c.Total {
		t[2] = c.Offset + lim
		t[3] = (c.Total - 1) / lim * lim
	}
	return t
}

func (p *PagerWidget) Render(surface Surface) {
	inner := p.RenderBox(surface)
	w, h := inner.Size()
	p.hits = p.hits[:0]
	if w <= 0 || h <= 0 {
		return
	}
	targets := p.targets()
	x := 0
	arrow := func(i int) {
		style := term.StyleButton
		if targets[i] < 0 {
			style = term.StyleMuted
		}
		x0 := x
		x = inner.DrawText(x, 0, pagerArrows[i], w, style)
		if targets[i] >= 0 {
			p.hits = append(p.hits, pagerHit{x0: x0, x1: x, offset: targets[i]})
		}
		x++
	}
	arrow(0)
	arrow(1)
	textStyle := term.StyleDefault
	if p.focused {
		textStyle = term.StyleButtonFocused
	}
	x = inner.DrawText(x, 0, " "+p.rangeText()+" ", w, textStyle)
	x++
	arrow(2)
	arrow(3)
}

func (p *PagerWidget) HandleEvent(ev tcell.Event) EventResult {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		pressed := e.Buttons()&tcell.Button1 != 0
		fresh := pressed && !p.wasPressed
		p.wasPressed = pressed
		mx, my := e.Position()
		r := p.GetRect()
		if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
			return EventIgnored
		}
		if !fresh {
			return EventIgnored
		}
		ox, _ := p.contentOrigin()
		lx := mx - ox
		for _, hit := range p.hits {
			if lx >= hit.x0 && lx < hit.x1 {
				p.change(hit.offset)
				return EventConsumed
			}
		}
		return EventConsumed
	case *tcell.EventKey:
		if !p.focused {
			return EventIgnored
		}
		t := p.targets()
		switch e.Key() {
		case tcell.KeyHome:
			return p.changeIf(t[0])
		case tcell.KeyLeft, tcell.KeyPgUp:
			return p.changeIf(t[1])
		case tcell.KeyRight, tcell.KeyPgDn:
			return p.changeIf(t[2])
		case tcell.KeyEnd:
			return p.changeIf(t[3])
		}
	}
	return EventIgnored
}

func (p *PagerWidget) changeIf(offset int) EventResult {
	if offset >= 0 {
		p.change(offset)
	}
	return EventConsumed
}

func (p *PagerWidget) change(offset int) {
	if offset == p.Config.Offset {
		return
	}
	p.Config.Offset = offset
	if p.Config.OnChange != nil {
		p.Config.OnChange(offset)
	}
}
