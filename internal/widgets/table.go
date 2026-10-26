package widgets

import (
	"time"

	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type TableColumn struct {
	Label string
	Width int
	// MinWidth bounds an auto-sized column (Width 0); 0 means the label's width.
	MinWidth int
	Align    string // "left" (default), "right", "center"
	// Actions draws clickable links in every row instead of cell text. The
	// last column, when it has actions, stays pinned under horizontal scroll.
	Actions []TableAction
}

type TableAction struct {
	Label   string
	Command string
	Style   term.Style
	// Menu opens Entries as a dropdown at the action, or NodeMenu when
	// Entries is empty.
	Menu    bool
	Entries []MenuEntry
}

func (a TableAction) text() string {
	if a.Label == "" && a.Menu {
		return string(icons.Rune(icons.More))
	}
	return a.Label
}

func actionsWidth(actions []TableAction) int {
	w := 0
	for i, a := range actions {
		if i > 0 {
			w++
		}
		w += textwidth.String(a.text())
	}
	return w
}

type actionSpan struct {
	action TableAction
	x, w   int
}

type TableConfig struct {
	Columns     []TableColumn
	Rows        [][]string
	OnSelect    func(rowIndex int)
	OnActivate  func(rowIndex int)
	OnCommand   func(command string, rowIndex int)
	OnMenu      func(entries []MenuEntry, rowIndex int, screenX, screenY int)
	NodeMenu    []MenuEntry
	KeyCommands map[rune]string
	RowStyles   []term.Style
	// Loading and EmptyText fill the data area while there are no rows.
	Loading   bool
	EmptyText string
	// Inset indents cells on both sides; row highlight and scrollbar still span the full width.
	Inset int
}

type TableWidget struct {
	BaseWidget
	Config    TableConfig
	selected  int
	scrollTop int
	lastSel   int
	focused   bool
	pressed   bool
	clickRow  int
	clickAt   int64
	now       func() time.Time

	scrollbar                 scrollbar
	hbar                      horizontalScrollbar
	scrollLeft                int
	contentW                  int
	fullW                     int
	dataH                     int
	widths                    []int
	pinW                      int
	actionSpans               []actionSpan
	pointerCaptureInvalidated func()
}

const tableDoubleClickMs = 500

func (t *TableWidget) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
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
		t.dataH = 0
		_, invalidated := t.scrollbar.Render(surface, scrollbarGeometry{}, newScrollRange(0, len(t.Config.Rows), t.scrollTop))
		_, hInvalidated := t.hbar.Render(surface, scrollbarGeometry{}, newScrollRange(0, 0, t.scrollLeft))
		t.notifyPointerCaptureInvalidated(invalidated || hInvalidated)
		return
	}

	// Header takes 2 lines (labels + separator)
	headerH := 2
	minW := t.minContentWidth()
	viewW, dataH := fitScrollViewport(w, h-headerH, minW, len(t.Config.Rows))
	t.dataH = dataH

	rangeModel := newScrollRange(dataH, len(t.Config.Rows), t.scrollTop)
	t.scrollTop = rangeModel.offset

	t.ensureVisible(dataH)
	rangeModel = newScrollRange(dataH, len(t.Config.Rows), t.scrollTop)
	t.scrollTop = rangeModel.offset

	t.contentW = viewW
	t.fullW = max(viewW, minW)
	t.widths = t.effectiveWidths(t.cellsEnd() - t.inset())
	t.layoutActions()
	hRange := newScrollRange(viewW, t.fullW, t.scrollLeft)
	t.scrollLeft = hRange.offset

	line := newVirtualSurface(t.fullW, 1)
	t.renderHeader(surface, line)
	t.renderState(surface, headerH, dataH)

	for i := range dataH {
		idx := t.scrollTop + i
		if idx >= len(t.Config.Rows) {
			break
		}
		t.renderRow(surface, line, idx, headerH+i)
	}

	ox, oy := t.contentOrigin()
	geometry := scrollbarGeometry{
		localTrack: Rect{X: viewW, Y: headerH, W: 1, H: dataH},
		hitTrack:   Rect{X: ox + viewW, Y: oy + headerH, W: 1, H: dataH},
	}
	_, invalidated := t.scrollbar.Render(surface, geometry, rangeModel)
	hGeometry := scrollbarGeometry{
		localTrack: Rect{X: 0, Y: headerH + dataH, W: viewW, H: 1},
		hitTrack:   Rect{X: ox, Y: oy + headerH + dataH, W: viewW, H: 1},
	}
	_, hInvalidated := t.hbar.Render(surface, hGeometry, hRange)
	t.notifyPointerCaptureInvalidated(invalidated || hInvalidated)
}

