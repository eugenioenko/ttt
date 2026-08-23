package widgets

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type SelectItem struct {
	ID    string
	Label string
}

type SelectConfig struct {
	Items       []SelectItem
	Placeholder string
	ShowDivider bool
	Collapsible bool
	// OnOpen fires when the list opens, letting an owner close sibling selects.
	OnOpen    func()
	OnSelect  func(id string)
	OnChange  func(id string)
	OnDismiss func()
}

type SelectWidget struct {
	BaseWidget
	// FixedWidth bounds a collapsed select so its right-aligned chevron stays
	// next to the value instead of drifting to the far edge of the pane.
	FixedWidth int
	Config     SelectConfig
	input      *InputWidget
	divider    *DividerWidget
	filtered   []int
	selected   int
	focused    bool

	scrollTop                 int
	scrollbar                 scrollbar
	popupBounds               Rect
	open                      bool
	suppress                  bool
	currentID                 string
	pointerCaptureInvalidated func()
}

func NewSelectWidget(config SelectConfig) *SelectWidget {
	s := &SelectWidget{
		Config:  config,
		divider: NewDividerWidget(DividerConfig{}),
	}
	s.input = NewInputWidget(InputConfig{
		Placeholder: config.Placeholder,
		OnChange: func(_ string) {
			if s.suppress {
				return
			}
			if s.Config.Collapsible && !s.open {
				s.setOpen(true)
			}
			s.filter()
			s.selected = 0
			s.scrollTop = 0
			if s.Config.OnChange != nil && len(s.filtered) > 0 {
				s.Config.OnChange(s.Config.Items[s.filtered[0]].ID)
			}
		},
	})
	s.filter()
	return s
}

// SetSelectedID preselects an item and shows its label as the placeholder, so a
// collapsed select displays its current value while the text field stays free
// for filtering. An id with no matching item is still shown, so a value written
// by hand into settings.json does not render as blank.
func (s *SelectWidget) SetSelectedID(id string) {
	s.currentID = id
	label := id
	for _, item := range s.Config.Items {
		if item.ID == id {
			label = item.Label
			break
		}
	}
	s.input.Config.Placeholder = label
	s.syncSelectedToCurrent()
}

// syncSelectedToCurrent points the highlighted row at currentID within the
// filtered list. Without this the index survives a filter change and ends up
// referring to a different item.
func (s *SelectWidget) syncSelectedToCurrent() {
	s.selected = 0
	for fi, idx := range s.filtered {
		if s.Config.Items[idx].ID == s.currentID {
			s.selected = fi
			return
		}
	}
}

func (s *SelectWidget) selectedID() string {
	if s.selected >= 0 && s.selected < len(s.filtered) {
		return s.Config.Items[s.filtered[s.selected]].ID
	}
	return ""
}

func (s *SelectWidget) Height() int {
	if s.Config.Collapsible {
		return 1
	}
	h := len(s.Config.Items) + 1
	if s.Config.ShowDivider {
		h += 2
	}
	max := 11
	if h > max {
		h = max
	}
	return h
}

func (s *SelectWidget) popupHeight() int {
	h := len(s.filtered)
	max := 8
	if h > max {
		h = max
	}
	if h < 1 {
		h = 1
	}
	return h
}

// Keyed off open rather than focused, so a container can decide independently
// whether to paint the popup.
func (s *SelectWidget) HasPopup() bool {
	return s.Config.Collapsible && s.open
}

func (s *SelectWidget) setOpen(open bool) {
	wasOpen := s.open
	s.open = open
	if !open {
		s.notifyPointerCaptureInvalidated(s.scrollbar.cancel())
		s.suppress = true
		s.input.SetText("")
		s.suppress = false
		s.filter()
		// The filter query is gone, so the highlighted row must be re-resolved
		// against the restored full list.
		s.syncSelectedToCurrent()
		return
	}
	if !wasOpen && s.Config.OnOpen != nil {
		s.Config.OnOpen()
	}
}

