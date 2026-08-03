package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type TableColumn struct {
	Label string
	Width int
	Align string // "left" (default), "right", "center"
}

type TableConfig struct {
	Columns     []TableColumn
	Rows        [][]string
	OnSelect    func(rowIndex int)
	OnCommand   func(command string, rowIndex int)
	OnMenu      func(entries []MenuEntry, rowIndex int, screenX, screenY int)
	NodeMenu    []MenuEntry
	KeyCommands map[rune]string
}

type TableWidget struct {
	BaseWidget
	Config    TableConfig
	selected  int
	scrollTop int
	lastSel   int
	focused   bool

	scrollbar scrollbar
	contentW  int
	widths    []int
}

func NewTableWidget(cfg TableConfig) *TableWidget {
	return &TableWidget{Config: cfg}
}

func (t *TableWidget) Height() int { return 0 }
func (t *TableWidget) Width() int  { return 0 }

// ContentHeight reports header + rows so scroll views can measure the table.
func (t *TableWidget) ContentHeight() int {
	h := len(t.Config.Rows) + t.BoxOverheadH()
	if len(t.Config.Columns) > 0 {
		h++ // header row
	}
	return h
}
func (t *TableWidget) Focusable() bool    { return true }
func (t *TableWidget) SetFocused(f bool)  { t.focused = f }
func (t *TableWidget) IsFocused() bool    { return t.focused }
func (t *TableWidget) SelectedIndex() int { return t.selected }

func (t *TableWidget) SetSelectedIndex(i int) {
	t.selected = i
	t.clampSelected()
}

func (t *TableWidget) clampSelected() {
	if t.selected >= len(t.Config.Rows) {
		t.selected = len(t.Config.Rows) - 1
	}
	if t.selected < 0 {
		t.selected = 0
	}
}

func (t *TableWidget) ensureVisible(visibleH int) {
	if t.selected != t.lastSel {
		t.lastSel = t.selected
		if t.selected < t.scrollTop {
			t.scrollTop = t.selected
		}
		if t.selected >= t.scrollTop+visibleH {
			t.scrollTop = t.selected - visibleH + 1
		}
	}
}

func (t *TableWidget) Render(surface Surface) {
	surface = t.RenderBox(surface)
	w, h := surface.Size()
	surface.Fill(term.Cell{Ch: ' '})

	if h <= 0 || w <= 0 || len(t.Config.Columns) == 0 {
		return
	}

	// Header takes 2 lines (labels + separator)
	headerH := 2
	dataH := h - headerH
	if dataH < 0 {
		dataH = 0
	}

	maxScroll := len(t.Config.Rows) - dataH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.scrollTop > maxScroll {
		t.scrollTop = maxScroll
	}

	t.ensureVisible(dataH)

	t.scrollbar.X = t.rect.X + w - 1
	t.scrollbar.Y = t.rect.Y + headerH
	t.scrollbar.Height = dataH
	t.scrollbar.TotalItems = len(t.Config.Rows)
	t.scrollbar.TopItem = t.scrollTop

	t.contentW = w
	if t.scrollbar.visible() {
		t.contentW = w - 1
	}
	t.widths = t.effectiveWidths(t.contentW)

	t.renderHeader(surface, t.contentW)

	for i := range dataH {
		idx := t.scrollTop + i
		if idx >= len(t.Config.Rows) {
			break
		}
		t.renderRow(surface, idx, headerH+i, t.contentW)
	}

	t.scrollbar.Render(surface, w-1, headerH)
}

// effectiveWidths keeps fixed widths; auto columns split the remaining space (min 1 cell).
func (t *TableWidget) effectiveWidths(w int) []int {
	n := len(t.Config.Columns)
	widths := make([]int, n)
	sep := 0
	if n > 1 {
		sep = 2 * (n - 1)
	}
	fixed := 0
	autoCount := 0
	for i, col := range t.Config.Columns {
		if col.Width > 0 {
			widths[i] = col.Width
			fixed += col.Width
		} else {
			autoCount++
		}
	}
	if autoCount > 0 {
		remaining := w - fixed - sep
		if remaining < autoCount {
			remaining = autoCount
		}
		per := remaining / autoCount
		rem := remaining % autoCount
		for i := range widths {
			if widths[i] == 0 {
				widths[i] = per
				if rem > 0 {
					widths[i]++
					rem--
				}
			}
		}
	}
	return widths
}

func (t *TableWidget) renderHeader(surface Surface, w int) {
	style := term.StyleHoverBold
	x := 0
	for i, col := range t.Config.Columns {
		if i > 0 {
			if x < w {
				surface.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
				x++
			}
			if x < w {
				surface.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
				x++
			}
		}
		t.renderCell(surface, col.Label, col, t.widths[i], x, 0, style)
		x += t.widths[i]
	}
	// Fill remaining header space
	for fx := x; fx < w; fx++ {
		surface.SetCell(fx, 0, term.Cell{Ch: ' ', Style: style})
	}

	// Separator line
	sepStyle := term.StyleBorder
	for sx := 0; sx < w; sx++ {
		surface.SetCell(sx, 1, term.Cell{Ch: '─', Style: sepStyle})
	}
}

