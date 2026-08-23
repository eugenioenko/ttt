package ui

import (
	"log/slog"
	"path/filepath"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type tabSpan struct {
	start, end int
	label      string
	active     bool
}

type Tab struct {
	ID       string
	Name     string
	Dirty    bool
	Active   bool
	Closable bool
	Pinned   bool
}

type TabDragAutoScrollTick struct {
	Generation uint64
}

type TabBarWidget struct {
	BaseWidget
	Tabs                      []Tab
	Borders                   *term.BorderSet
	ScrollOffset              int
	MoreButton                *MoreButtonWidget
	OnTabClick                func(index int)
	OnTabClose                func(index int)
	OnTabUnpin                func(index int)
	OnTabReorder              func(from, to int)
	NormalizeDropTarget       func(from, to int) int
	PostDragAutoScrollTick    func(generation uint64)
	PointerInteractionValid   func() bool
	OnTabRightClick           func(index, screenX, screenY int)
	OnPrevTab                 func()
	OnNextTab                 func()
	OnDoubleClick             func()
	tabSpans                  []tabSpan
	renderArrowW              int // arrow-gutter width from the last Render, reused by HandleEvent
	renderInnerRight          int // right edge of the tab zone from the last Render, reused by HandleEvent
	hasOverflowLeft           bool
	hasOverflowRight          bool
	totalTabWidth             int
	closeDownX                int // screen X where mouse-down hit a close button, -1 if none
	closeDownY                int
	wasPressed                bool
	lastSeenBtn               tcell.ButtonMask
	lastClickTime             int64
	drag                      widgets.TabDragState
	dragPointerX              int
	autoScrollTimer           *time.Timer
	autoScrollGeneration      uint64
	autoScrollDirection       int
	dragAutoScrollDelay       time.Duration
	pointerCaptureInvalidated func()
}

func NewTabBarWidget() *TabBarWidget {
	return &TabBarWidget{closeDownX: -1}
}

func (t *TabBarWidget) SetTabs(tabs []Tab) {
	t.Tabs = tabs
	t.validateGestureSource()
}

func (t *TabBarWidget) tabLabel(tab Tab) string {
	name := filepath.Base(tab.Name)
	label := " "
	if tab.Dirty {
		label += "● "
	}
	label += name
	if tab.Active && tab.Closable {
		if tab.Pinned {
			label += " ♦"
		} else {
			label += " x"
		}
	}
	label += " "
	return label
}

// drawTabLabel draws a tab label from x, advancing by each rune's display width
// and clipping to the scrollable inner zone. The dirty marker keeps its own
// style so it stays visible against the tab background.
func drawTabLabel(surface Surface, x, innerLeft, innerRight int, label string, dirty bool, pinned bool, style term.Style) {
	for _, ch := range label {
		chStyle := style
		if dirty && ch == '●' {
			chStyle = term.StyleWarning
		}
		if pinned && ch == '♦' {
			chStyle = term.StyleMuted
		}
		w := textwidth.Rune(ch)
		if x >= innerLeft && x+w <= innerRight {
			surface.SetCell(x, 1, term.Cell{Ch: ch, Style: chStyle})
		}
		x += w
	}
}

func (t *TabBarWidget) Render(surface Surface) {
	t.validateGestureSource()
	w, h := surface.Size()
	if w <= 0 || h < 3 {
		t.ClearRenderedGeometry()
		return
	}

	b := term.SingleBorderSet()
	if t.Borders != nil {
		b = *t.Borders
	}
	bs := term.StyleBorder

	// Compute spans: active tab needs 2 extra cols for │ │ side borders
	spans := make([]tabSpan, len(t.Tabs))
	pos := 0
	activeIdx := -1
	for i, tab := range t.Tabs {
		label := t.tabLabel(tab)
		labelW := textwidth.String(label)
		spanW := labelW
		if tab.Active {
			spanW += 2
		}
		spans[i] = tabSpan{start: pos, end: pos + spanW, label: label, active: tab.Active}
		if tab.Active {
			activeIdx = i
		}
		pos += spanW
	}
	t.tabSpans = spans

	t.totalTabWidth = pos

	// Overflow is measured against the space left after the ⋮ MoreButton, so the
	// arrow gutter is reserved before tabs can overlap the chevron. (issue #354)
	moreW := 0
	if t.MoreButton != nil && w >= 5 {
		moreW = 4
	}
	hasOverflow := pos > w-moreW
	arrowW := 0
	if hasOverflow {
		arrowW = 3 // " ◀ " or " ▶ "
	}
	innerLeft := arrowW
	t.renderArrowW = arrowW
	innerRight := w - moreW - arrowW
	t.renderInnerRight = innerRight
	if innerRight <= innerLeft {
		t.ClearRenderedGeometry()
		return
	}
	innerW := innerRight - innerLeft

	// Scroll to keep active tab visible within the inner zone
	if activeIdx >= 0 && !t.drag.Active() {
		s := spans[activeIdx]
		if s.end-t.ScrollOffset > innerW {
			t.ScrollOffset = s.end - innerW
		}
		if s.start < t.ScrollOffset {
			t.ScrollOffset = s.start
		}
	}
	// Never scroll past the last tab, so closing tabs can't strand the view on
	// the final tab with empty space to its right.
	if maxScroll := pos - innerW; t.ScrollOffset > maxScroll {
		t.ScrollOffset = maxScroll
	}
	if t.ScrollOffset < 0 {
		t.ScrollOffset = 0
	}

	t.hasOverflowLeft = t.ScrollOffset > 0
	t.hasOverflowRight = pos-t.ScrollOffset > innerW

	// Row 0: top of active tab ┌───┐, spaces elsewhere
	for x := 0; x < w; x++ {
		surface.SetCell(x, 0, term.Cell{Ch: ' '})
	}
	if activeIdx >= 0 {
		s := spans[activeIdx]
		sx := s.start - t.ScrollOffset + innerLeft
		ex := s.end - t.ScrollOffset + innerLeft
		if sx >= innerLeft && sx < innerRight {
			surface.SetCell(sx, 0, term.Cell{Ch: b.TopLeft, Style: bs})
		}
		for x := sx + 1; x < ex-1; x++ {
			if x >= innerLeft && x < innerRight {
				surface.SetCell(x, 0, term.Cell{Ch: b.Horizontal, Style: bs})
			}
		}
		if ex-1 > sx && ex-1 >= innerLeft && ex-1 < innerRight {
			surface.SetCell(ex-1, 0, term.Cell{Ch: b.TopRight, Style: bs})
		}
	}

	// Row 1: tab labels, active tab has │ sides
	for x := 0; x < w; x++ {
		surface.SetCell(x, 1, term.Cell{Ch: ' '})
	}
	for i, s := range spans {
		sx := s.start - t.ScrollOffset + innerLeft
		ex := s.end - t.ScrollOffset + innerLeft
		dirty := t.Tabs[i].Dirty
		pinned := t.Tabs[i].Pinned
		if s.active {
			if sx >= innerLeft && sx < innerRight {
				surface.SetCell(sx, 1, term.Cell{Ch: b.Vertical, Style: bs})
			}
			drawTabLabel(surface, sx+1, innerLeft, innerRight, s.label, dirty, pinned, term.StyleActiveTab)
			if ex-1 >= innerLeft && ex-1 < innerRight {
				surface.SetCell(ex-1, 1, term.Cell{Ch: b.Vertical, Style: bs})
			}
		} else {
			drawTabLabel(surface, sx, innerLeft, innerRight, s.label, dirty, pinned, term.StyleInactiveTab)
		}
	}

	// Row 2: baseline ─┘  └─, horizontal line with gap for active tab
	for x := 0; x < w; x++ {
		surface.SetCell(x, 2, term.Cell{Ch: b.Horizontal, Style: bs})
	}
	if activeIdx >= 0 {
		s := spans[activeIdx]
		sx := s.start - t.ScrollOffset + innerLeft
		ex := s.end - t.ScrollOffset + innerLeft
		if sx >= innerLeft && sx < innerRight {
			surface.SetCell(sx, 2, term.Cell{Ch: b.BottomRight, Style: bs})
		}
		for x := sx + 1; x < ex-1; x++ {
			if x >= innerLeft && x < innerRight {
				surface.SetCell(x, 2, term.Cell{Ch: ' '})
			}
		}
		if ex-1 > sx && ex-1 >= innerLeft && ex-1 < innerRight {
			surface.SetCell(ex-1, 2, term.Cell{Ch: b.BottomLeft, Style: bs})
		}
	}

	// Arrow zones: " ◀ " on left, " ▶ " on right
	if t.hasOverflowLeft {
		surface.SetCell(1, 1, term.Cell{Ch: '◀', Style: term.StyleMuted})
	}
	if t.hasOverflowRight {
		surface.SetCell(innerRight+1, 1, term.Cell{Ch: '▶', Style: term.StyleMuted})
	}

	if t.MoreButton != nil && w >= 5 {
		r := t.GetRect()
		t.MoreButton.SetRect(Rect{X: r.X + w - 4, Y: r.Y + 1, W: 3, H: 1})
		moreSurface := surface.Sub(Rect{X: w - 4, Y: 1, W: 3, H: 1})
		t.MoreButton.Render(moreSurface)
	}

	if x := t.dropIndicatorX(); x >= innerLeft && x < innerRight {
		for y := 0; y < 3; y++ {
			surface.SetCell(x, y, term.Cell{Ch: '│', Style: term.StyleBorderActive})
		}
	}
}

func (t *TabBarWidget) HandleEvent(ev tcell.Event) EventResult {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}
	r := t.GetRect()
	mx, my := mev.Position()
	btn := mev.Buttons()

	prevBtn := t.lastSeenBtn
	t.lastSeenBtn = btn

	slog.Debug("tabBar", "mx", mx, "my", my, "btn", btn, "rect", r, "hasMore", t.MoreButton != nil)

	if btn == tcell.ButtonNone && t.drag.Active() {
		t.wasPressed = false
		t.cancelAutoScroll()
		if !t.validateGestureSource() {
			return EventIgnored
		}
		from, to, dragged := t.drag.End()
		if dragged && from != to && t.OnTabReorder != nil {
			t.OnTabReorder(from, to)
		}
		if dragged {
			return EventConsumed
		}
		return EventIgnored
	}
	if t.drag.Active() && btn&tcell.Button1 != 0 {
		if !t.validateGestureSource() {
			return EventIgnored
		}
		if t.drag.Update(mx, t.effectiveDropTarget(t.dropTargetAt(mx))) {
			t.dragPointerX = mx
			t.scrollDragOnce(mx)
			t.drag.Update(mx, t.effectiveDropTarget(t.dropTargetAt(mx)))
			t.updateAutoScrollZone(mx)
			t.closeDownX = -1
			return EventCaptured
		}
		return EventConsumed
	}

	if t.MoreButton != nil {
		if t.MoreButton.HandleEvent(ev) == EventConsumed {
			return EventConsumed
		}
	}

	if my < r.Y || my >= r.Y+r.H || mx < r.X || mx >= r.X+r.W {
		slog.Debug("tabBar", "action", "outOfBounds")
		return EventIgnored
	}

	// Mouse wheel on tab bar switches tabs
	if btn&tcell.WheelUp != 0 && t.OnPrevTab != nil {
		t.OnPrevTab()
		return EventConsumed
	}
	if btn&tcell.WheelDown != 0 && t.OnNextTab != nil {
		t.OnNextTab()
		return EventConsumed
	}

	// Reuse Render's gutter width so click hit-tests line up with the screen.
	arrowW := t.renderArrowW

	if btn&tcell.Button2 != 0 && t.OnTabRightClick != nil {
		localX := mx - r.X - arrowW + t.ScrollOffset
		for i, s := range t.tabSpans {
			if localX >= s.start && localX < s.end {
				t.OnTabRightClick(i, mx, my)
				return EventConsumed
			}
		}
	}

	// Mouse release: only close if released at the exact same screen position as mouse-down
	if btn == tcell.ButtonNone {
		t.wasPressed = false
		if t.closeDownX >= 0 && mx == t.closeDownX && my == t.closeDownY {
			t.closeDownX = -1
			localX := mx - r.X - arrowW + t.ScrollOffset
			for i, s := range t.tabSpans {
				if s.active && localX == s.end-3 {
					if t.Tabs[i].Pinned && t.OnTabUnpin != nil {
						t.OnTabUnpin(i)
					} else if t.OnTabClose != nil {
						t.OnTabClose(i)
					}
					return EventConsumed
				}
			}
		}
		t.closeDownX = -1
		return EventIgnored
	}

	if btn&tcell.Button1 == 0 {
		return EventIgnored
	}

	freshClick := !t.wasPressed && prevBtn&tcell.Button1 == 0
	t.wasPressed = true
	if !freshClick {
		return EventConsumed
	}

	// Clicks in the reserved ◀/▶ arrow columns are consumed here so they never
	// fall through to the empty-space double-click handler (which would spawn a
	// tab — the "jump to the other side" bug when clicking a hidden chevron).
	// Scroll only when there is something hidden in that direction; the overflow
	// flag is set only when the active tab isn't already at that end, so it can't
	// wrap.
	if arrowW > 0 && mx >= r.X && mx < r.X+arrowW {
		if t.hasOverflowLeft && t.OnPrevTab != nil {
			t.OnPrevTab()
		}
		return EventConsumed
	}
	rightZoneStart := r.X + t.renderInnerRight
	if arrowW > 0 && mx >= rightZoneStart && mx < rightZoneStart+arrowW {
		if t.hasOverflowRight && t.OnNextTab != nil {
			t.OnNextTab()
		}
		return EventConsumed
	}

	localX := mx - r.X - arrowW + t.ScrollOffset
	for i, s := range t.tabSpans {
		if localX >= s.start && localX < s.end {
			if s.active && (t.OnTabClose != nil || t.OnTabUnpin != nil) {
				closeX := s.end - 3
				if localX == closeX {
					t.closeDownX = mx
					t.closeDownY = my
					return EventConsumed
				}
			}
			if t.OnTabClick != nil {
				t.OnTabClick(i)
			}
			if t.OnTabReorder != nil {
				t.drag.Begin(i, tabIdentity(t.Tabs[i]), mx)
				return EventCaptured
			}
			return EventConsumed
		}
	}

	now := time.Now().UnixMilli()
	if now-t.lastClickTime < DoubleClickMs && t.OnDoubleClick != nil {
		t.OnDoubleClick()
		t.lastClickTime = 0
	} else {
		t.lastClickTime = now
	}
	return EventConsumed
}

