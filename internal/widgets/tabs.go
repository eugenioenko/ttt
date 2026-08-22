package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type TabItem struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Active bool   `json:"-"`
	Dirty  bool   `json:"-"`
}

type TabAction struct {
	Icon    string
	OnClick func(screenX, screenY int)
}

type TabsConfig struct {
	Items                   []TabItem   `json:"items"`
	Actions                 []TabAction `json:"-"`
	Style                   term.Style  `json:"-"`
	Align                   string      `json:"align,omitempty"`
	Reorderable             bool        `json:"-"`
	PointerInteractionValid func() bool `json:"-"`
	OnTabClick              func(index int)
	OnOverflow              func(screenX, screenY int)
	OnReorder               func(from, to int)
}

type TabsWidget struct {
	BaseWidget
	Config                    TabsConfig
	tabSpans                  [][2]int
	overSpan                  [2]int
	actionSpans               [][2]int
	hiddenTabs                []int
	wasPressed                bool
	focused                   bool
	selected                  int
	drag                      TabDragState
	pointerCaptureInvalidated func()
}

func NewTabsWidget(config TabsConfig) *TabsWidget {
	return &TabsWidget{Config: config}
}

func (t *TabsWidget) SetItems(items []TabItem) {
	t.Config.Items = items
	t.validateGestureSource()
}

func (t *TabsWidget) Height() int { return 1 + t.BoxOverheadH() }
func (t *TabsWidget) Width() int  { return 0 }

func (t *TabsWidget) Focusable() bool { return true }
func (t *TabsWidget) SetFocused(f bool) {
	t.focused = f
	if f {
		t.selected = t.activeIndex()
	}
}
func (t *TabsWidget) IsFocused() bool { return t.focused }

func (t *TabsWidget) SetActive(id string) {
	for i := range t.Config.Items {
		t.Config.Items[i].Active = id == t.Config.Items[i].ID
	}
}

func (t *TabsWidget) SetDirty(id string, dirty bool) {
	for i := range t.Config.Items {
		if t.Config.Items[i].ID == id {
			t.Config.Items[i].Dirty = dirty
			return
		}
	}
}

func (t *TabsWidget) ActiveID() string {
	for _, item := range t.Config.Items {
		if item.Active {
			return item.ID
		}
	}
	return ""
}

