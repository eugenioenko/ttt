package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v2"
)

type KeybindingsWidget struct {
	BaseWidget
	Borders     *term.BorderSet
	OnDismiss   func()
	GetShortcut func(cmdID string) string
	GetDefault  func(cmdID string) string
	OnEdit      func(cmdID string, newKey string)
	OnReset     func(cmdID string)
	OnClear     func(cmdID string)
	OnHelp      func()

	input        *InputWidget
	allItems     []keybindingEntry
	items        []keybindingEntry
	selected     int
	scrollOffset int

	recording     bool
	recordCombo   string
	recordChord   string
	focusedAction int // -1 = input/list, 0..4 = footer buttons

	boxX, boxY, boxW, boxH int
	inputX, inputY         int
	visibleItems           int
	showScroll             bool
	scrollbar              Scrollbar
	btnEdit                HitRegion
	btnReset               HitRegion
	btnClear               HitRegion
	btnHelp                HitRegion
	btnClose               HitRegion
}

type keybindingEntry struct {
	CmdID    string
	Title    string
	Keywords []string
}

func NewKeybindingsWidget(commands []command.Command) *KeybindingsWidget {
	w := &KeybindingsWidget{focusedAction: -1}
	w.allItems = make([]keybindingEntry, len(commands))
	for i, cmd := range commands {
		w.allItems[i] = keybindingEntry{
			CmdID:    cmd.ID,
			Title:    cmd.Title,
			Keywords: cmd.Keywords,
		}
	}
	sort.Slice(w.allItems, func(i, j int) bool {
		return w.allItems[i].Title < w.allItems[j].Title
	})
	w.input = NewInputWidget()
	w.input.Placeholder = "Search shortcuts or commands..."
	w.input.OnChange = func(text string) {
		w.filter()
	}
	w.filter()
	return w
}

func (w *KeybindingsWidget) Focusable() bool { return true }

func (w *KeybindingsWidget) CursorPosition() (int, int, bool) {
	if w.recording || w.focusedAction >= 0 {
		return 0, 0, false
	}
	return w.input.CursorX(w.inputX), w.inputY, true
}

