package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

// Largest first; the layout takes the biggest one the tab can hold.
var welcomeTitles = [][]string{
	{
		"████████╗████████╗████████╗",
		"╚══██╔══╝╚══██╔══╝╚══██╔══╝",
		"   ██║      ██║      ██║   ",
		"   ██║      ██║      ██║   ",
		"   ██║      ██║      ██║   ",
		"   ╚═╝      ╚═╝      ╚═╝   ",
	},
	{
		"┏┳┓ ┏┳┓ ┏┳┓",
		" ┃   ┃   ┃ ",
		" ╹   ╹   ╹ ",
	},
	{"TTT Editor"},
}

const (
	welcomeSubtitle  = "Terminal Text Tool"
	welcomeHint      = "↑↓ select   enter open"
	welcomeCopyHint  = "↑↓ select   enter open   c copy path"
	welcomeCopyIcon  = "⧉"
	welcomeShortcutW = 6
	// Long favorite paths are cut from the left rather than stretching the list.
	welcomeMaxListW = 64
	// Fewest favorites kept on screen before the title gives way instead.
	welcomeMinFavorites = 3
)

type welcomeItem struct {
	label   string
	detail  string // shortcut or folder path, right-aligned and muted
	section string // heading drawn above this item, after a blank row
	tight   bool   // no spacing row above it, for lists that can grow long
	run     func()
	copy    func() // when set, a copy button ends the row and "c" runs it
}

// Each section heading takes a blank row plus the heading row.
const welcomeSectionRows = 2

type welcomeLayout struct {
	title    []string
	subtitle bool
	gap      int
	hint     bool
	top      int
	height   int
}

// layoutWelcome fits the page into h rows for n actions, tight of them with
// no spacing row above, under the given number of section headings. Space
// between the actions outranks the size of the title; with too little room
// even for the smallest title, only the actions are shown, without headings.
func layoutWelcome(w, h, n, tight, sections int) welcomeLayout {
	for _, gap := range []int{1, 0} {
		for _, title := range welcomeTitles {
			if textwidth.String(title[0]) > w {
				continue
			}
			l := welcomeLayout{title: title, subtitle: len(title) > 1, gap: gap}
			l.height = len(title) + 1 + gap + n + (n-1-tight)*gap + sections*welcomeSectionRows
			if l.subtitle {
				l.height += 2
			}
			if l.height > h {
				continue
			}
			if l.height+2 <= h {
				l.hint = true
				l.height += 2
			}
			l.top = (h - l.height) / 2
			return l
		}
	}
	return welcomeLayout{height: min(n, h), top: max((h-n)/2, 0)}
}

type welcomeView struct {
	widgets.BaseWidget
	items      []welcomeItem
	note       string // muted line under the items, laid out like a section
	selected   int
	wasPressed bool
	rowX, rowW int
	rowY       []int
	copyX      int
	secOff     int // first favorite shown when the list scrolls
}

func (v *welcomeView) Height() int { return 0 }
func (v *welcomeView) Width() int  { return 0 }

