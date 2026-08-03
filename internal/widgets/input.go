package widgets

import (
	"strings"
	"time"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type InputConfig struct {
	Prefix      string     `json:"prefix,omitempty"`
	Placeholder string     `json:"placeholder,omitempty"`
	Bordered    bool       `json:"bordered"`
	Style       term.Style `json:"-"`

	OnChange func(text string)
	OnSubmit func(text string)
}

type InputWidget struct {
	BaseWidget
	Config        InputConfig
	text          string
	cursorPos     int
	scrollOffset  int
	selStart      int
	selEnd        int
	focused       bool
	lastClickTime int64
	lastClickPos  int
	clickCount    int
}

func NewInputWidget(config InputConfig) *InputWidget {
	if !config.Bordered && config.Prefix == "" {
		config.Prefix = " ❯ "
	}
	return &InputWidget{
		Config:   config,
		selStart: -1,
	}
}

func (inp *InputWidget) Height() int {
	h := 1 + inp.BoxOverheadH()
	if inp.Config.Bordered {
		h += 2
	}
	return h
}

func (inp *InputWidget) Width() int { return 0 }

func (inp *InputWidget) Focusable() bool   { return true }
func (inp *InputWidget) SetFocused(f bool) { inp.focused = f }
func (inp *InputWidget) IsFocused() bool   { return inp.focused }

func (inp *InputWidget) CursorPosition() (int, int, bool) {
	if !inp.focused {
		return 0, 0, false
	}
	r := inp.GetRect()
	textX := r.X + inp.Box.MarginLeft + inp.Box.PaddingLeft
	textY := r.Y + inp.Box.MarginTop + inp.Box.PaddingTop
	if inp.Config.Bordered {
		textX += 2
		textY += 1
	} else {
		textX += textwidth.String(inp.Config.Prefix)
	}
	return textX + inp.visualOffset([]rune(inp.text), inp.cursorPos), textY, true
}

// visualOffset returns how many columns the rune at index pos sits to the right
// of the first visible rune. Fullwidth runes count as two.
func (inp *InputWidget) visualOffset(runes []rune, pos int) int {
	lo := max(min(inp.scrollOffset, len(runes)), 0)
	hi := max(min(pos, len(runes)), lo)
	return textwidth.Runes(runes[lo:hi])
}

func (inp *InputWidget) Text() string { return inp.text }
func (inp *InputWidget) SetText(t string) {
	inp.text = t
	inp.cursorPos = len([]rune(t))
	inp.clearSel()
	inp.notify()
}

func (inp *InputWidget) ResetScroll() {
	inp.scrollOffset = 0
}

func (inp *InputWidget) PasteText(text string) {
	if inp.hasSelection() {
		inp.deleteSelection()
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimRight(text, "\n")
	text = strings.ReplaceAll(text, "\n", " ")
	if text == "" {
		return
	}
	runes := []rune(inp.text)
	pasted := []rune(text)
	runes = append(runes[:inp.cursorPos], append(pasted, runes[inp.cursorPos:]...)...)
	inp.text = string(runes)
	inp.cursorPos += len(pasted)
	inp.notify()
}

func (inp *InputWidget) Clear() {
	inp.text = ""
	inp.cursorPos = 0
	inp.scrollOffset = 0
	inp.clearSel()
	inp.notify()
}

func (inp *InputWidget) Render(surface Surface) {
	if inp.Config.Bordered {
		inp.renderBordered(surface)
	} else {
		inp.renderBorderless(surface)
	}
}

func (inp *InputWidget) renderBordered(surface Surface) {
	inner := inp.RenderBox(surface)
	w, h := inner.Size()
	if w < 3 || h < 3 {
		return
	}

	borderStyle := term.StyleBorder
	if inp.focused {
		borderStyle = term.StyleBorderActive
	}
	bs := widgetBorders(inp.Box)

	inner.DrawBorder(0, 0, w, h, bs, borderStyle)

	inp.renderText(inner, 2, 1, w-4)
}

func (inp *InputWidget) renderBorderless(surface Surface) {
	inner := inp.RenderBox(surface)
	w, _ := inner.Size()
	if w <= 0 {
		return
	}

	prefixRunes := []rune(inp.Config.Prefix)
	prefixW := textwidth.Runes(prefixRunes)

	prefixStyle := term.StyleBorder
	if inp.focused {
		prefixStyle = term.StyleBorderActive
	}

	px := 0
	for _, ch := range prefixRunes {
		if px < w {
			inner.SetCell(px, 0, term.Cell{Ch: ch, Style: prefixStyle})
		}
		px += textwidth.Rune(ch)
	}

	inp.renderText(inner, prefixW, 0, w-prefixW)
}

func (inp *InputWidget) renderText(surface Surface, x, y, textW int) {
	if textW <= 0 {
		return
	}

	style := inp.Config.Style
	if style == 0 {
		style = term.StyleInput
	}

	textRunes := []rune(inp.text)

	// Scrolling is measured in columns, not runes: a window of textW columns
	// holds fewer fullwidth runes than narrow ones.
	if maxOffset := lastOffsetFitting(textRunes, textW); inp.scrollOffset > maxOffset {
		inp.scrollOffset = maxOffset
	}
	if inp.cursorPos < inp.scrollOffset {
		inp.scrollOffset = inp.cursorPos
	}
	// Walk the runes once to find the new offset rather than re-measuring the
	// whole visible range per step — pasted text can be long.
	if over := inp.visualOffset(textRunes, inp.cursorPos) - textW + 1; over > 0 {
		for i := inp.scrollOffset; i < inp.cursorPos && over > 0; i++ {
			over -= textwidth.Rune(textRunes[i])
			inp.scrollOffset = i + 1
		}
	}

	if len(textRunes) == 0 && inp.Config.Placeholder != "" {
		inp.drawRunes(surface, x, y, textW, []rune(inp.Config.Placeholder), 0, func(int) term.Style {
			return term.StyleInputPlaceholder
		})
		return
	}

	selLo, selHi := -1, -1
	if inp.hasSelection() {
		selLo, selHi = inp.selRange()
	}

	inp.drawRunes(surface, x, y, textW, textRunes, inp.scrollOffset, func(ri int) term.Style {
		if selLo >= 0 && ri >= selLo && ri < selHi {
			return term.StyleSelection
		}
		return style
	})
}

// drawRunes fills textW columns starting at x with runes from index `from`,
// advancing by display width and padding the remainder with spaces so the input
// keeps a solid background. A fullwidth rune that would not fit in the
// remaining columns is dropped rather than drawn half-outside the field.
func (inp *InputWidget) drawRunes(surface Surface, x, y, textW int, runes []rune, from int, styleAt func(ri int) term.Style) {
	col := 0
	ri := from
	for col < textW {
		ch := ' '
		s := styleAt(ri)
		w := 1
		if ri < len(runes) {
			ch = runes[ri]
			w = textwidth.Rune(ch)
			if col+w > textW {
				ch, w = ' ', 1
				ri = len(runes)
			}
		}
		surface.SetCell(x+col, y, term.Cell{Ch: ch, Style: s})
		if w > 1 {
			// The terminal covers this column with the rune itself; the cell
			// only needs to carry the matching background.
			surface.SetCell(x+col+1, y, term.Cell{Ch: ' ', Style: s})
		}
		col += w
		ri++
	}
}

// posAtColumn maps a column offset from the start of the visible text to a rune
// index. A click on the second column of a fullwidth rune selects that rune.
func (inp *InputWidget) posAtColumn(runes []rune, col int) int {
	if col <= 0 {
		return max(min(inp.scrollOffset, len(runes)), 0)
	}
	used := 0
	for i := inp.scrollOffset; i < len(runes); i++ {
		if i < 0 {
			continue
		}
		w := textwidth.Rune(runes[i])
		if col < used+w {
			return i
		}
		used += w
	}
	return len(runes)
}

// lastOffsetFitting returns the largest starting rune index whose tail still
// fills a window of w columns, so the field never scrolls past its own text.
func lastOffsetFitting(runes []rune, w int) int {
	used := 0
	i := len(runes)
	for i > 0 {
		rw := textwidth.Rune(runes[i-1])
		if used+rw > w {
			break
		}
		used += rw
		i--
	}
	return i
}

func (inp *InputWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		return inp.handleKey(tev)
	case *tcell.EventMouse:
		return inp.handleMouse(tev)
	}
	return EventIgnored
}

func (inp *InputWidget) handleKey(ev *tcell.EventKey) EventResult {
	if !inp.focused {
		return EventIgnored
	}
	shift := ev.Modifiers()&tcell.ModShift != 0
	ctrl := ev.Modifiers()&tcell.ModCtrl != 0

	switch ev.Key() {
	case tcell.KeyRune:
		if ev.Modifiers()&^tcell.ModShift != 0 {
			return EventIgnored
		}
		if inp.hasSelection() {
			inp.deleteSelection()
		}
		runes := []rune(inp.text)
		runes = append(runes[:inp.cursorPos], append([]rune{term.KeyRune(ev)}, runes[inp.cursorPos:]...)...)
		inp.text = string(runes)
		inp.cursorPos++
		inp.notify()
		return EventConsumed
	case tcell.KeyEnter:
		if !shift && inp.Config.OnSubmit != nil {
			inp.Config.OnSubmit(inp.text)
		}
		return EventConsumed
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if inp.hasSelection() {
			inp.deleteSelection()
		} else if inp.cursorPos > 0 {
			runes := []rune(inp.text)
			newPos := inp.cursorPos - 1
			if ctrl {
				newPos = inp.wordLeft()
			}
			inp.text = string(append(runes[:newPos], runes[inp.cursorPos:]...))
			inp.cursorPos = newPos
			inp.notify()
		}
		return EventConsumed
	case tcell.KeyDelete:
		if inp.hasSelection() {
			inp.deleteSelection()
		} else {
			runes := []rune(inp.text)
			if inp.cursorPos < len(runes) {
				end := inp.cursorPos + 1
				if ctrl {
					end = inp.wordRight()
				}
				inp.text = string(append(runes[:inp.cursorPos], runes[end:]...))
				inp.notify()
			}
		}
		return EventConsumed
	case tcell.KeyLeft:
		if shift {
			inp.startSel()
		}
		if !shift && inp.hasSelection() {
			lo, _ := inp.selRange()
			inp.cursorPos = lo
			inp.clearSel()
		} else if ctrl {
			inp.cursorPos = inp.wordLeft()
		} else if inp.cursorPos > 0 {
			inp.cursorPos--
		}
		if shift {
			inp.selEnd = inp.cursorPos
		}
		return EventConsumed
	case tcell.KeyRight:
		if shift {
			inp.startSel()
		}
		if !shift && inp.hasSelection() {
			_, hi := inp.selRange()
			inp.cursorPos = hi
			inp.clearSel()
		} else if ctrl {
			inp.cursorPos = inp.wordRight()
		} else if inp.cursorPos < len([]rune(inp.text)) {
			inp.cursorPos++
		}
		if shift {
			inp.selEnd = inp.cursorPos
		}
		return EventConsumed
	case tcell.KeyHome:
		if shift {
			inp.startSel()
		}
		inp.cursorPos = 0
		if shift {
			inp.selEnd = inp.cursorPos
		} else {
			inp.clearSel()
		}
		return EventConsumed
	case tcell.KeyEnd:
		if shift {
			inp.startSel()
		}
		inp.cursorPos = len([]rune(inp.text))
		if shift {
			inp.selEnd = inp.cursorPos
		} else {
			inp.clearSel()
		}
		return EventConsumed
	case tcell.KeyCtrlV:
		inp.pasteClipboard()
		return EventConsumed
	case tcell.KeyCtrlA:
		inp.selectAll()
		return EventConsumed
	case tcell.KeyCtrlC:
		inp.copySelection()
		return EventConsumed
	case tcell.KeyCtrlX:
		inp.cutSelection()
		return EventConsumed
	}
	return EventIgnored
}

func (inp *InputWidget) handleMouse(ev *tcell.EventMouse) EventResult {
	if ev.Buttons()&tcell.Button1 == 0 {
		return EventIgnored
	}
	mx, my := ev.Position()
	r := inp.GetRect()
	if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
		return EventIgnored
	}

	textX := r.X + inp.Box.MarginLeft + inp.Box.PaddingLeft + textwidth.String(inp.Config.Prefix)
	if inp.Config.Bordered {
		textX = r.X + inp.Box.MarginLeft + inp.Box.PaddingLeft + 2
	}
	runes := []rune(inp.text)
	pos := inp.posAtColumn(runes, mx-textX)

	now := time.Now().UnixMilli()
	if now-inp.lastClickTime < 400 && pos == inp.lastClickPos {
		inp.clickCount++
	} else {
		inp.clickCount = 1
	}
	inp.lastClickTime = now
	inp.lastClickPos = pos

	if inp.clickCount == 2 {
		inp.selectWordAt(pos)
	} else {
		inp.cursorPos = pos
		inp.clearSel()
	}
	return EventConsumed
}