func (t *TabBarWidget) effectiveDropTarget(target int) int {
	if target < 0 || !t.drag.Active() {
		return target
	}
	if t.NormalizeDropTarget != nil {
		return t.NormalizeDropTarget(t.drag.From(), target)
	}
	return target
}

func (t *TabBarWidget) CancelPointerCapture() bool {
	t.cancelAutoScroll()
	active := t.drag.Active()
	if active {
		t.drag.Cancel()
	}
	t.wasPressed = false
	t.lastSeenBtn = 0
	t.closeDownX = -1
	if active && t.pointerCaptureInvalidated != nil {
		t.pointerCaptureInvalidated()
	}
	return active
}

func (t *TabBarWidget) SetPointerCaptureInvalidated(invalidated func()) {
	t.pointerCaptureInvalidated = invalidated
}

func (t *TabBarWidget) OwnsPointerCapture() bool {
	return t.validateGestureSource()
}

func (t *TabBarWidget) ClearRenderedGeometry() {
	t.tabSpans = nil
	t.renderArrowW = 0
	t.renderInnerRight = 0
	t.hasOverflowLeft = false
	t.hasOverflowRight = false
	t.totalTabWidth = 0
	t.CancelPointerCapture()
}

func (t *TabBarWidget) InvalidatePointerInteraction() bool {
	active := t.drag.Active()
	t.ClearRenderedGeometry()
	return active
}