// commitSelection records the highlighted item as the current value and
// notifies the owner. Recording it is what lets the highlight survive the
// filter being cleared when the list closes.
func (s *SelectWidget) commitSelection() {
	if s.selected < 0 || s.selected >= len(s.filtered) {
		return
	}
	id := s.Config.Items[s.filtered[s.selected]].ID
	s.currentID = id
	if s.Config.OnSelect != nil {
		s.Config.OnSelect(id)
	}
}

// ClosePopup collapses the list without touching the selected value.
func (s *SelectWidget) ClosePopup() {
	if s.open {
		s.setOpen(false)
	}
}

// Zero-height bounds means unconstrained.
func (s *SelectWidget) SetPopupBounds(r Rect) {
	s.popupBounds = r
}

func (s *SelectWidget) PopupRect() Rect {
	r := s.GetRect()
	h := s.popupHeight() + 2 // border top and bottom
	below := r.Y + 1
	y := below

	if b := s.popupBounds; b.H > 0 {
		if y+h > b.Y+b.H {
			above := r.Y - h
			if above >= b.Y && r.Y-b.Y > (b.Y+b.H)-below {
				y = above
			}
		}
	}
	// Full control width, so the popup also covers the chevron column of any
	// control it overlaps.
	return Rect{X: r.X, Y: y, W: r.W, H: h}
}

func (s *SelectWidget) RenderPopup(surface Surface) {
	pr := s.PopupRect()
	w, h := pr.W, pr.H
	listH := h - 2
	if w <= 2 || listH <= 0 {
		_, invalidated := s.scrollbar.Render(surface, scrollbarGeometry{}, newScrollRange(0, len(s.filtered), s.scrollTop))
		s.notifyPointerCaptureInvalidated(invalidated)
		return
	}

	s.ensureVisible(listH)
	rangeModel := newScrollRange(listH, len(s.filtered), s.scrollTop)
	s.scrollTop = rangeModel.offset

	// Fill first so the popup is opaque over whatever it covers.
	for y := range h {
		for x := range w {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StylePaletteItem})
		}
	}
	surface.DrawBorder(0, 0, w, h, widgetBorders(s.Box), term.StyleBorder)

	for i := range listH {
		idx := s.scrollTop + i
		if idx >= len(s.filtered) {
			break
		}
		item := s.Config.Items[s.filtered[idx]]
		style := term.StylePaletteItem
		if idx == s.selected {
			style = term.StylePaletteSelected
		}
		for x := 1; x < w-1; x++ {
			surface.SetCell(x, i+1, term.Cell{Ch: ' ', Style: style})
		}
		surface.DrawText(2, i+1, item.Label, w-4, style)
	}

	geometry := scrollbarGeometry{
		localTrack: Rect{X: w - 2, Y: 1, W: 1, H: listH},
		hitTrack:   Rect{X: pr.X + w - 2, Y: pr.Y + 1, W: 1, H: listH},
	}
	_, invalidated := s.scrollbar.Render(surface, geometry, rangeModel)
	s.notifyPointerCaptureInvalidated(invalidated)
}
func (s *SelectWidget) Width() int { return s.FixedWidth }

func (s *SelectWidget) Focusable() bool { return true }
func (s *SelectWidget) SetFocused(f bool) {
	s.focused = f
	s.input.SetFocused(f)
	if !f && s.Config.Collapsible {
		s.setOpen(false)
	}
}
func (s *SelectWidget) IsFocused() bool { return s.focused }

func (s *SelectWidget) CursorPosition() (x, y int, visible bool) {
	return s.input.CursorPosition()
}

func (s *SelectWidget) filter() {
	query := strings.ToLower(strings.TrimSpace(s.input.Text()))
	s.filtered = s.filtered[:0]
	for i, item := range s.Config.Items {
		if query == "" || strings.Contains(strings.ToLower(item.Label), query) {
			s.filtered = append(s.filtered, i)
		}
	}
}

func (s *SelectWidget) visibleListH() int {
	if s.Config.Collapsible {
		return s.popupHeight()
	}
	listStart := 1
	if s.Config.ShowDivider {
		listStart = 3
	}
	return s.rect.H - listStart
}