func (t *TabsWidget) Render(surface Surface) {
	t.validateGestureSource()
	inner := t.RenderBox(surface)
	w, _ := inner.Size()
	if w <= 0 {
		t.ClearRenderedGeometry()
		return
	}

	for x := range w {
		inner.SetCell(x, 0, term.Cell{Ch: ' '})
	}

	actionsW := 0
	for _, a := range t.Config.Actions {
		actionsW += textwidth.String(a.Icon) + 2
	}

	overflowW := 3

	tabWidths := make([]int, len(t.Config.Items))
	total := 0
	for i, item := range t.Config.Items {
		tw := textwidth.String(item.Label) + 2
		if item.Dirty {
			tw += 2
		}
		tabWidths[i] = tw
		total += tw
	}

	hasOverflow := total > w-actionsW
	tabAreaW := w - actionsW
	if hasOverflow {
		tabAreaW -= overflowW
	}
	if tabAreaW <= 0 {
		t.ClearRenderedGeometry()
		return
	}

	t.tabSpans = make([][2]int, len(t.Config.Items))
	t.hiddenTabs = t.hiddenTabs[:0]

	activeIdx := t.activeIndex()

	if hasOverflow {
		// Chrome-like: active tab always gets priority.
		// Fill from left in order, but ensure the active tab is visible.
		// If a non-active tab would take the space needed for the active tab, skip it.
		remaining := tabAreaW
		if activeIdx >= 0 {
			remaining -= tabWidths[activeIdx]
		}
		visible := make([]bool, len(t.Config.Items))
		if activeIdx >= 0 {
			visible[activeIdx] = true
		}
		for i := range t.Config.Items {
			if i == activeIdx {
				continue
			}
			if tabWidths[i] <= remaining {
				visible[i] = true
				remaining -= tabWidths[i]
			}
		}
		for i := range t.Config.Items {
			if !visible[i] {
				t.hiddenTabs = append(t.hiddenTabs, i)
			}
		}

		x := 0
		for i, item := range t.Config.Items {
			if !visible[i] {
				continue
			}
			style := term.StyleInactiveTab
			if item.Active {
				style = term.StyleActiveTab
			}
			if t.focused && i == t.selected {
				style = term.StyleSelectedTab
			}
			startX := x
			inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
			x++
			if item.Dirty {
				inner.SetCell(x, 0, term.Cell{Ch: '●', Style: term.StyleWarning})
				x++
				inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
				x++
			}
			x = inner.DrawText(x, 0, item.Label, tabAreaW, style)
			if x < tabAreaW {
				inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
				x++
			}
			t.tabSpans[i] = [2]int{startX, x}
		}
	} else {
		x := 0
		if t.Config.Align == "center" {
			if total < tabAreaW {
				x = (tabAreaW - total) / 2
			}
		}
		for i, item := range t.Config.Items {
			style := term.StyleInactiveTab
			if item.Active {
				style = term.StyleActiveTab
			}
			if t.focused && i == t.selected {
				style = term.StyleSelectedTab
			}
			startX := x
			inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
			x++
			if item.Dirty {
				inner.SetCell(x, 0, term.Cell{Ch: '●', Style: term.StyleWarning})
				x++
				inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
				x++
			}
			x = inner.DrawText(x, 0, item.Label, tabAreaW, style)
			if x < tabAreaW {
				inner.SetCell(x, 0, term.Cell{Ch: ' ', Style: style})
				x++
			}
			t.tabSpans[i] = [2]int{startX, x}
		}
	}

	t.overSpan = [2]int{0, 0}
	if hasOverflow {
		ox := tabAreaW
		t.overSpan = [2]int{ox, ox + overflowW}
		style := term.StyleInactiveTab
		if t.focused && t.isHidden(t.selected) {
			style = term.StyleSelectedTab
		}
		inner.SetCell(ox, 0, term.Cell{Ch: ' ', Style: style})
		inner.SetCell(ox+1, 0, term.Cell{Ch: '»', Style: style})
		inner.SetCell(ox+2, 0, term.Cell{Ch: ' ', Style: style})
	}

	t.actionSpans = t.actionSpans[:0]
	ax := w - actionsW
	for _, action := range t.Config.Actions {
		startX := ax
		inner.SetCell(ax, 0, term.Cell{Ch: ' ', Style: term.StyleInactiveTab})
		ax++
		ax = inner.DrawText(ax, 0, action.Icon, w, term.StyleInactiveTab)
		inner.SetCell(ax, 0, term.Cell{Ch: ' ', Style: term.StyleInactiveTab})
		ax++
		t.actionSpans = append(t.actionSpans, [2]int{startX, ax})
	}

	if x := t.dropIndicatorX(); x >= 0 && x < w {
		inner.SetCell(x, 0, term.Cell{Ch: '│', Style: term.StyleBorderActive})
	}
}

func (t *TabsWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		return t.handleKey(tev)
	case *tcell.EventMouse:
		return t.handleMouse(tev)
	}
	return EventIgnored
}

func (t *TabsWidget) HiddenTabs() []int {
	return t.hiddenTabs
}

func (t *TabsWidget) isHidden(idx int) bool {
	for _, h := range t.hiddenTabs {
		if h == idx {
			return true
		}
	}
	return false
}

func (t *TabsWidget) activeIndex() int {
	for i, item := range t.Config.Items {
		if item.Active {
			return i
		}
	}
	return 0
}

func (t *TabsWidget) handleKey(ev *tcell.EventKey) EventResult {
	if !t.focused {
		return EventIgnored
	}
	n := len(t.Config.Items)
	if n == 0 {
		return EventIgnored
	}
	switch ev.Key() {
	case tcell.KeyLeft:
		t.selected--
		if t.selected < 0 {
			t.selected = n - 1
		}
		return EventConsumed
	case tcell.KeyRight:
		t.selected = (t.selected + 1) % n
		return EventConsumed
	case tcell.KeyEnter, tcell.KeyRune:
		if ev.Key() == tcell.KeyRune && term.KeyRune(ev) != ' ' {
			return EventIgnored
		}
		if t.isHidden(t.selected) {
			if t.Config.OnOverflow != nil {
				r := t.GetRect()
				t.Config.OnOverflow(r.X+t.overSpan[0], r.Y)
			}
		} else if t.Config.OnTabClick != nil {
			t.Config.OnTabClick(t.selected)
		}
		return EventConsumed
	}
	return EventIgnored
}