// selection helpers

func (inp *InputWidget) hasSelection() bool {
	return inp.selStart >= 0 && inp.selStart != inp.selEnd
}

func (inp *InputWidget) selRange() (int, int) {
	if inp.selStart < inp.selEnd {
		return inp.selStart, inp.selEnd
	}
	return inp.selEnd, inp.selStart
}

func (inp *InputWidget) clearSel() {
	inp.selStart = -1
	inp.selEnd = -1
}

func (inp *InputWidget) startSel() {
	if inp.selStart < 0 {
		inp.selStart = inp.cursorPos
		inp.selEnd = inp.cursorPos
	}
}

func (inp *InputWidget) deleteSelection() {
	lo, hi := inp.selRange()
	runes := []rune(inp.text)
	if lo > len(runes) {
		lo = len(runes)
	}
	if hi > len(runes) {
		hi = len(runes)
	}
	inp.text = string(append(runes[:lo], runes[hi:]...))
	inp.cursorPos = lo
	inp.clearSel()
	inp.notify()
}

func (inp *InputWidget) selectWordAt(pos int) {
	runes := []rune(inp.text)
	if pos < 0 || pos >= len(runes) {
		return
	}
	lo, hi := pos, pos
	if isInputWordRune(runes[pos]) {
		for lo > 0 && isInputWordRune(runes[lo-1]) {
			lo--
		}
		for hi < len(runes) && isInputWordRune(runes[hi]) {
			hi++
		}
	} else if !unicode.IsSpace(runes[pos]) {
		for lo > 0 && !isInputWordRune(runes[lo-1]) && !unicode.IsSpace(runes[lo-1]) {
			lo--
		}
		for hi < len(runes) && !isInputWordRune(runes[hi]) && !unicode.IsSpace(runes[hi]) {
			hi++
		}
	} else {
		for lo > 0 && unicode.IsSpace(runes[lo-1]) {
			lo--
		}
		for hi < len(runes) && unicode.IsSpace(runes[hi]) {
			hi++
		}
	}
	inp.selStart = lo
	inp.selEnd = hi
	inp.cursorPos = hi
}