func (s *SelectWidget) ensureVisible(visibleH int) {
	if visibleH <= 0 {
		return
	}
	if s.selected < s.scrollTop {
		s.scrollTop = s.selected
	}
	if s.selected >= s.scrollTop+visibleH {
		s.scrollTop = s.selected - visibleH + 1
	}
}

func (s *SelectWidget) Render(surface Surface) {
	w, h := surface.Size()
	if w <= 0 || h <= 0 {
		_, invalidated := s.scrollbar.Render(surface, scrollbarGeometry{}, newScrollRange(0, len(s.filtered), s.scrollTop))
		s.notifyPointerCaptureInvalidated(invalidated)
		return
	}

	y := 0
	if s.Config.ShowDivider {
		s.divider.SetRect(Rect{X: s.rect.X, Y: s.rect.Y + y, W: s.rect.W, H: 1})
		divSurface := surface.Sub(Rect{X: 0, Y: y, W: w, H: 1})
		s.divider.Render(divSurface)
		y++
	}

	inputW := w - 2
	s.input.SetRect(Rect{X: s.rect.X, Y: s.rect.Y + y, W: inputW, H: 1})
	inputSurface := surface.Sub(Rect{X: 0, Y: y, W: inputW, H: 1})
	s.input.Render(inputSurface)

	chevron := '▼'
	if (s.Config.Collapsible && s.open) || (!s.Config.Collapsible && s.focused) {
		chevron = '▲'
	}
	surface.SetCell(w-2, y, term.Cell{Ch: chevron, Style: term.StyleMuted})
	y++

	// A collapsed select is just the one-line control; its list lives in a popup.
	if s.Config.Collapsible {
		if !s.open {
			_, invalidated := s.scrollbar.Render(surface, scrollbarGeometry{}, newScrollRange(0, len(s.filtered), s.scrollTop))
			s.notifyPointerCaptureInvalidated(invalidated)
		}
		return
	}

	if s.Config.ShowDivider {
		s.divider.SetRect(Rect{X: s.rect.X, Y: s.rect.Y + y, W: s.rect.W, H: 1})
		divSurface := surface.Sub(Rect{X: 0, Y: y, W: w, H: 1})
		s.divider.Render(divSurface)
		y++
	}

	listStart := y
	listH := h - listStart
	if listH <= 0 {
		_, invalidated := s.scrollbar.Render(surface, scrollbarGeometry{}, newScrollRange(0, len(s.filtered), s.scrollTop))
		s.notifyPointerCaptureInvalidated(invalidated)
		return
	}

	rangeModel := newScrollRange(listH, len(s.filtered), s.scrollTop)
	s.scrollTop = rangeModel.offset

	for i := range listH {
		idx := s.scrollTop + i
		ly := listStart + i
		if idx >= len(s.filtered) {
			break
		}
		item := s.Config.Items[s.filtered[idx]]
		style := term.StylePaletteItem
		if idx == s.selected {
			style = term.StylePaletteSelected
		}
		for x := range w {
			surface.SetCell(x, ly, term.Cell{Ch: ' ', Style: style})
		}
		surface.DrawText(1, ly, item.Label, w-1, style)
	}

	geometry := scrollbarGeometry{
		localTrack: Rect{X: w - 1, Y: listStart, W: 1, H: listH},
		hitTrack:   Rect{X: s.rect.X + w - 1, Y: s.rect.Y + listStart, W: 1, H: listH},
	}
	_, invalidated := s.scrollbar.Render(surface, geometry, rangeModel)
	s.notifyPointerCaptureInvalidated(invalidated)
}

func (s *SelectWidget) HandleEvent(ev tcell.Event) EventResult {
	if newTop, result := s.scrollbar.HandleEvent(ev); result != EventIgnored {
		s.scrollTop = newTop
		return result
	}

	switch tev := ev.(type) {
	case *tcell.EventKey:
		return s.handleKey(tev)
	case *tcell.EventMouse:
		return s.handleMouse(tev)
	}
	return EventIgnored
}

func (s *SelectWidget) CancelPointerCapture() bool {
	canceled := s.scrollbar.cancel()
	s.notifyPointerCaptureInvalidated(canceled)
	return canceled
}

func (s *SelectWidget) OwnsPointerCapture() bool {
	return s.scrollbar.isDragging()
}

