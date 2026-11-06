package ui

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

type inputSnapshot struct {
	text string
	pos  int
}

type inputGroupType int

const (
	inputGroupNone inputGroupType = iota
	inputGroupInsert
	inputGroupDelete
)

const maxInputUndo = 100

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

	renderLeft, renderWidth, textWidth int
	drag                               widgets.InputDrag

	undoStack []inputSnapshot
	redoStack []inputSnapshot
	groupType inputGroupType
	lastRune  rune
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

func (inp *InputWidget) pushUndo(snap inputSnapshot) {
	if len(inp.undoStack) > 0 {
		top := inp.undoStack[len(inp.undoStack)-1]
		if top.text == snap.text && top.pos == snap.pos {
			return
		}
	}
	inp.undoStack = append(inp.undoStack, snap)
	if len(inp.undoStack) > maxInputUndo {
		inp.undoStack = inp.undoStack[len(inp.undoStack)-maxInputUndo:]
	}
	inp.redoStack = nil
}

func (inp *InputWidget) breakGroup() {
	inp.groupType = inputGroupNone
	inp.lastRune = 0
}

func (inp *InputWidget) recordEdit() {
	inp.pushUndo(inputSnapshot{text: inp.Text, pos: inp.CursorPos})
	inp.breakGroup()
}

func (inp *InputWidget) recordRuneInsert(r rune) {
	if (r == ' ' || r == '\t') && inp.lastRune != ' ' && inp.lastRune != '\t' {
		inp.breakGroup()
	}
	if inp.groupType != inputGroupInsert {
		inp.pushUndo(inputSnapshot{text: inp.Text, pos: inp.CursorPos})
		inp.groupType = inputGroupInsert
	}
	inp.lastRune = r
}

func (inp *InputWidget) recordDelete() {
	if inp.groupType != inputGroupDelete {
		inp.pushUndo(inputSnapshot{text: inp.Text, pos: inp.CursorPos})
		inp.groupType = inputGroupDelete
		inp.lastRune = 0
	}
}

func (inp *InputWidget) Undo() bool {
	inp.breakGroup()
	if len(inp.undoStack) == 0 {
		return false
	}
	curr := inputSnapshot{text: inp.Text, pos: inp.CursorPos}
	inp.redoStack = append(inp.redoStack, curr)
	if len(inp.redoStack) > maxInputUndo {
		inp.redoStack = inp.redoStack[len(inp.redoStack)-maxInputUndo:]
	}

	snap := inp.undoStack[len(inp.undoStack)-1]
	inp.undoStack = inp.undoStack[:len(inp.undoStack)-1]

	inp.Text = snap.text
	runes := []rune(inp.Text)
	inp.CursorPos = snap.pos
	if inp.CursorPos < 0 {
		inp.CursorPos = 0
	} else if inp.CursorPos > len(runes) {
		inp.CursorPos = len(runes)
	}
	inp.clearSel()
	inp.notify()
	return true
}

func (inp *InputWidget) Redo() bool {
	inp.breakGroup()
	if len(inp.redoStack) == 0 {
		return false
	}
	curr := inputSnapshot{text: inp.Text, pos: inp.CursorPos}
	inp.undoStack = append(inp.undoStack, curr)
	if len(inp.undoStack) > maxInputUndo {
		inp.undoStack = inp.undoStack[len(inp.undoStack)-maxInputUndo:]
	}

	snap := inp.redoStack[len(inp.redoStack)-1]
	inp.redoStack = inp.redoStack[:len(inp.redoStack)-1]

	inp.Text = snap.text
	runes := []rune(inp.Text)
	inp.CursorPos = snap.pos
	if inp.CursorPos < 0 {
		inp.CursorPos = 0
	} else if inp.CursorPos > len(runes) {
		inp.CursorPos = len(runes)
	}
	inp.clearSel()
	inp.notify()
	return true
}