func (t *TableWidget) renderRow(surface Surface, idx, y, w int) {
	style := term.StyleDefault
	if idx == t.selected && t.focused {
		style = term.StyleSidebarSelected
	}

	for fx := 0; fx < w; fx++ {
		surface.SetCell(fx, y, term.Cell{Ch: ' ', Style: style})
	}

	row := t.Config.Rows[idx]
	x := 0
	for i, col := range t.Config.Columns {
		if i > 0 {
			if x < w {
				surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
				x++
			}
			if x < w {
				surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
				x++
			}
		}
		cellText := ""
		if i < len(row) {
			cellText = row[i]
		}
		t.renderCell(surface, cellText, col, t.widths[i], x, y, style)
		x += t.widths[i]
	}
}

func (t *TableWidget) renderCell(surface Surface, text string, col TableColumn, colW, x, y int, style term.Style) {
	runes := []rune(text)
	if colW <= 0 {
		return
	}

	maxX := min(x+colW, t.contentW)
	textW := textwidth.Runes(runes)
	if textW > colW {
		drawRunesClipped(surface, x, y, maxX, runes, style)
		return
	}

	padding := colW - textW
	switch col.Align {
	case "right":
		for i := 0; i < padding && x+i < t.contentW; i++ {
			surface.SetCell(x+i, y, term.Cell{Ch: ' ', Style: style})
		}
		drawRunesClipped(surface, x+padding, y, maxX, runes, style)
	case "center":
		leftPad := padding / 2
		for i := 0; i < leftPad && x+i < t.contentW; i++ {
			surface.SetCell(x+i, y, term.Cell{Ch: ' ', Style: style})
		}
		drawRunesClipped(surface, x+leftPad, y, maxX, runes, style)
	default: // left
		drawRunesClipped(surface, x, y, maxX, runes, style)
	}
}

func (t *TableWidget) HandleEvent(ev tcell.Event) EventResult {
	if newTop, consumed := t.scrollbar.HandleEvent(ev); consumed {
		t.scrollTop = newTop
		return EventConsumed
	}

	switch tev := ev.(type) {
	case *tcell.EventMouse:
		// handleMouse fires OnSelect explicitly on click.
		return t.handleMouse(tev)
	case *tcell.EventKey:
		prev := t.selected
		result := t.handleKey(tev)
		if t.selected != prev && t.Config.OnSelect != nil {
			t.Config.OnSelect(t.selected)
		}
		return result
	}
	return EventIgnored
}

func (t *TableWidget) handleMouse(ev *tcell.EventMouse) EventResult {
	btn := ev.Buttons()
	mx, my := ev.Position()
	r := t.rect

	if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
		return EventIgnored
	}

	headerH := 2

	if btn&tcell.WheelUp != 0 {
		t.scrollTop -= 3
		if t.scrollTop < 0 {
			t.scrollTop = 0
		}
		return EventConsumed
	}
	if btn&tcell.WheelDown != 0 {
		dataH := r.H - headerH
		if dataH < 0 {
			dataH = 0
		}
		max := len(t.Config.Rows) - dataH
		if max < 0 {
			max = 0
		}
		t.scrollTop += 3
		if t.scrollTop > max {
			t.scrollTop = max
		}
		return EventConsumed
	}

	localY := my - r.Y
	if localY < headerH {
		return EventIgnored
	}

	idx := t.scrollTop + (localY - headerH)
	if idx < 0 || idx >= len(t.Config.Rows) {
		return EventIgnored
	}

	if btn&tcell.Button2 != 0 {
		t.selected = idx
		if t.Config.OnMenu != nil && len(t.Config.NodeMenu) > 0 {
			t.Config.OnMenu(t.Config.NodeMenu, idx, mx, my)
		}
		return EventConsumed
	}

	if btn&tcell.Button1 != 0 {
		t.selected = idx
		if t.Config.OnSelect != nil {
			t.Config.OnSelect(idx)
		}
		return EventConsumed
	}

	return EventIgnored
}

func (t *TableWidget) handleKey(ev *tcell.EventKey) EventResult {
	switch ev.Key() {
	case tcell.KeyUp:
		if t.selected > 0 {
			t.selected--
		}
		return EventConsumed
	case tcell.KeyDown:
		if t.selected < len(t.Config.Rows)-1 {
			t.selected++
		}
		return EventConsumed
	case tcell.KeyEnter:
		if t.selected >= 0 && t.selected < len(t.Config.Rows) {
			if ev.Modifiers()&tcell.ModShift != 0 {
				if t.Config.OnMenu != nil && len(t.Config.NodeMenu) > 0 {
					r := t.GetRect()
					t.Config.OnMenu(t.Config.NodeMenu, t.selected, r.X, r.Y+2+t.selected-t.scrollTop)
				}
				return EventConsumed
			}
			if t.Config.OnSelect != nil {
				t.Config.OnSelect(t.selected)
			}
		}
		return EventConsumed
	case tcell.KeyRune:
		if t.handleShortcutKey(term.KeyRune(ev)) == EventConsumed {
			return EventConsumed
		}
	}
	return EventIgnored
}

func (t *TableWidget) handleShortcutKey(r rune) EventResult {
	if t.Config.KeyCommands == nil {
		return EventIgnored
	}
	cmd, ok := t.Config.KeyCommands[r]
	if !ok {
		return EventIgnored
	}
	if t.Config.OnCommand != nil && t.selected >= 0 && t.selected < len(t.Config.Rows) {
		t.Config.OnCommand(cmd, t.selected)
	}
	return EventConsumed
}