func (s *SelectWidget) InvalidatePointerInteraction() bool {
	return s.CancelPointerCapture()
}

func (s *SelectWidget) SetPointerCaptureInvalidated(invalidated func()) {
	s.pointerCaptureInvalidated = invalidated
}

func (s *SelectWidget) notifyPointerCaptureInvalidated(invalidated bool) {
	if invalidated && s.pointerCaptureInvalidated != nil {
		s.pointerCaptureInvalidated()
	}
}

func (s *SelectWidget) handleKey(ev *tcell.EventKey) EventResult {
	// A collapsed select opens on Up/Down/Enter before it starts navigating.
	if s.Config.Collapsible && !s.open {
		switch ev.Key() {
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyEnter:
			s.setOpen(true)
			return EventConsumed
		}
	}

	switch ev.Key() {
	case tcell.KeyUp:
		if s.selected > 0 {
			s.selected--
			s.ensureVisible(s.visibleListH())
			s.notifyChange()
		}
		return EventConsumed
	case tcell.KeyDown:
		if s.selected < len(s.filtered)-1 {
			s.selected++
			s.ensureVisible(s.visibleListH())
			s.notifyChange()
		}
		return EventConsumed
	case tcell.KeyEnter:
		if s.selected >= 0 && s.selected < len(s.filtered) {
			s.commitSelection()
		}
		if s.Config.Collapsible {
			s.setOpen(false)
		}
		return EventConsumed
	case tcell.KeyEscape:
		if s.Config.Collapsible && s.open {
			s.setOpen(false)
			return EventConsumed
		}
		if s.Config.OnDismiss != nil {
			s.Config.OnDismiss()
		}
		return EventConsumed
	default:
		return s.input.HandleEvent(ev)
	}
}

func (s *SelectWidget) handleMouse(ev *tcell.EventMouse) EventResult {
	btn := ev.Buttons()
	mx, my := ev.Position()
	r := s.rect

	if btn&tcell.WheelUp != 0 || btn&tcell.WheelDown != 0 {
		if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
			return EventIgnored
		}
	}

	if btn&tcell.WheelUp != 0 {
		s.scrollTop -= 3
		if s.scrollTop < 0 {
			s.scrollTop = 0
		}
		return EventConsumed
	}
	if btn&tcell.WheelDown != 0 {
		s.scrollTop += 3
		maxScroll := len(s.filtered) - s.visibleListH()
		if maxScroll < 0 {
			maxScroll = 0
		}
		if s.scrollTop > maxScroll {
			s.scrollTop = maxScroll
		}
		return EventConsumed
	}

	if btn&tcell.Button1 != 0 {
		if s.Config.Collapsible {
			if s.open {
				pr := s.PopupRect()
				// The popup's first and last rows are its border.
				if mx >= pr.X && mx < pr.X+pr.W && my > pr.Y && my < pr.Y+pr.H-1 {
					idx := s.scrollTop + (my - pr.Y - 1)
					if idx >= 0 && idx < len(s.filtered) {
						s.selected = idx
						s.notifyChange()
						s.commitSelection()
					}
					s.setOpen(false)
					return EventConsumed
				}
			}
			// Clicking the control itself toggles the list.
			if mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+1 {
				s.setOpen(!s.open)
				return EventConsumed
			}
			if s.open {
				s.setOpen(false)
			}
			return EventIgnored
		}
		listStart := r.Y + 1
		if s.Config.ShowDivider {
			listStart += 2
		}
		if my < listStart {
			return s.input.HandleEvent(ev)
		}
		idx := s.scrollTop + (my - listStart)
		if idx >= 0 && idx < len(s.filtered) {
			s.selected = idx
			s.notifyChange()
			if s.Config.OnSelect != nil {
				s.Config.OnSelect(s.Config.Items[s.filtered[s.selected]].ID)
			}
			return EventConsumed
		}
	}
	return EventIgnored
}

func (s *SelectWidget) notifyChange() {
	if s.Config.OnChange != nil && s.selected >= 0 && s.selected < len(s.filtered) {
		s.Config.OnChange(s.Config.Items[s.filtered[s.selected]].ID)
	}
}