func (inp *InputWidget) Render(surface Surface, x, y, w int) {
	actionsW := inp.actionsWidth()
	prefixW := textwidth.String(inp.Prefix)
	textW := w - actionsW - prefixW

	ox, oy := surface.Origin()
	inp.renderX = ox + x + prefixW
	inp.renderY = oy + y
	inp.renderLeft, inp.renderWidth, inp.textWidth = ox+x, w, max(textW, 0)
	px := x
	for _, ch := range inp.Prefix {
		surface.SetCell(px, y, term.Cell{Ch: ch, Style: inp.Style})
		px += textwidth.Rune(ch)
	}

	if textW > 0 {
		textRunes := []rune(inp.Text)
		showPlaceholder := len(textRunes) == 0 && inp.Placeholder != ""

		// The visible window is textW columns wide, which holds fewer fullwidth
		// runes than narrow ones — scroll by column, not by rune index.
		if inp.CursorPos < inp.scrollOffset {
			inp.scrollOffset = inp.CursorPos
		}
		// Walk the runes once to find the new offset rather than re-measuring
		// the whole visible range per step — pasted text can be long.
		if over := inp.visualOffset(textRunes, inp.CursorPos) - textW + 1; over > 0 {
			for i := inp.scrollOffset; i < inp.CursorPos && over > 0; i++ {
				over -= textwidth.Rune(textRunes[i])
				inp.scrollOffset = i + 1
			}
		}

		selLo, selHi := -1, -1
		if inp.HasSelection() {
			selLo, selHi = inp.selRange()
		}

		if showPlaceholder {
			drawInputRunes(surface, x+prefixW, y, textW, []rune(inp.Placeholder), 0, func(int) term.Style {
				return term.StyleInputPlaceholder
			})
		} else {
			drawInputRunes(surface, x+prefixW, y, textW, textRunes, inp.scrollOffset, func(ri int) term.Style {
				if selLo >= 0 && ri >= selLo && ri < selHi {
					return term.StyleSelection
				}
				return inp.Style
			})
		}
	}

	ax := x + prefixW + textW
	inp.ActionHits = inp.ActionHits[:0]
	for _, action := range inp.Actions {
		style := term.StyleInputAction
		if action.Active {
			style = term.StyleDefault
		}
		labelW := textwidth.String(action.Label)
		inp.ActionHits = append(inp.ActionHits, HitRegion{X: ox + ax, Y: inp.renderY, W: labelW})
		for _, ch := range action.Label {
			cw := textwidth.Rune(ch)
			if ax+cw <= x+w {
				surface.SetCell(ax, y, term.Cell{Ch: ch, Style: style})
				ax += cw
			}
		}
		if ax < x+w {
			surface.SetCell(ax, y, term.Cell{Ch: ' ', Style: inp.Style})
			ax++
		}
	}
}

// drawInputRunes fills textW columns starting at x with runes from index
// `from`, advancing by display width and padding the rest with spaces so the
// field keeps a solid background. A fullwidth rune that does not fit in the
// remaining columns is dropped rather than drawn half outside the field.
func drawInputRunes(surface Surface, x, y, textW int, runes []rune, from int, styleAt func(ri int) term.Style) {
	col := 0
	ri := from
	for col < textW {
		ch := ' '
		style := styleAt(ri)
		w := 1
		if ri >= 0 && ri < len(runes) {
			ch = runes[ri]
			w = textwidth.Rune(ch)
			if col+w > textW {
				ch, w = ' ', 1
				ri = len(runes)
			}
		}
		surface.SetCell(x+col, y, term.Cell{Ch: ch, Style: style})
		if w > 1 {
			// The terminal covers this column with the rune itself; the cell
			// only needs to carry the matching background.
			surface.SetCell(x+col+1, y, term.Cell{Ch: ' ', Style: style})
		}
		col += w
		ri++
	}
}

// visualOffset returns how many columns the rune at index pos sits to the right
// of the first visible rune. Fullwidth runes count as two.
func (inp *InputWidget) visualOffset(runes []rune, pos int) int {
	lo := max(min(inp.scrollOffset, len(runes)), 0)
	hi := max(min(pos, len(runes)), lo)
	return textwidth.Runes(runes[lo:hi])
}

// posAtColumn maps a column offset from the start of the visible text to a rune
// index. A click on the second column of a fullwidth rune selects that rune.
func (inp *InputWidget) posAtColumn(runes []rune, col int) int {
	if col <= 0 {
		return max(min(inp.scrollOffset, len(runes)), 0)
	}
	used := 0
	for i := max(inp.scrollOffset, 0); i < len(runes); i++ {
		w := textwidth.Rune(runes[i])
		if col < used+w {
			return i
		}
		used += w
	}
	return len(runes)
}

func (inp *InputWidget) actionsWidth() int {
	if len(inp.Actions) == 0 {
		return 0
	}
	w := 0
	for _, a := range inp.Actions {
		w += textwidth.String(a.Label) + 1
	}
	return w
}

func (inp *InputWidget) CursorX(x int) int {
	return x + textwidth.String(inp.Prefix) + inp.visualOffset([]rune(inp.Text), inp.CursorPos)
}

func (inp *InputWidget) ResetScroll() {
	inp.scrollOffset = 0
}

