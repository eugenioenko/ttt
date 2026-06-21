package ui

import (
	"strings"
	"time"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

type InputAction struct {
	Label   string
	Active  bool
	OnClick func()
}

type InputWidget struct {
	Text          string
	Prefix        string
	Placeholder   string
	CursorPos     int
	scrollOffset  int
	selStart      int // -1 means no selection
	selEnd        int
	lastClickTime int64
	lastClickPos  int
	clickCount    int
	renderX       int // screen-absolute X of the text area start
	renderY       int // screen-absolute Y of the input row
	Style         term.Style
	Actions       []InputAction
	ActionHits    []HitRegion
	OnChange      func(text string)
}

func NewInputWidget() *InputWidget {
	return &InputWidget{
		Prefix:   " ❯ ",
		Style:    term.StyleInput,
		selStart: -1,
	}
}

func (inp *InputWidget) HasSelection() bool {
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

func (inp *InputWidget) deleteSelection() {
	lo, hi := inp.selRange()
	runes := []rune(inp.Text)
	if lo > len(runes) {
		lo = len(runes)
	}
	if hi > len(runes) {
		hi = len(runes)
	}
	if lo > hi {
		lo = hi
	}
	inp.Text = string(append(runes[:lo], runes[hi:]...))
	inp.CursorPos = lo
	inp.clearSel()
	inp.notify()
}

func (inp *InputWidget) Render(surface *RenderSurface, x, y, w int) {
	actionsW := inp.actionsWidth()
	prefixRunes := []rune(inp.Prefix)
	prefixW := len(prefixRunes)
	textW := w - actionsW - prefixW

	ox, oy := surface.Origin()
	inp.renderX = ox + x + prefixW
	inp.renderY = oy + y
	for i, ch := range prefixRunes {
		surface.SetCell(x+i, y, term.Cell{Ch: ch, Style: inp.Style})
	}

	if textW > 0 {
		textRunes := []rune(inp.Text)
		showPlaceholder := len(textRunes) == 0 && inp.Placeholder != ""

		if inp.CursorPos < inp.scrollOffset {
			inp.scrollOffset = inp.CursorPos
		}
		if inp.CursorPos >= inp.scrollOffset+textW {
			inp.scrollOffset = inp.CursorPos - textW + 1
		}

		selLo, selHi := -1, -1
		if inp.HasSelection() {
			selLo, selHi = inp.selRange()
		}

		if showPlaceholder {
			phRunes := []rune(inp.Placeholder)
			for i := 0; i < textW; i++ {
				ch := ' '
				if i < len(phRunes) {
					ch = phRunes[i]
				}
				surface.SetCell(x+prefixW+i, y, term.Cell{Ch: ch, Style: term.StyleInputPlaceholder})
			}
		} else {
			for i := 0; i < textW; i++ {
				ch := ' '
				ri := inp.scrollOffset + i
				if ri < len(textRunes) {
					ch = textRunes[ri]
				}
				style := inp.Style
				if selLo >= 0 && ri >= selLo && ri < selHi {
					style = term.StyleSelection
				}
				surface.SetCell(x+prefixW+i, y, term.Cell{Ch: ch, Style: style})
			}
		}
	}

	ax := x + prefixW + textW
	inp.ActionHits = inp.ActionHits[:0]
	for _, action := range inp.Actions {
		style := term.StyleInputAction
		if action.Active {
			style = term.StyleDefault
		}
		labelW := len([]rune(action.Label))
		inp.ActionHits = append(inp.ActionHits, HitRegion{X: ox + ax, Y: inp.renderY, W: labelW})
		for _, ch := range action.Label {
			if ax < x+w {
				surface.SetCell(ax, y, term.Cell{Ch: ch, Style: style})
				ax++
			}
		}
		if ax < x+w {
			surface.SetCell(ax, y, term.Cell{Ch: ' ', Style: inp.Style})
			ax++
		}
	}
}

func (inp *InputWidget) actionsWidth() int {
	if len(inp.Actions) == 0 {
		return 0
	}
	w := 0
	for _, a := range inp.Actions {
		w += len([]rune(a.Label)) + 1
	}
	return w
}

func (inp *InputWidget) CursorX(x int) int {
	return x + len([]rune(inp.Prefix)) + inp.CursorPos - inp.scrollOffset
}

func (inp *InputWidget) ResetScroll() {
	inp.scrollOffset = 0
}

func (inp *InputWidget) HandleEvent(ev tcell.Event) EventResult {
	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventIgnored
	}
	shift := kev.Modifiers()&tcell.ModShift != 0

	switch kev.Key() {
	case tcell.KeyRune:
		if inp.HasSelection() {
			inp.deleteSelection()
		}
		runes := []rune(inp.Text)
		runes = append(runes[:inp.CursorPos], append([]rune{kev.Rune()}, runes[inp.CursorPos:]...)...)
		inp.Text = string(runes)
		inp.CursorPos++
		inp.notify()
		return EventConsumed
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if inp.HasSelection() {
			inp.deleteSelection()
		} else if inp.CursorPos > 0 {
			runes := []rune(inp.Text)
			newPos := inp.CursorPos - 1
			if kev.Modifiers()&tcell.ModCtrl != 0 {
				newPos = inp.wordLeft()
			}
			inp.Text = string(append(runes[:newPos], runes[inp.CursorPos:]...))
			inp.CursorPos = newPos
			inp.notify()
		}
		return EventConsumed
	case tcell.KeyDelete:
		if inp.HasSelection() {
			inp.deleteSelection()
		} else {
			runes := []rune(inp.Text)
			if inp.CursorPos < len(runes) {
				end := inp.CursorPos + 1
				if kev.Modifiers()&tcell.ModCtrl != 0 {
					end = inp.wordRight()
				}
				inp.Text = string(append(runes[:inp.CursorPos], runes[end:]...))
				inp.notify()
			}
		}
		return EventConsumed
	case tcell.KeyLeft:
		ctrl := kev.Modifiers()&tcell.ModCtrl != 0
		if shift {
			inp.startSel()
		}
		if ctrl {
			inp.CursorPos = inp.wordLeft()
		} else if inp.CursorPos > 0 {
			inp.CursorPos--
		}
		if shift {
			inp.selEnd = inp.CursorPos
		} else {
			inp.clearSel()
		}
		return EventConsumed
	case tcell.KeyRight:
		ctrl := kev.Modifiers()&tcell.ModCtrl != 0
		if shift {
			inp.startSel()
		}
		if ctrl {
			inp.CursorPos = inp.wordRight()
		} else if inp.CursorPos < len([]rune(inp.Text)) {
			inp.CursorPos++
		}
		if shift {
			inp.selEnd = inp.CursorPos
		} else {
			inp.clearSel()
		}
		return EventConsumed
	case tcell.KeyHome:
		if shift {
			inp.startSel()
		}
		inp.CursorPos = 0
		if shift {
			inp.selEnd = inp.CursorPos
		} else {
			inp.clearSel()
		}
		return EventConsumed
	case tcell.KeyEnd:
		if shift {
			inp.startSel()
		}
		inp.CursorPos = len([]rune(inp.Text))
		if shift {
			inp.selEnd = inp.CursorPos
		} else {
			inp.clearSel()
		}
		return EventConsumed
	case tcell.KeyCtrlV:
		inp.PasteClipboard()
		return EventConsumed
	case tcell.KeyCtrlA:
		inp.SelectAll()
		return EventConsumed
	case tcell.KeyCtrlC:
		inp.CopySelection()
		return EventConsumed
	case tcell.KeyCtrlX:
		inp.CutSelection()
		return EventConsumed
	}
	return EventIgnored
}

func (inp *InputWidget) startSel() {
	if inp.selStart < 0 {
		inp.selStart = inp.CursorPos
		inp.selEnd = inp.CursorPos
	}
}

func (inp *InputWidget) SelectAll() {
	runes := []rune(inp.Text)
	if len(runes) == 0 {
		return
	}
	inp.selStart = 0
	inp.selEnd = len(runes)
	inp.CursorPos = len(runes)
}

func (inp *InputWidget) CopySelection() {
	if !inp.HasSelection() {
		return
	}
	lo, hi := inp.selRange()
	runes := []rune(inp.Text)
	clipboard.Set(string(runes[lo:hi]))
}

func (inp *InputWidget) CutSelection() {
	if !inp.HasSelection() {
		return
	}
	inp.CopySelection()
	inp.deleteSelection()
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (inp *InputWidget) wordLeft() int {
	runes := []rune(inp.Text)
	pos := inp.CursorPos - 1
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
	} else if isWordRune(runes[pos]) {
		for pos > 0 && isWordRune(runes[pos-1]) {
			pos--
		}
	} else {
		for pos > 0 && !isWordRune(runes[pos-1]) && !unicode.IsSpace(runes[pos-1]) {
			pos--
		}
	}
	return pos
}

func (inp *InputWidget) wordRight() int {
	runes := []rune(inp.Text)
	pos := inp.CursorPos
	if pos >= len(runes) {
		return len(runes)
	}
	if unicode.IsSpace(runes[pos]) {
		for pos < len(runes) && unicode.IsSpace(runes[pos]) {
			pos++
		}
	} else if isWordRune(runes[pos]) {
		for pos < len(runes) && isWordRune(runes[pos]) {
			pos++
		}
	} else {
		for pos < len(runes) && !isWordRune(runes[pos]) && !unicode.IsSpace(runes[pos]) {
			pos++
		}
	}
	return pos
}

func (inp *InputWidget) selectWordAt(pos int) {
	runes := []rune(inp.Text)
	if pos < 0 || pos >= len(runes) {
		return
	}
	lo, hi := pos, pos
	if isWordRune(runes[pos]) {
		for lo > 0 && isWordRune(runes[lo-1]) {
			lo--
		}
		for hi < len(runes) && isWordRune(runes[hi]) {
			hi++
		}
	} else if !unicode.IsSpace(runes[pos]) {
		for lo > 0 && !isWordRune(runes[lo-1]) && !unicode.IsSpace(runes[lo-1]) {
			lo--
		}
		for hi < len(runes) && !isWordRune(runes[hi]) && !unicode.IsSpace(runes[hi]) {
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
	inp.CursorPos = hi
}

// HandleClick handles a mouse click at screen-absolute coordinates.
// It checks action buttons first, then positions the cursor or selects
// a word on double-click.
func (inp *InputWidget) HandleClick(screenX, screenY int) bool {
	for i, hit := range inp.ActionHits {
		if screenX >= hit.X && screenX < hit.X+hit.W && screenY == hit.Y {
			if i < len(inp.Actions) && inp.Actions[i].OnClick != nil {
				inp.Actions[i].OnClick()
			}
			return true
		}
	}

	if screenY != inp.renderY {
		return false
	}

	pos := inp.scrollOffset + (screenX - inp.renderX)
	runes := []rune(inp.Text)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}

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
		inp.CursorPos = pos
		inp.clearSel()
	}
	return true
}

// InputHolder is implemented by widgets that host an InputWidget so global
// clipboard commands can be routed to the focused input. FocusedInput may
// return nil when no input is currently focused.
type InputHolder interface {
	FocusedInput() *InputWidget
}

// PasteClipboard inserts clipboard text at the cursor, replacing any
// selection. Newlines are flattened for single-line inputs.
func (inp *InputWidget) PasteClipboard() {
	if inp.HasSelection() {
		inp.deleteSelection()
	}
	text := sanitizePaste(clipboard.Get())
	if text == "" {
		return
	}
	runes := []rune(inp.Text)
	pasted := []rune(text)
	runes = append(runes[:inp.CursorPos], append(pasted, runes[inp.CursorPos:]...)...)
	inp.Text = string(runes)
	inp.CursorPos += len(pasted)
	inp.notify()
}

func sanitizePaste(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimRight(text, "\n")
	return strings.ReplaceAll(text, "\n", " ")
}

func (inp *InputWidget) SetText(text string) {
	inp.Text = text
	inp.CursorPos = len([]rune(text))
	inp.clearSel()
	inp.notify()
}

func (inp *InputWidget) Clear() {
	inp.Text = ""
	inp.CursorPos = 0
	inp.clearSel()
	inp.notify()
}

func (inp *InputWidget) notify() {
	if inp.OnChange != nil {
		inp.OnChange(inp.Text)
	}
}