func (w *KeybindingsWidget) Render(surface *RenderSurface) {
	sw, sh := surface.Size()

	boxW := sw * 7 / 10
	if boxW > 70 {
		boxW = 70
	}
	if boxW < 40 {
		boxW = 40
	}
	if boxW > sw-4 {
		boxW = sw - 4
	}

	maxItems := 12
	boxH := 6 + len(w.items)
	if boxH > maxItems+6 {
		boxH = maxItems + 6
	}
	if boxH > sh-2 {
		boxH = sh - 2
	}
	if boxH < 7 {
		boxH = 7
	}

	boxX := (sw - boxW) / 2
	boxY := 2

	w.boxX = boxX
	w.boxY = boxY
	w.boxW = boxW
	w.boxH = boxH

	b := term.DoubleBorderSet()
	if w.Borders != nil {
		b = *w.Borders
	}

	surface.DrawBorder(boxX, boxY, boxW, boxH, b, term.StyleBorder)
	surface.ClearRect(boxX+1, boxY+1, boxW-2, boxH-2, term.StyleDefault)

	w.inputX = boxX + 1
	w.inputY = boxY + 1
	w.input.Render(surface, w.inputX, w.inputY, boxW-2)

	for x := boxX + 1; x < boxX+boxW-1; x++ {
		surface.SetCell(x, boxY+2, term.Cell{Ch: b.Horizontal, Style: term.StyleBorder})
	}

	visibleItems := boxH - 6
	w.visibleItems = visibleItems
	w.ensureVisible(visibleItems)
	showScroll := len(w.items) > visibleItems
	w.showScroll = showScroll
	contentRight := boxX + boxW - 1
	if showScroll {
		contentRight--
	}

	if showScroll {
		w.scrollbar.Height = visibleItems
		w.scrollbar.TotalItems = len(w.items)
		w.scrollbar.TopItem = w.scrollOffset
		w.scrollbar.X = boxX + boxW - 2
		w.scrollbar.Y = boxY + 3
	}

	w.btnEdit = HitRegion{}
	w.btnReset = HitRegion{}
	w.btnClear = HitRegion{}
	w.btnHelp = HitRegion{}

	for i := 0; i < visibleItems && w.scrollOffset+i < len(w.items); i++ {
		y := boxY + 3 + i
		idx := w.scrollOffset + i
		item := w.items[idx]
		isSelected := idx == w.selected

		style := term.StylePaletteItem
		if isSelected {
			style = term.StylePaletteSelected
		}

		surface.ClearRect(boxX+1, y, contentRight-boxX-1, 1, style)
		surface.DrawText(boxX+2, y, item.Title, contentRight-1, style)

		if isSelected && w.recording {
			label := "Press key..."
			if w.recordCombo != "" {
				label = w.recordCombo + " ..."
			}
			detailRunes := []rune(label)
			sx := contentRight - 1 - len(detailRunes)
			if sx > w.boxX+1 {
				surface.DrawText(sx, y, label, contentRight-1, term.StyleInput)
			}
		} else {
			w.renderShortcut(surface, y, contentRight, item, style)
		}
	}

	if showScroll {
		w.scrollbar.Render(surface, w.scrollbar.X, w.scrollbar.Y)
	}

	dividerY := boxY + boxH - 3
	for x := boxX + 1; x < boxX+boxW-1; x++ {
		surface.SetCell(x, dividerY, term.Cell{Ch: b.Horizontal, Style: term.StyleBorder})
	}

	footerY := boxY + boxH - 2
	surface.ClearRect(boxX+1, footerY, boxW-2, 1, term.StyleDefault)
	ox, oy := surface.Origin()
	if w.recording {
		hint := "Press key combination, Enter to confirm"
		surface.DrawText(boxX+2, footerY, hint, boxX+boxW-2, term.StyleMuted)
	} else {
		w.renderFooterActions(surface, footerY, ox, oy)
	}
}

func (w *KeybindingsWidget) renderShortcut(surface *RenderSurface, y, contentRight int, item keybindingEntry, style term.Style) {
	shortcut := ""
	if w.GetShortcut != nil {
		shortcut = w.GetShortcut(item.CmdID)
	}
	if shortcut == "" {
		return
	}

	modified := false
	if w.GetDefault != nil {
		def := w.GetDefault(item.CmdID)
		modified = shortcut != def
	}

	display := shortcut
	if modified {
		display = "* " + shortcut
	}

	detailRunes := []rune(display)
	sx := contentRight - 1 - len(detailRunes)
	if sx > w.boxX+1 {
		detailStyle := term.StyleMuted
		if style == term.StylePaletteSelected {
			detailStyle = style
		}
		surface.DrawText(sx, y, display, contentRight-1, detailStyle)
	}
}

func (w *KeybindingsWidget) renderFooterActions(surface *RenderSurface, y, ox, oy int) {
	type footerBtn struct {
		label string
		hit   *HitRegion
	}

	allBtns := []footerBtn{
		{"Cancel", &w.btnClose},
		{"Edit", &w.btnEdit},
		{"Reset", &w.btnReset},
		{"Clear", &w.btnClear},
		{"Help", &w.btnHelp},
	}

	// Cancel on the left
	x := w.boxX + 2
	btn := allBtns[0]
	labelRunes := []rune(btn.label)
	*btn.hit = HitRegion{X: ox + x, Y: oy + y, W: len(labelRunes)}
	style := term.StyleDefault
	if w.focusedAction == 0 {
		style = term.StylePaletteSelected
	}
	for _, ch := range labelRunes {
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
		x++
	}

	// Right-aligned actions
	rightBtns := allBtns[1:]
	totalW := 0
	for i, btn := range rightBtns {
		if i > 0 {
			totalW++
		}
		totalW += len([]rune(btn.label))
	}
	x = w.boxX + w.boxW - 2 - totalW
	for i, btn := range rightBtns {
		if i > 0 {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StyleDefault})
			x++
		}
		labelRunes := []rune(btn.label)
		*btn.hit = HitRegion{X: ox + x, Y: oy + y, W: len(labelRunes)}
		style := term.StyleDefault
		if w.focusedAction == i+1 {
			style = term.StylePaletteSelected
		}
		for _, ch := range labelRunes {
			surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
			x++
		}
	}
}