func (inp *InputWidget) HandleEvent(ev tcell.Event) EventResult {
	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventIgnored
	}
	inp.drag.Cancel()
	shift := kev.Modifiers()&tcell.ModShift != 0

	switch kev.Key() {
	case tcell.KeyRune:
		if kev.Modifiers()&^tcell.ModShift != 0 {
			return EventIgnored
		}
		if inp.HasSelection() {
			inp.recordEdit()
			inp.deleteSelection()
			inp.groupType = inputGroupInsert
			inp.lastRune = term.KeyRune(kev)
		} else {
			inp.recordRuneInsert(term.KeyRune(kev))
		}
		runes := []rune(inp.Text)
		runes = append(runes[:inp.CursorPos], append([]rune{term.KeyRune(kev)}, runes[inp.CursorPos:]...)...)
		inp.Text = string(runes)
		inp.CursorPos++
		inp.notify()
		return EventConsumed
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if inp.HasSelection() {
			inp.recordEdit()
			inp.deleteSelection()
		} else if inp.CursorPos > 0 {
			inp.recordDelete()
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
			inp.recordEdit()
			inp.deleteSelection()
		} else {
			runes := []rune(inp.Text)
			if inp.CursorPos < len(runes) {
				inp.recordDelete()
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
		inp.breakGroup()
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
		inp.breakGroup()
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
		inp.breakGroup()
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
		inp.breakGroup()
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
	case tcell.KeyCtrlZ:
		if shift {
			inp.Redo()
		} else {
			inp.Undo()
		}
		return EventConsumed
	case tcell.KeyCtrlY:
		inp.Redo()
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
	inp.breakGroup()
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
	inp.recordEdit()
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

func (inp *InputWidget) OwnsPointerCapture() bool   { return inp.drag.Active() }
func (inp *InputWidget) CancelPointerCapture() bool { return inp.drag.Cancel() }

func (inp *InputWidget) HandleMouse(ev *tcell.EventMouse) EventResult {
	x, y := ev.Position()
	if inp.drag.Active() {
		runes := []rune(inp.Text)
		position := inp.posAtColumn(runes, x-inp.renderX)
		if y < inp.renderY {
			position = 0
		} else if y > inp.renderY {
			position = len(runes)
		} else if x < inp.renderX {
			position = max(inp.scrollOffset-1, 0)
		} else if x >= inp.renderX+inp.textWidth {
			position = min(position+1, len(runes))
		}
		if anchor, end, moved := inp.drag.Extend(x, y, position); moved {
			inp.selStart, inp.selEnd, inp.CursorPos = anchor, end, end
			inp.lastClickTime, inp.clickCount = 0, 0
		}
		if ev.Buttons()&tcell.Button1 == 0 {
			inp.drag.Cancel()
			return EventConsumed
		}
		return EventCaptured
	}
	if ev.Buttons()&tcell.Button1 == 0 || y != inp.renderY || x < inp.renderLeft || x >= inp.renderLeft+inp.renderWidth {
		return EventIgnored
	}
	for _, hit := range inp.ActionHits {
		if hit.Contains(x, y) {
			inp.HandleClick(x, y)
			return EventConsumed
		}
	}
	inp.HandleClick(x, y)
	inp.drag.Begin(x, y, inp.CursorPos)
	return EventCaptured
}

// HandleClick handles a mouse click at screen-absolute coordinates.
// It checks action buttons first, then positions the cursor or selects
// a word on double-click.
func (inp *InputWidget) HandleClick(screenX, screenY int) bool {
	inp.breakGroup()
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

	runes := []rune(inp.Text)
	pos := inp.posAtColumn(runes, screenX-inp.renderX)

	now := time.Now().UnixMilli()
	if now-inp.lastClickTime < DoubleClickMs && pos == inp.lastClickPos {
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
	inp.PasteText(clipboard.Get())
}

func (inp *InputWidget) PasteText(text string) {
	text = sanitizePaste(text)
	if text == "" {
		return
	}
	inp.recordEdit()
	if inp.HasSelection() {
		inp.deleteSelection()
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

func (inp *InputWidget) CopyTextState(source *InputWidget) {
	inp.drag.Cancel()
	inp.Text, inp.CursorPos = source.Text, source.CursorPos
	inp.scrollOffset = source.scrollOffset
	inp.selStart, inp.selEnd = source.selStart, source.selEnd
	inp.undoStack = slices.Clone(source.undoStack)
	inp.redoStack = slices.Clone(source.redoStack)
	inp.groupType = source.groupType
	inp.lastRune = source.lastRune
}

func (inp *InputWidget) SetText(text string) {
	inp.drag.Cancel()
	inp.Text = text
	inp.CursorPos = len([]rune(text))
	inp.clearSel()
	inp.undoStack = nil
	inp.redoStack = nil
	inp.breakGroup()
	inp.notify()
}

func (inp *InputWidget) Clear() {
	inp.drag.Cancel()
	if inp.Text != "" {
		inp.recordEdit()
	}
	inp.Text = ""
	inp.CursorPos = 0
	inp.clearSel()
	inp.notify()
}

func (inp *InputWidget) notify() {
	inp.CancelPointerCapture()
	if inp.OnChange != nil {
		inp.OnChange(inp.Text)
	}
}