func (t *TabBarWidget) validateGestureSource() bool {
	if !t.drag.Active() {
		return false
	}
	if t.PointerInteractionValid != nil && !t.PointerInteractionValid() {
		t.CancelPointerCapture()
		return false
	}
	sourceID := t.drag.SourceID()
	for i := range t.Tabs {
		if tabIdentity(t.Tabs[i]) == sourceID {
			t.drag.SetSourceIndex(i)
			return true
		}
	}
	t.CancelPointerCapture()
	return false
}

func tabIdentity(tab Tab) string {
	if tab.ID != "" {
		return tab.ID
	}
	return tab.Name
}

func (t *TabBarWidget) dropTargetAt(screenX int) int {
	localX := screenX - t.GetRect().X - t.renderArrowW + t.ScrollOffset
	last := -1
	for i, span := range t.tabSpans {
		last = i
		if localX < span.start+(span.end-span.start)/2 {
			return i
		}
	}
	return last
}

func (t *TabBarWidget) dropIndicatorX() int {
	if !t.drag.Dragging() || t.drag.From() == t.drag.Target() {
		return -1
	}
	target := t.drag.Target()
	if target < 0 || target >= len(t.tabSpans) {
		return -1
	}
	span := t.tabSpans[target]
	logicalX := span.end - 1
	if target < t.drag.From() {
		logicalX = span.start
	}
	return logicalX - t.ScrollOffset + t.renderArrowW
}