func (t *TableWidget) inset() int {
	if t.Config.Inset <= 0 || t.contentW <= 2*t.Config.Inset {
		return 0
	}
	return t.Config.Inset
}

func (t *TableWidget) cellsEnd() int { return t.fullW - t.inset() }

func (t *TableWidget) columnGaps() int {
	if n := len(t.Config.Columns); n > 1 {
		return 2 * (n - 1)
	}
	return 0
}

func columnMinWidth(col TableColumn) int {
	switch {
	case len(col.Actions) > 0:
		return max(col.Width, textwidth.String(col.Label), actionsWidth(col.Actions), 1)
	case col.Width > 0:
		return col.Width
	case col.MinWidth > 0:
		return col.MinWidth
	}
	return max(textwidth.String(col.Label), 1)
}

func (t *TableWidget) minContentWidth() int {
	w := t.columnGaps() + 2*max(t.Config.Inset, 0)
	for _, col := range t.Config.Columns {
		w += columnMinWidth(col)
	}
	return w
}

// effectiveWidths keeps fixed widths; auto columns split the remaining space
// equally, except that none gets less than its minimum.
func (t *TableWidget) effectiveWidths(w int) []int {
	widths := make([]int, len(t.Config.Columns))
	remaining := w - t.columnGaps()
	var auto []int
	for i, col := range t.Config.Columns {
		if col.Width > 0 || len(col.Actions) > 0 {
			widths[i] = columnMinWidth(col)
			remaining -= widths[i]
		} else {
			auto = append(auto, i)
		}
	}
	for len(auto) > 0 {
		per := max(remaining, 0) / len(auto)
		var free []int
		for _, i := range auto {
			if m := columnMinWidth(t.Config.Columns[i]); m > per {
				widths[i] = m
				remaining -= m
			} else {
				free = append(free, i)
			}
		}
		if len(free) < len(auto) {
			auto = free
			continue
		}
		rem := max(remaining, 0) % len(auto)
		for _, i := range auto {
			widths[i] = per
			if rem > 0 {
				widths[i]++
				rem--
			}
		}
		break
	}
	return widths
}

func (t *TableWidget) layoutActions() {
	t.pinW = 0
	t.actionSpans = t.actionSpans[:0]
	x := t.inset()
	last := len(t.Config.Columns) - 1
	for i, col := range t.Config.Columns {
		if i > 0 {
			x += 2
		}
		if len(col.Actions) > 0 {
			ax := x + alignOffset(col.Align, t.widths[i]-actionsWidth(col.Actions))
			for j, a := range col.Actions {
				if j > 0 {
					ax++
				}
				w := textwidth.String(a.text())
				t.actionSpans = append(t.actionSpans, actionSpan{action: a, x: ax, w: w})
				ax += w
			}
			if i == last {
				t.pinW = t.fullW - x
				if i > 0 {
					t.pinW += 2
				}
			}
		}
		x += t.widths[i]
	}
}

func alignOffset(align string, padding int) int {
	switch {
	case padding <= 0:
		return 0
	case align == "right":
		return padding
	case align == "center":
		return padding / 2
	}
	return 0
}

func (t *TableWidget) pinStart() int {
	return max(t.contentW-t.pinW, 0)
}

func (t *TableWidget) lineX(x int) int {
	if t.pinW > 0 && x >= t.pinStart() {
		return t.fullW - t.contentW + x
	}
	return x + t.scrollLeft
}

func (t *TableWidget) blitLine(surface Surface, line *virtualSurface, y int) {
	cells := line.cells[0]
	pin := t.pinStart()
	for x := range t.contentW {
		sx := t.lineX(x)
		if sx >= len(cells) {
			break
		}
		c := cells[sx]
		if (x == t.contentW-1 || (t.pinW > 0 && x == pin-1)) && textwidth.Rune(c.Ch) > 1 {
			c.Ch = ' '
		}
		surface.SetCell(x, y, c)
	}
}

func (t *TableWidget) drawColumns(line *virtualSurface, texts func(i int) string, style term.Style, rowActions bool) {
	line.Fill(term.Cell{Ch: ' ', Style: style})
	x := t.inset()
	for i, col := range t.Config.Columns {
		if i > 0 {
			x += 2
		}
		if !rowActions || len(col.Actions) == 0 {
			t.renderCell(line, texts(i), col, t.widths[i], x, 0, style)
		}
		x += t.widths[i]
	}
	if rowActions {
		t.renderActions(line, style)
	}
}