func (inp *InputWidget) selectAll() {
	runes := []rune(inp.text)
	if len(runes) == 0 {
		return
	}
	inp.selStart = 0
	inp.selEnd = len(runes)
	inp.cursorPos = len(runes)
}

func (inp *InputWidget) copySelection() {
	if !inp.hasSelection() {
		return
	}
	lo, hi := inp.selRange()
	runes := []rune(inp.text)
	clipboard.Set(string(runes[lo:hi]))
}

func (inp *InputWidget) cutSelection() {
	if !inp.hasSelection() {
		return
	}
	inp.copySelection()
	inp.deleteSelection()
}

func (inp *InputWidget) pasteClipboard() {
	inp.PasteText(clipboard.Get())
}

// word navigation

func (inp *InputWidget) wordLeft() int {
	runes := []rune(inp.text)
	pos := inp.cursorPos - 1
	if pos >= len(runes) {
		pos = len(runes) - 1
	}
	if pos < 0 {
		return 0
	}
	if unicode.IsSpace(runes[pos]) {
		for pos > 0 && unicode.IsSpace(runes[pos-1]) {
			pos--
		}
	} else if isInputWordRune(runes[pos]) {
		for pos > 0 && isInputWordRune(runes[pos-1]) {
			pos--
		}
	} else {
		for pos > 0 && !isInputWordRune(runes[pos-1]) && !unicode.IsSpace(runes[pos-1]) {
			pos--
		}
	}
	return pos
}

func (inp *InputWidget) wordRight() int {
	runes := []rune(inp.text)
	pos := inp.cursorPos
	if pos >= len(runes) {
		return len(runes)
	}
	if unicode.IsSpace(runes[pos]) {
		for pos < len(runes) && unicode.IsSpace(runes[pos]) {
			pos++
		}
	} else if isInputWordRune(runes[pos]) {
		for pos < len(runes) && isInputWordRune(runes[pos]) {
			pos++
		}
	} else {
		for pos < len(runes) && !isInputWordRune(runes[pos]) && !unicode.IsSpace(runes[pos]) {
			pos++
		}
	}
	return pos
}

func isInputWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (inp *InputWidget) notify() {
	if inp.Config.OnChange != nil {
		inp.Config.OnChange(inp.text)
	}
}