func (t *TabBarWidget) dragScrollDirection(screenX int) int {
	if !t.drag.Dragging() || t.renderArrowW == 0 || len(t.tabSpans) == 0 {
		return 0
	}
	innerW := t.renderInnerRight - t.renderArrowW
	if innerW < 1 {
		return 0
	}
	maxScroll := t.totalTabWidth - innerW
	if maxScroll < 0 {
		maxScroll = 0
	}
	r := t.GetRect()
	if screenX <= r.X+t.renderArrowW {
		if t.ScrollOffset > 0 {
			return -1
		}
	} else if screenX >= r.X+t.renderInnerRight-1 && t.ScrollOffset < maxScroll {
		return 1
	}
	return 0
}

func (t *TabBarWidget) scrollDragOnce(screenX int) bool {
	direction := t.dragScrollDirection(screenX)
	if direction == 0 {
		return false
	}
	const scrollStep = 3
	t.ScrollOffset += direction * scrollStep
	innerW := t.renderInnerRight - t.renderArrowW
	maxScroll := t.totalTabWidth - innerW
	if t.ScrollOffset > maxScroll {
		t.ScrollOffset = maxScroll
	}
	if t.ScrollOffset < 0 {
		t.ScrollOffset = 0
	}
	return true
}

func (t *TabBarWidget) updateAutoScrollZone(screenX int) {
	direction := t.dragScrollDirection(screenX)
	if direction == 0 {
		t.cancelAutoScroll()
		return
	}
	if direction == t.autoScrollDirection && t.autoScrollTimer != nil {
		return
	}
	t.cancelAutoScroll()
	if t.PostDragAutoScrollTick == nil {
		return
	}
	t.autoScrollDirection = direction
	t.autoScrollGeneration++
	t.armAutoScrollTick(t.autoScrollGeneration)
}