func (v *welcomeView) Render(surface widgets.Surface) {
	w, h := surface.Size()
	surface.Fill(term.Cell{Ch: ' ', Style: term.StyleDefault})
	if w <= 0 || h <= 0 {
		return
	}
	sections, tight := 0, 0
	if v.note != "" {
		sections++
	}
	hint := welcomeHint
	for _, item := range v.items {
		if item.section != "" {
			sections++
		}
		if item.tight {
			tight++
		}
		if item.copy != nil {
			hint = welcomeCopyHint
		}
	}
	secStart := len(v.items)
	for i, item := range v.items {
		if item.section != "" {
			secStart = i
			break
		}
	}
	// A long favorites list scrolls inside its section: the page keeps the
	// title, spacing and hint it has with welcomeMinFavorites of them.
	l := layoutWelcome(w, h, len(v.items), tight, sections)
	hidden := 0
	if excess := len(v.items) - secStart - welcomeMinFavorites; excess > 0 {
		want := layoutWelcome(w, h, len(v.items)-excess, tight-excess, sections)
		for hidden < excess && (len(l.title) != len(want.title) || l.gap != want.gap || l.hint != want.hint) {
			hidden++
			l = layoutWelcome(w, h, len(v.items)-hidden, tight-hidden, sections)
		}
	}
	shown := len(v.items) - secStart - hidden
	if hidden > 0 {
		if v.selected >= secStart {
			v.secOff = min(v.secOff, v.selected-secStart)
			v.secOff = max(v.secOff, v.selected-secStart-shown+1)
		}
		v.secOff = min(max(v.secOff, 0), hidden)
	} else {
		v.secOff = 0
	}
	y := l.top

	if len(l.title) > 0 {
		titleW := textwidth.String(l.title[0])
		for _, line := range l.title {
			surface.DrawText((w-titleW)/2, y, line, w, term.StyleBorderActive)
			y++
		}
		if l.subtitle {
			y++
			surface.DrawText((w-len(welcomeSubtitle))/2, y, welcomeSubtitle, w, term.StyleMuted)
			y++
		}
		y += 1 + l.gap
	}

	listW := 0
	for _, item := range v.items {
		rowW := textwidth.String(item.label) + welcomeShortcutW + textwidth.String(item.detail)
		if item.copy != nil {
			rowW += 2
		}
		listW = max(listW, rowW)
	}
	// Two cells of padding either side of the highlight.
	v.rowW = min(listW+4, welcomeMaxListW, w)
	v.rowX = max((w-v.rowW)/2, 0)
	v.copyX = v.GetRect().X + v.rowX + v.rowW - 3
	origin := v.GetRect()
	// Too short for every action: scroll so the selection stays visible.
	first, last := 0, len(v.items)
	if len(l.title) == 0 && len(v.items) > h {
		first = min(max(v.selected-h+1, 0), len(v.items)-h)
		last = first + h
	}
	v.rowY = v.rowY[:0]
	for i := range v.items {
		scrolledOut := i >= secStart && (i < secStart+v.secOff || i >= secStart+v.secOff+shown)
		if i < first || i >= last || scrolledOut {
			v.rowY = append(v.rowY, -1)
			continue
		}
		sectionTop := i == secStart+v.secOff
		if i > first && (!v.items[i].tight || sectionTop) {
			y += l.gap
		}
		if sectionTop && len(l.title) > 0 {
			heading := v.items[secStart].section
			if hidden > 0 {
				heading = fmt.Sprintf("%s  %d–%d of %d", heading, v.secOff+1, v.secOff+shown, shown+hidden)
			}
			y++
			surface.DrawText(v.rowX+2, y, heading, v.rowX+v.rowW-2, term.StyleMuted)
			y++
		}
		v.rowY = append(v.rowY, origin.Y+y)
		v.renderRow(surface, y, v.items[i], i == v.selected)
		y++
	}

	if v.note != "" && len(l.title) > 0 {
		y += l.gap
		surface.DrawText((w-textwidth.String(v.note))/2, y, v.note, w, term.StyleMuted)
	}

	if l.hint {
		surface.DrawText((w-textwidth.String(hint))/2, l.top+l.height-1, hint, w, term.StyleMuted)
	}
}

func (v *welcomeView) renderRow(surface widgets.Surface, y int, item welcomeItem, selected bool) {
	labelStyle, shortcutStyle := term.StyleDefault, term.StyleMuted
	if selected {
		labelStyle, shortcutStyle = term.StylePaletteSelected, term.StylePaletteSelected
		for x := v.rowX; x < v.rowX+v.rowW; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StylePaletteSelected})
		}
	}
	// DrawText takes an absolute column limit, not a width.
	left, right := v.rowX+2, v.rowX+v.rowW-2
	if item.copy != nil {
		surface.DrawText(right-1, y, welcomeCopyIcon, right, shortcutStyle)
		right -= 2
	}
	surface.DrawText(left, y, item.label, right, labelStyle)
	// Keep at least two columns between the label and the detail.
	if avail := right - left - textwidth.String(item.label) - 2; avail > 1 && item.detail != "" {
		detail := ui.TruncateLeft(item.detail, avail)
		surface.DrawText(right-textwidth.String(detail), y, detail, right, shortcutStyle)
	}
}

func (v *welcomeView) rowAt(mx, my int) int {
	r := v.GetRect()
	if mx < r.X+v.rowX || mx >= r.X+v.rowX+v.rowW {
		return -1
	}
	for i, y := range v.rowY {
		if y == my {
			return i
		}
	}
	return -1
}

func (v *welcomeView) HandleEvent(ev tcell.Event) widgets.EventResult {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyUp:
			v.selected = (v.selected + len(v.items) - 1) % len(v.items)
		case tcell.KeyDown:
			v.selected = (v.selected + 1) % len(v.items)
		case tcell.KeyHome:
			v.selected = 0
		case tcell.KeyEnd:
			v.selected = len(v.items) - 1
		case tcell.KeyEnter:
			v.items[v.selected].run()
		case tcell.KeyRune:
			if term.KeyRune(ev) != 'c' || v.items[v.selected].copy == nil {
				return widgets.EventIgnored
			}
			v.items[v.selected].copy()
		default:
			return widgets.EventIgnored
		}
		return widgets.EventConsumed
	case *tcell.EventMouse:
		switch {
		case ev.Buttons()&tcell.WheelUp != 0:
			v.selected = max(v.selected-1, 0)
			return widgets.EventConsumed
		case ev.Buttons()&tcell.WheelDown != 0:
			v.selected = min(v.selected+1, len(v.items)-1)
			return widgets.EventConsumed
		}
		pressed := ev.Buttons()&tcell.Button1 != 0
		fresh := pressed && !v.wasPressed
		v.wasPressed = pressed
		mx, my := ev.Position()
		i := v.rowAt(mx, my)
		if i < 0 {
			return widgets.EventIgnored
		}
		v.selected = i
		switch {
		case !fresh:
		case v.items[i].copy != nil && mx >= v.copyX-1:
			v.items[i].copy()
		default:
			v.items[i].run()
		}
		return widgets.EventConsumed
	}
	return widgets.EventIgnored
}