func (t *TableWidget) renderActions(line *virtualSurface, rowStyle term.Style) {
	highlighted := rowStyle == term.StyleSidebarSelected
	end := t.cellsEnd()
	for _, span := range t.actionSpans {
		a := span.action
		cell := term.Cell{Style: rowStyle}
		switch {
		case a.Style != 0 && !highlighted:
			cell.Style, cell.BgStyle = a.Style, rowStyle
		case !a.Menu:
			cell.Underline = true
		}
		x := span.x
		for _, ch := range a.text() {
			cw := textwidth.Rune(ch)
			if x+cw > end {
				break
			}
			cell.Ch = ch
			line.SetCell(x, 0, cell)
			x += cw
		}
	}
}

func (t *TableWidget) actionAt(x int) (actionSpan, bool) {
	lx := t.lineX(x)
	for _, span := range t.actionSpans {
		if lx >= span.x && lx < span.x+span.w {
			return span, true
		}
	}
	return actionSpan{}, false
}

func (t *TableWidget) renderHeader(surface Surface, line *virtualSurface) {
	t.drawColumns(line, func(i int) string { return t.Config.Columns[i].Label }, term.StyleHoverBold, false)
	t.blitLine(surface, line, 0)

	sepStyle := term.StyleBorder
	for sx := 0; sx < t.contentW; sx++ {
		surface.SetCell(sx, 1, term.Cell{Ch: '─', Style: sepStyle})
	}
}

func (t *TableWidget) renderState(surface Surface, headerH, dataH int) {
	w := t.contentW - t.inset()
	if len(t.Config.Rows) > 0 {
		if t.Config.Loading && w > 12 {
			label := "Loading… "
			surface.DrawText(w-textwidth.String(label), 0, label, w, term.StyleMuted)
		}
		return
	}
	text := t.Config.EmptyText
	if t.Config.Loading {
		text = "Loading…"
	}
	if text == "" || dataH <= 0 {
		return
	}
	y := headerH + min(dataH/2, 2)
	x := max((t.contentW-textwidth.String(text))/2, 0)
	surface.DrawText(x, y, text, t.contentW, term.StyleMuted)
}

func (t *TableWidget) renderRow(surface Surface, line *virtualSurface, idx, y int) {
	style := term.StyleDefault
	if idx < len(t.Config.RowStyles) && t.Config.RowStyles[idx] != 0 {
		style = t.Config.RowStyles[idx]
	}
	if idx == t.selected && t.focused {
		style = term.StyleSidebarSelected
	}

	row := t.Config.Rows[idx]
	t.drawColumns(line, func(i int) string {
		if i < len(row) {
			return row[i]
		}
		return ""
	}, style, true)
	t.blitLine(surface, line, y)
}

func (t *TableWidget) renderCell(surface Surface, text string, col TableColumn, colW, x, y int, style term.Style) {
	runes := []rune(text)
	if colW <= 0 {
		return
	}

	end := t.cellsEnd()
	maxX := min(x+colW, end)
	textW := textwidth.Runes(runes)
	if textW > colW {
		drawRunesClipped(surface, x, y, maxX, runes, style)
		return
	}

	padding := colW - textW
	switch col.Align {
	case "right":
		for i := 0; i < padding && x+i < end; i++ {
			surface.SetCell(x+i, y, term.Cell{Ch: ' ', Style: style})
		}
		drawRunesClipped(surface, x+padding, y, maxX, runes, style)
	case "center":
		leftPad := padding / 2
		for i := 0; i < leftPad && x+i < end; i++ {
			surface.SetCell(x+i, y, term.Cell{Ch: ' ', Style: style})
		}
		drawRunesClipped(surface, x+leftPad, y, maxX, runes, style)
	default: // left
		drawRunesClipped(surface, x, y, maxX, runes, style)
	}
}