func (t *TabBarWidget) armAutoScrollTick(generation uint64) {
	delay := t.dragAutoScrollDelay
	if delay <= 0 {
		delay = 75 * time.Millisecond
	}
	post := t.PostDragAutoScrollTick
	t.autoScrollTimer = time.AfterFunc(delay, func() {
		post(generation)
	})
}

func (t *TabBarWidget) cancelAutoScroll() {
	t.autoScrollGeneration++
	if t.autoScrollTimer != nil {
		t.autoScrollTimer.Stop()
		t.autoScrollTimer = nil
	}
	t.autoScrollDirection = 0
}

func (t *TabBarWidget) HandleDragAutoScrollTick(generation uint64) bool {
	if generation != t.autoScrollGeneration {
		return false
	}
	t.autoScrollTimer = nil
	if !t.validateGestureSource() || !t.drag.Dragging() || t.autoScrollDirection == 0 {
		t.cancelAutoScroll()
		return false
	}
	if !t.scrollDragOnce(t.dragPointerX) {
		t.cancelAutoScroll()
		return false
	}
	t.drag.Update(t.dragPointerX, t.effectiveDropTarget(t.dropTargetAt(t.dragPointerX)))
	if t.dragScrollDirection(t.dragPointerX) != t.autoScrollDirection {
		t.updateAutoScrollZone(t.dragPointerX)
	} else {
		t.armAutoScrollTick(generation)
	}
	return true
}