func (w *KeybindingsWidget) HandleEvent(ev tcell.Event) EventResult {
	if mev, ok := ev.(*tcell.EventMouse); ok {
		return w.handleMouse(mev)
	}

	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventConsumed
	}

	if w.recording {
		return w.handleRecordKey(kev)
	}

	const actionCount = 5 // Cancel, Edit, Reset, Clear, Help

	switch kev.Key() {
	case tcell.KeyTab:
		if w.focusedAction < actionCount-1 {
			w.focusedAction++
		} else {
			w.focusedAction = 0
		}
	case tcell.KeyBacktab:
		if w.focusedAction > 0 {
			w.focusedAction--
		} else {
			w.focusedAction = actionCount - 1
		}
	case tcell.KeyEscape:
		if w.focusedAction >= 0 {
			w.focusedAction = -1
		} else if w.OnDismiss != nil {
			w.OnDismiss()
		}
	case tcell.KeyEnter:
		if w.focusedAction >= 0 {
			w.activateAction(w.focusedAction)
		} else {
			w.startRecording()
		}
	case tcell.KeyUp:
		w.focusedAction = -1
		if w.selected > 0 {
			w.selected--
		} else if len(w.items) > 0 {
			w.selected = len(w.items) - 1
		}
	case tcell.KeyDown:
		w.focusedAction = -1
		if w.selected < len(w.items)-1 {
			w.selected++
		} else {
			w.selected = 0
		}
	case tcell.KeyDelete:
		w.clearSelected()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(w.input.Text) == 0 {
			w.resetSelected()
		} else {
			w.focusedAction = -1
			w.input.HandleEvent(ev)
		}
	default:
		w.focusedAction = -1
		w.input.HandleEvent(ev)
	}

	return EventConsumed
}

func (w *KeybindingsWidget) handleMouse(mev *tcell.EventMouse) EventResult {
	btn := mev.Buttons()
	mx, my := mev.Position()

	if w.showScroll {
		if newTop, consumed := w.scrollbar.HandleEvent(mev); consumed {
			w.scrollOffset = newTop
			w.clampSelected()
			if w.scrollbar.IsDragging() {
				return EventCaptured
			}
			return EventConsumed
		}
		if w.scrollbar.IsDragging() {
			return EventCaptured
		}
	}

	if btn&tcell.WheelUp != 0 {
		if w.selected > 0 {
			w.selected--
		}
		return EventConsumed
	}
	if btn&tcell.WheelDown != 0 {
		if w.selected < len(w.items)-1 {
			w.selected++
		}
		return EventConsumed
	}

	if btn&tcell.Button1 != 0 {
		if my == w.inputY {
			w.input.HandleClick(mx, my)
			if w.recording {
				w.recording = false
				w.recordCombo = ""
				w.recordChord = ""
			}
			return EventConsumed
		}

		if !w.recording {
			if w.btnEdit.W > 0 && w.btnEdit.Contains(mx, my) {
				w.startRecording()
				return EventConsumed
			}
			if w.btnReset.W > 0 && w.btnReset.Contains(mx, my) {
				w.resetSelected()
				return EventConsumed
			}
			if w.btnClear.W > 0 && w.btnClear.Contains(mx, my) {
				w.clearSelected()
				return EventConsumed
			}
			if w.btnHelp.W > 0 && w.btnHelp.Contains(mx, my) {
				if w.OnHelp != nil {
					w.OnHelp()
				}
				return EventConsumed
			}
			if w.btnClose.W > 0 && w.btnClose.Contains(mx, my) {
				if w.OnDismiss != nil {
					w.OnDismiss()
				}
				return EventConsumed
			}
		}

		itemsStartY := w.boxY + 3
		if w.visibleItems > 0 && my >= itemsStartY && my < itemsStartY+w.visibleItems {
			clickedIdx := w.scrollOffset + (my - itemsStartY)
			if clickedIdx >= 0 && clickedIdx < len(w.items) {
				if w.recording {
					w.recording = false
					w.recordCombo = ""
					w.recordChord = ""
				}
				w.selected = clickedIdx
			}
		}
	}

	return EventConsumed
}