func (t *TabsWidget) handleMouse(mev *tcell.EventMouse) EventResult {
	pressed := mev.Buttons()&tcell.Button1 != 0
	mx, my := mev.Position()
	r := t.GetRect()
	lx := mx - r.X - t.Box.MarginLeft - t.Box.PaddingLeft

	if mev.Buttons() == tcell.ButtonNone {
		t.wasPressed = false
		if t.drag.Active() {
			if !t.validateGestureSource() {
				return EventIgnored
			}
			from, to, dragged := t.drag.End()
			if dragged && from != to && t.Config.OnReorder != nil {
				t.Config.OnReorder(from, to)
			}
			if dragged {
				return EventConsumed
			}
		}
		return EventIgnored
	}
	if t.drag.Active() && pressed {
		if !t.validateGestureSource() {
			return EventIgnored
		}
		if t.drag.Update(mx, t.dropTargetAt(lx)) {
			return EventCaptured
		}
		return EventConsumed
	}

	freshClick := pressed && !t.wasPressed
	t.wasPressed = pressed
	if !freshClick {
		return EventIgnored
	}
	if my < r.Y || my >= r.Y+r.H || mx < r.X || mx >= r.X+r.W {
		return EventIgnored
	}

	for i, span := range t.actionSpans {
		if lx >= span[0] && lx < span[1] {
			if i < len(t.Config.Actions) && t.Config.Actions[i].OnClick != nil {
				t.Config.Actions[i].OnClick(mx, my+1)
			}
			return EventConsumed
		}
	}

	if t.Config.OnOverflow != nil && t.overSpan[1] > 0 && lx >= t.overSpan[0] && lx < t.overSpan[1] {
		t.Config.OnOverflow(mx, my)
		return EventConsumed
	}

	for i, span := range t.tabSpans {
		if lx >= span[0] && lx < span[1] {
			t.selected = i
			if t.Config.OnTabClick != nil {
				t.Config.OnTabClick(i)
			}
			if t.Config.Reorderable && t.Config.OnReorder != nil {
				t.drag.Begin(i, tabItemIdentity(t.Config.Items[i]), mx)
				return EventCaptured
			}
			return EventConsumed
		}
	}
	return EventIgnored
}

func (t *TabsWidget) PointerGestureActive() bool {
	return t.validateGestureSource()
}

func (t *TabsWidget) OwnsPointerCapture() bool {
	return t.validateGestureSource()
}

func (t *TabsWidget) CancelPointerCapture() bool {
	active := t.drag.Active()
	if active {
		t.drag.Cancel()
	}
	t.wasPressed = false
	if active && t.pointerCaptureInvalidated != nil {
		t.pointerCaptureInvalidated()
	}
	return active
}

func (t *TabsWidget) SetPointerCaptureInvalidated(invalidated func()) {
	t.pointerCaptureInvalidated = invalidated
}

func (t *TabsWidget) ClearRenderedGeometry() {
	t.tabSpans = nil
	t.overSpan = [2]int{}
	t.actionSpans = nil
	t.hiddenTabs = nil
	t.CancelPointerCapture()
}

func (t *TabsWidget) InvalidatePointerInteraction() bool {
	active := t.drag.Active()
	t.ClearRenderedGeometry()
	return active
}

func (t *TabsWidget) validateGestureSource() bool {
	if !t.drag.Active() {
		return false
	}
	if t.Config.PointerInteractionValid != nil && !t.Config.PointerInteractionValid() {
		t.CancelPointerCapture()
		return false
	}
	sourceID := t.drag.SourceID()
	for i := range t.Config.Items {
		if tabItemIdentity(t.Config.Items[i]) == sourceID {
			t.drag.SetSourceIndex(i)
			return true
		}
	}
	t.CancelPointerCapture()
	return false
}

func tabItemIdentity(item TabItem) string {
	if item.ID != "" {
		return item.ID
	}
	return item.Label
}

func (t *TabsWidget) dropTargetAt(localX int) int {
	last := -1
	for i, span := range t.tabSpans {
		if span[1] <= span[0] {
			continue
		}
		last = i
		if localX < span[0]+(span[1]-span[0])/2 {
			return i
		}
	}
	return last
}

func (t *TabsWidget) dropIndicatorX() int {
	if !t.drag.Dragging() || t.drag.From() == t.drag.Target() {
		return -1
	}
	target := t.drag.Target()
	if target < 0 || target >= len(t.tabSpans) {
		return -1
	}
	span := t.tabSpans[target]
	if span[1] <= span[0] {
		return -1
	}
	if target < t.drag.From() {
		return span[0]
	}
	return span[1] - 1
}