func (t *TableWidget) HandleEvent(ev tcell.Event) EventResult {
	if newLeft, result := t.hbar.HandleEvent(ev); result != EventIgnored {
		t.scrollLeft = newLeft
		return result
	}
	if newTop, result := t.scrollbar.HandleEvent(ev); result != EventIgnored {
		t.scrollTop = newTop
		return result
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

func (t *TableWidget) CancelPointerCapture() bool {
	canceled := t.scrollbar.cancel()
	if t.hbar.cancel() {
		canceled = true
	}
	t.notifyPointerCaptureInvalidated(canceled)
	return canceled
}

func (t *TableWidget) OwnsPointerCapture() bool {
	return t.scrollbar.isDragging() || t.hbar.isDragging()
}

func (t *TableWidget) hRange() scrollRange {
	return newScrollRange(t.contentW, t.fullW, t.scrollLeft)
}

func (t *TableWidget) scrollHorizontally(delta int) {
	t.scrollLeft = t.hRange().clampOffset(t.scrollLeft + delta)
}

func (t *TableWidget) columnStops() []int {
	stops := []int{0}
	x := t.inset()
	for i, w := range t.widths {
		if i > 0 {
			x += 2
		}
		if x > 0 {
			stops = append(stops, x)
		}
		x += w
	}
	return stops
}

func (t *TableWidget) stepColumn(dir int) {
	step := max(t.contentW, 1)
	target := t.scrollLeft + dir*step
	for _, s := range t.columnStops() {
		if dir > 0 && s > t.scrollLeft && s < target {
			target = s
			break
		}
		if dir < 0 && s < t.scrollLeft && s > target {
			target = s
		}
	}
	t.scrollLeft = t.hRange().clampOffset(target)
}

func (t *TableWidget) InvalidatePointerInteraction() bool {
	return t.CancelPointerCapture()
}

func (t *TableWidget) SetPointerCaptureInvalidated(invalidated func()) {
	t.pointerCaptureInvalidated = invalidated
}

func (t *TableWidget) notifyPointerCaptureInvalidated(invalidated bool) {
	if invalidated && t.pointerCaptureInvalidated != nil {
		t.pointerCaptureInvalidated()
	}
}

func (t *TableWidget) handleMouse(ev *tcell.EventMouse) EventResult {
	btn := ev.Buttons()
	fresh := btn&tcell.Button1 != 0 && !t.pressed
	t.pressed = btn&tcell.Button1 != 0
	mx, my := ev.Position()
	r := t.rect

	if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
		return EventIgnored
	}

	headerH := 2

	shift := ev.Modifiers()&tcell.ModShift != 0
	if btn&tcell.WheelLeft != 0 || (btn&tcell.WheelUp != 0 && shift) {
		t.scrollHorizontally(-3)
		return EventConsumed
	}
	if btn&tcell.WheelRight != 0 || (btn&tcell.WheelDown != 0 && shift) {
		t.scrollHorizontally(3)
		return EventConsumed
	}
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

	ox, oy := t.contentOrigin()
	onHBar := t.hbar.visible() && my >= oy+headerH+t.dataH
	localY := my - r.Y
	idx := t.scrollTop + (localY - headerH)
	if localY < headerH || onHBar || idx < 0 || idx >= len(t.Config.Rows) {
		if fresh {
			t.clickAt = 0
		}
		return EventIgnored
	}

	if btn&tcell.Button2 != 0 {
		t.clickAt = 0
		t.selected = idx
		if t.Config.OnMenu == nil || len(t.Config.NodeMenu) == 0 {
			return EventIgnored
		}
		t.Config.OnMenu(t.Config.NodeMenu, idx, mx, my)
		return EventConsumed
	}

	if btn&tcell.Button1 != 0 {
		t.selected = idx
		if t.Config.OnSelect != nil {
			t.Config.OnSelect(idx)
		}
		if span, ok := t.actionAt(mx - ox); fresh && ok {
			t.clickAt = 0
			t.runAction(span, idx, mx-(t.lineX(mx-ox)-span.x), my+1)
			return EventConsumed
		}
		if fresh && t.Config.OnActivate != nil {
			now := t.clock().UnixMilli()
			if idx == t.clickRow && now-t.clickAt < tableDoubleClickMs {
				t.clickAt = 0
				t.Config.OnActivate(idx)
			} else {
				t.clickRow, t.clickAt = idx, now
			}
		}
		return EventConsumed
	}

	return EventIgnored
}

func (t *TableWidget) runAction(span actionSpan, idx, screenX, screenY int) {
	a := span.action
	if !a.Menu {
		if t.Config.OnCommand != nil {
			t.Config.OnCommand(a.Command, idx)
		}
		return
	}
	entries := a.Entries
	if len(entries) == 0 {
		entries = t.Config.NodeMenu
	}
	if t.Config.OnMenu != nil && len(entries) > 0 {
		t.Config.OnMenu(entries, idx, screenX, screenY)
	}
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
	case tcell.KeyLeft, tcell.KeyRight:
		if !t.hRange().visible() {
			return EventIgnored
		}
		if ev.Key() == tcell.KeyLeft {
			t.stepColumn(-1)
		} else {
			t.stepColumn(1)
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
			if t.Config.OnActivate != nil {
				t.Config.OnActivate(t.selected)
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