func (w *KeybindingsWidget) handleRecordKey(kev *tcell.EventKey) EventResult {
	if kev.Key() == tcell.KeyEscape {
		w.recording = false
		w.recordCombo = ""
		w.recordChord = ""
		return EventConsumed
	}

	combo := describeKeyCombo(kev)

	if w.recordChord != "" {
		if kev.Key() == tcell.KeyEnter {
			w.finishRecording(w.recordChord)
		} else {
			w.finishRecording(w.recordChord + " " + combo)
		}
		return EventConsumed
	}

	w.recordCombo = combo
	w.recordChord = combo

	if kev.Key() == tcell.KeyEnter {
		w.finishRecording(combo)
	}

	return EventConsumed
}

func (w *KeybindingsWidget) activateAction(idx int) {
	switch idx {
	case 0: // Cancel
		if w.OnDismiss != nil {
			w.OnDismiss()
		}
	case 1: // Edit
		w.startRecording()
	case 2: // Reset
		w.resetSelected()
	case 3: // Clear
		w.clearSelected()
	case 4: // Help
		if w.OnHelp != nil {
			w.OnHelp()
		}
	}
}

func (w *KeybindingsWidget) startRecording() {
	if w.selected < 0 || w.selected >= len(w.items) {
		return
	}
	w.recording = true
	w.recordCombo = ""
	w.recordChord = ""
}

func (w *KeybindingsWidget) finishRecording(combo string) {
	w.recording = false
	w.recordCombo = ""
	w.recordChord = ""
	if w.selected >= 0 && w.selected < len(w.items) {
		item := w.items[w.selected]
		if w.OnEdit != nil {
			w.OnEdit(item.CmdID, combo)
		}
	}
}

func (w *KeybindingsWidget) resetSelected() {
	if w.selected >= 0 && w.selected < len(w.items) {
		item := w.items[w.selected]
		if w.OnReset != nil {
			w.OnReset(item.CmdID)
		}
	}
}

func (w *KeybindingsWidget) clearSelected() {
	if w.selected >= 0 && w.selected < len(w.items) {
		item := w.items[w.selected]
		if w.OnClear != nil {
			w.OnClear(item.CmdID)
		}
	}
}

func (w *KeybindingsWidget) filter() {
	query := strings.TrimSpace(w.input.Text)
	w.items = nil

	if query == "" {
		w.items = make([]keybindingEntry, len(w.allItems))
		copy(w.items, w.allItems)
	} else {
		type scored struct {
			item  keybindingEntry
			score int
		}
		var matches []scored
		for _, item := range w.allItems {
			bestOk, bestScore := fuzzyMatch(query, item.Title)

			shortcut := ""
			if w.GetShortcut != nil {
				shortcut = w.GetShortcut(item.CmdID)
			}
			if shortcut != "" {
				if ok, score := fuzzyMatch(query, shortcut); ok {
					if !bestOk || score > bestScore {
						bestOk = true
						bestScore = score
					}
				}
			}

			for _, kw := range item.Keywords {
				if ok, score := fuzzyMatch(query, kw); ok {
					penalized := score / 2
					if !bestOk || penalized > bestScore {
						bestOk = true
						bestScore = penalized
					}
				}
			}

			if ok, score := fuzzyMatch(query, item.CmdID); ok {
				penalized := score / 2
				if !bestOk || penalized > bestScore {
					bestOk = true
					bestScore = penalized
				}
			}

			if bestOk {
				matches = append(matches, scored{item: item, score: bestScore})
			}
		}
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].score > matches[j].score
		})
		for _, m := range matches {
			w.items = append(w.items, m.item)
		}
	}

	w.selected = 0
	w.scrollOffset = 0
}

