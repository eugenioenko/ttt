package ui

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

const (
	verticalTabBarBorderX = 4
	VerticalTabBarWidth   = verticalTabBarBorderX + 1
)

const verticalTabLabelMaxCols = 2

type VerticalTabBar struct {
	BaseWidget
	Count  int
	Active int
	// Labels are optional per-tab badges (e.g. "pi", "sh"). The strip is 4
	// content cells wide, so longer labels are truncated here; the full label
	// lives on the tab (app.TerminalTab.Label) for tooling and the debug dump.
	Labels   []string
	OnSelect func(index int)
	Borders  *term.BorderSet
}

func NewVerticalTabBar() *VerticalTabBar {
	return &VerticalTabBar{}
}

func (v *VerticalTabBar) Focusable() bool { return false }

func (v *VerticalTabBar) Render(surface Surface) {
	_, h := surface.Size()

	vertical := '│'
	if v.Borders != nil {
		vertical = v.Borders.Vertical
	}

	borderX := verticalTabBarBorderX
	for y := 0; y < h; y++ {
		for x := 0; x < borderX; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StyleDefault})
		}
		surface.SetCell(borderX, y, term.Cell{Ch: vertical, Style: term.StyleBorder})
	}

	for i := 0; i < v.Count; i++ {
		if i >= h {
			break
		}
		style := term.StyleInactiveTab
		if i == v.Active {
			style = term.StyleActiveTab
		}
		n := i + 1
		var label string
		if i < len(v.Labels) && v.Labels[i] != "" {
			label = fmt.Sprintf("[%s]", truncateTabLabel(v.Labels[i]))
		} else if n < 10 {
			label = fmt.Sprintf("[>%d]", n)
		} else {
			label = fmt.Sprintf("[%d]", n)
		}
		surface.DrawText(0, i, label, borderX, style)
	}
}

// truncateTabLabel fits a label into the strip's 4-cell content area ("[pi]").
func truncateTabLabel(label string) string {
	cols := 0
	for i, ch := range label {
		w := textwidth.Rune(ch)
		if cols+w > verticalTabLabelMaxCols {
			return label[:i]
		}
		cols += w
	}
	return label
}

func (v *VerticalTabBar) HandleEvent(ev tcell.Event) EventResult {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}
	if mev.Buttons()&tcell.Button1 == 0 {
		return EventIgnored
	}
	_, my := mev.Position()
	r := v.GetRect()
	ly := my - r.Y

	if ly >= 0 && ly < v.Count && v.OnSelect != nil {
		v.OnSelect(ly)
		return EventConsumed
	}
	return EventConsumed
}