func (w *KeybindingsWidget) ensureVisible(visibleItems int) {
	if visibleItems <= 0 {
		return
	}
	if w.selected < w.scrollOffset {
		w.scrollOffset = w.selected
	}
	if w.selected >= w.scrollOffset+visibleItems {
		w.scrollOffset = w.selected - visibleItems + 1
	}
}

func (w *KeybindingsWidget) clampSelected() {
	if w.selected < w.scrollOffset {
		w.selected = w.scrollOffset
	} else if w.visibleItems > 0 && w.selected >= w.scrollOffset+w.visibleItems {
		w.selected = w.scrollOffset + w.visibleItems - 1
		if w.selected >= len(w.items) {
			w.selected = len(w.items) - 1
		}
	}
}

func describeKeyCombo(kev *tcell.EventKey) string {
	var parts []string

	mod := kev.Modifiers()
	if mod&tcell.ModCtrl != 0 {
		parts = append(parts, "ctrl")
	}
	if mod&tcell.ModShift != 0 {
		parts = append(parts, "shift")
	}
	if mod&tcell.ModAlt != 0 {
		parts = append(parts, "alt")
	}

	key := kev.Key()
	if name := specialKeyName(key); name != "" {
		parts = append(parts, name)
	} else if key == tcell.KeyRune {
		r := kev.Rune()
		if r == ' ' {
			parts = append(parts, "space")
		} else {
			parts = append(parts, string(r))
		}
	} else if key >= tcell.KeyCtrlA && key <= tcell.KeyCtrlZ {
		ch := 'a' + rune(key-tcell.KeyCtrlA)
		parts = append(parts, string(ch))
	} else {
		parts = append(parts, fmt.Sprintf("0x%x", int(key)))
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "+"
		}
		result += p
	}
	return result
}

func specialKeyName(key tcell.Key) string {
	names := map[tcell.Key]string{
		tcell.KeyEnter:      "enter",
		tcell.KeyTab:        "tab",
		tcell.KeyBacktab:    "shift+tab",
		tcell.KeyBackspace:  "backspace",
		tcell.KeyBackspace2: "backspace",
		tcell.KeyDelete:     "delete",
		tcell.KeyInsert:     "insert",
		tcell.KeyUp:         "up",
		tcell.KeyDown:       "down",
		tcell.KeyLeft:       "left",
		tcell.KeyRight:      "right",
		tcell.KeyHome:       "home",
		tcell.KeyEnd:        "end",
		tcell.KeyPgUp:       "pgup",
		tcell.KeyPgDn:       "pgdn",
		tcell.KeyF1:         "f1",
		tcell.KeyF2:         "f2",
		tcell.KeyF3:         "f3",
		tcell.KeyF4:         "f4",
		tcell.KeyF5:         "f5",
		tcell.KeyF6:         "f6",
		tcell.KeyF7:         "f7",
		tcell.KeyF8:         "f8",
		tcell.KeyF9:         "f9",
		tcell.KeyF10:        "f10",
		tcell.KeyF11:        "f11",
		tcell.KeyF12:        "f12",
		tcell.KeyEscape:     "escape",
	}
	if name, ok := names[key]; ok {
		return name
	}
	return ""
}
