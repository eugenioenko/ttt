package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type TreeNode struct {
	ID         string      `json:"id"`
	Label      string      `json:"label"`
	Icon       string      `json:"icon,omitempty"`
	IconStyle  term.Style  `json:"-"`
	Badge      string      `json:"badge,omitempty"`
	BadgeStyle term.Style  `json:"-"`
	Children   []*TreeNode `json:"children,omitempty"`
	Actions    []Action    `json:"actions,omitempty"`
	Muted      bool        `json:"-"`
	Expandable bool        `json:"-"`

	Expanded bool `json:"-"`
	depth    int
	parent   *TreeNode
}

type Action struct {
	Icon    string `json:"icon"`
	Command string `json:"command"`
}

type MenuEntry struct {
	Label     string `json:"label"`
	Command   string `json:"command"`
	Separator bool   `json:"separator,omitempty"`
}

type TreeConfig struct {
	Items          []*TreeNode `json:"items"`
	NodeMenu       []MenuEntry `json:"nodeMenu,omitempty"`
	MenuIcon       string      `json:"menuIcon,omitempty"`
	MenuIconPadded bool        `json:"menuIconPadded,omitempty"`
	Indent         int         `json:"indent,omitempty"`
	ActiveID       string      `json:"-"`
	EmptyText      string      `json:"emptyText,omitempty"`
	SelectOnClick  bool        `json:"-"`
	TruncateLeft   bool        `json:"truncateLeft,omitempty"` // truncate labels from the left (…tail) so the end stays visible

	OnCommand  func(command string, node *TreeNode)
	OnMenu     func(entries []MenuEntry, node *TreeNode, screenX, screenY int)
	OnExpand   func(node *TreeNode)
	OnSelect   func(node *TreeNode)
	OnKey      func(ev *tcell.EventKey, node *TreeNode) bool
	RenderItem func(surface Surface, node *TreeNode, idx, y, w int, selected bool)
}

type TreeWidget struct {
	BaseWidget
	Config   TreeConfig
	flatList []*TreeNode

	selected  int
	scrollTop int
	lastSel   int
	focused   bool

	scrollbar scrollbar
	contentX  int
	contentY  int
	contentW  int
}

func NewTreeWidget(cfg TreeConfig) *TreeWidget {
	if cfg.Indent == 0 {
		cfg.Indent = 2
	} else if cfg.Indent < 0 {
		cfg.Indent = 0
	}
	t := &TreeWidget{Config: cfg}
	t.flatten()
	return t
}

func (t *TreeWidget) Height() int { return 0 }
func (t *TreeWidget) Width() int  { return 0 }

// ContentHeight reports visible rows so scroll views can measure the tree.
func (t *TreeWidget) ContentHeight() int { return len(t.flatList) + t.BoxOverheadH() }

func (t *TreeWidget) Focusable() bool   { return true }
func (t *TreeWidget) SetFocused(f bool) { t.focused = f }
func (t *TreeWidget) IsFocused() bool   { return t.focused }

func (t *TreeWidget) Selected() *TreeNode {
	if t.selected >= 0 && t.selected < len(t.flatList) {
		return t.flatList[t.selected]
	}
	return nil
}

func (t *TreeWidget) SelectByID(id string) {
	for i, node := range t.flatList {
		if node.ID == id {
			t.selected = i
			return
		}
	}
}

func (t *TreeWidget) SetItems(items []*TreeNode) {
	t.Config.Items = items
	t.flatten()
	t.clampSelected()
}

func (t *TreeWidget) SetActiveID(id string) {
	t.Config.ActiveID = id
}

func (t *TreeWidget) Reload() {
	expanded := map[string]bool{}
	t.CollectExpanded(expanded)
	for _, root := range t.Config.Items {
		if root.Expanded && t.Config.OnExpand != nil {
			t.Config.OnExpand(root)
		}
	}
	t.RestoreExpanded(expanded)
	t.clampSelected()
}

func (t *TreeWidget) CollectExpanded(out map[string]bool) {
	t.collectExpanded(t.Config.Items, out)
}

func (t *TreeWidget) RestoreExpanded(expanded map[string]bool) {
	for _, root := range t.Config.Items {
		t.restoreExpanded(root, expanded, true)
	}
	t.flatten()
}

// RestoreExpandedSilent restores expansion without firing OnExpand — reconcile would loop otherwise.
func (t *TreeWidget) RestoreExpandedSilent(expanded map[string]bool) {
	for _, root := range t.Config.Items {
		t.restoreExpanded(root, expanded, false)
	}
	t.flatten()
}

func (t *TreeWidget) collectExpanded(nodes []*TreeNode, out map[string]bool) {
	for _, node := range nodes {
		if node.Expanded {
			out[node.ID] = true
			t.collectExpanded(node.Children, out)
		}
	}
}

func (t *TreeWidget) restoreExpanded(node *TreeNode, expanded map[string]bool, notify bool) {
	for _, child := range node.Children {
		if expanded[child.ID] && child.isExpandable() {
			child.Expanded = true
			if notify && t.Config.OnExpand != nil {
				t.Config.OnExpand(child)
			}
			t.restoreExpanded(child, expanded, notify)
		}
	}
}

func (t *TreeWidget) flatten() {
	t.flatList = nil
	for _, root := range t.Config.Items {
		t.flattenNode(root, 0)
	}
}

func (t *TreeWidget) flattenNode(node *TreeNode, depth int) {
	node.depth = depth
	t.flatList = append(t.flatList, node)
	if node.Expanded && len(node.Children) > 0 {
		for _, child := range node.Children {
			child.parent = node
			t.flattenNode(child, depth+1)
		}
	}
}

func (t *TreeWidget) clampSelected() {
	if t.selected >= len(t.flatList) {
		t.selected = len(t.flatList) - 1
	}
	if t.selected < 0 {
		t.selected = 0
	}
}

func (t *TreeWidget) ensureVisible(visibleH int) {
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

func (t *TreeWidget) Render(surface Surface) {
	surface = t.RenderBox(surface)
	w, h := surface.Size()
	surface.Fill(term.Cell{Ch: ' '})

	if h <= 0 || w <= 0 {
		return
	}

	ox := t.Box.MarginLeft + t.Box.PaddingLeft
	oy := t.Box.MarginTop + t.Box.PaddingTop
	if t.Box.BorderLeft {
		ox++
	}
	if t.Box.BorderTop {
		oy++
	}
	t.contentX = t.rect.X + ox
	t.contentY = t.rect.Y + oy

	if len(t.flatList) == 0 && t.Config.EmptyText != "" {
		x := 1
		for _, ch := range t.Config.EmptyText {
			if x >= w {
				break
			}
			surface.SetCell(x, 0, term.Cell{Ch: ch, Style: term.StyleDefault})
			x++
		}
		return
	}

	maxScroll := len(t.flatList) - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.scrollTop > maxScroll {
		t.scrollTop = maxScroll
	}

	t.ensureVisible(h)

	t.scrollbar.X = t.contentX + w - 1
	t.scrollbar.Y = t.contentY
	t.scrollbar.Height = h
	t.scrollbar.TotalItems = len(t.flatList)
	t.scrollbar.TopItem = t.scrollTop

	t.contentW = w
	if t.scrollbar.visible() {
		t.contentW = w - 1
	}
	for i := range h {
		idx := t.scrollTop + i
		if idx >= len(t.flatList) {
			break
		}
		node := t.flatList[idx]
		t.renderNode(surface, node, idx, i, t.contentW)
	}

	t.scrollbar.Render(surface, w-1, 0)
}

func (t *TreeWidget) menuIconWidth() int {
	if len(t.Config.NodeMenu) == 0 {
		return 0
	}
	label := t.Config.MenuIcon
	if label == "" {
		label = "⋮"
	}
	var box *BoxModel
	if t.Config.MenuIconPadded {
		box = &BoxModel{PaddingLeft: 1, PaddingRight: 1}
	} else {
		box = &BoxModel{PaddingLeft: 0, PaddingRight: 0}
	}
	dd := NewDropdownWidget(DropdownConfig{Label: label, Box: box})
	w := dd.Width()
	if !t.Config.MenuIconPadded {
		w++
	}
	return w
}

func (t *TreeWidget) rightSideWidth(node *TreeNode) int {
	rw := t.menuIconWidth()
	for i, action := range node.Actions {
		rw += textwidth.String(action.Icon)
		if i > 0 {
			rw++
		}
	}
	return rw
}

func (t *TreeWidget) renderNode(surface Surface, node *TreeNode, idx, y, w int) {
	if t.Config.RenderItem != nil {
		t.Config.RenderItem(surface, node, idx, y, w, idx == t.selected)
		return
	}

	style := term.StyleDefault
	if idx == t.selected && t.focused {
		style = term.StyleSidebarSelected
	} else if t.Config.ActiveID != "" && node.ID == t.Config.ActiveID {
		style = term.StyleSidebarSelected
	}

	for x := range w {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
	}

	maxX := w - 2 - t.rightSideWidth(node)

	x := node.depth * t.Config.Indent

	hasChildren := len(node.Children) > 0 || node.Expandable
	if hasChildren {
		chevron := '▶'
		if node.Expanded {
			chevron = '▼'
		}
		if x < w {
			surface.SetCell(x, y, term.Cell{Ch: chevron, Style: style})
		}
		x++
		if x < w {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		}
		x++
	}

	if node.Icon != "" {
		iconStyle := node.IconStyle
		if iconStyle == term.StyleDefault {
			iconStyle = style
		}
		if idx == t.selected {
			iconStyle = style
		}
		x = drawRunesClipped(surface, x, y, maxX, []rune(node.Icon), iconStyle)
		if x < maxX {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
			x++
		}
	}

	labelStyle := style
	if node.Muted && idx != t.selected {
		labelStyle = term.StyleMuted
	}
	labelRunes := []rune(node.Label)
	if t.Config.TruncateLeft {
		labelRunes = truncateRunesLeft(labelRunes, maxX-x)
	}
	x = drawRunesClipped(surface, x, y, maxX, labelRunes, labelStyle)

	if node.Badge != "" {
		badgeStyle := node.BadgeStyle
		if badgeStyle == term.StyleDefault {
			badgeStyle = term.StyleMuted
		}
		if idx == t.selected {
			badgeStyle = style
		}
		x++
		drawRunesClipped(surface, x, y, maxX, []rune(node.Badge), badgeStyle)
	}

	rightX := w - 2

	if len(t.Config.NodeMenu) > 0 {
		label := t.Config.MenuIcon
		if label == "" {
			if len(node.Children) > 0 {
				label = "⋮"
			} else {
				label = "⋯"
			}
		}
		var box *BoxModel
		if t.Config.MenuIconPadded {
			box = &BoxModel{PaddingLeft: 1, PaddingRight: 1}
		} else {
			box = &BoxModel{PaddingLeft: 0, PaddingRight: 0}
		}
		dd := NewDropdownWidget(DropdownConfig{Label: label, Style: style, Box: box})
		dw := dd.Width()
		ddX := rightX - dw + 1
		dd.SetRect(Rect{X: ddX, Y: y, W: dw, H: 1})
		ddSurface := surface.Sub(Rect{X: ddX, Y: y, W: dw, H: 1})
		dd.Render(ddSurface)
		rightX -= dw
		if !t.Config.MenuIconPadded {
			if rightX >= 0 && rightX < w {
				surface.SetCell(rightX, y, term.Cell{Ch: ' ', Style: style})
			}
			rightX--
		}
	}

	actionStyle := style
	if node.Muted && idx != t.selected {
		actionStyle = term.StyleMuted
	}
	for i := len(node.Actions) - 1; i >= 0; i-- {
		action := node.Actions[i]
		iconRunes := []rune(action.Icon)
		for j := len(iconRunes) - 1; j >= 0; j-- {
			cw := textwidth.Rune(iconRunes[j])
			// Icons are laid out right to left, so a fullwidth rune starts one
			// column further left than the cursor.
			startX := rightX - cw + 1
			if startX >= 0 && startX < w {
				surface.SetCell(startX, y, term.Cell{Ch: iconRunes[j], Style: actionStyle})
			}
			rightX -= cw
		}
		if i > 0 {
			if rightX >= 0 && rightX < w {
				surface.SetCell(rightX, y, term.Cell{Ch: ' ', Style: style})
			}
			rightX--
		}
	}
}

func (t *TreeWidget) HandleEvent(ev tcell.Event) EventResult {
	if newTop, consumed := t.scrollbar.HandleEvent(ev); consumed {
		t.scrollTop = newTop
		if t.scrollbar.isDragging() {
			return EventCaptured
		}
		return EventConsumed
	}

	prev := t.selected
	var result EventResult
	switch tev := ev.(type) {
	case *tcell.EventMouse:
		result = t.handleMouse(tev)
	case *tcell.EventKey:
		result = t.handleKey(tev)
	default:
		return EventIgnored
	}
	if t.selected != prev && t.Config.OnSelect != nil {
		t.Config.OnSelect(t.Selected())
	}
	return result
}

func (t *TreeWidget) handleMouse(ev *tcell.EventMouse) EventResult {
	btn := ev.Buttons()
	mx, my := ev.Position()
	r := t.rect
	if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
		return EventIgnored
	}

	if btn&tcell.WheelUp != 0 {
		t.scrollTop -= 3
		if t.scrollTop < 0 {
			t.scrollTop = 0
		}
		return EventConsumed
	}
	if btn&tcell.WheelDown != 0 {
		max := len(t.flatList) - r.H
		if max < 0 {
			max = 0
		}
		t.scrollTop += 3
		if t.scrollTop > max {
			t.scrollTop = max
		}
		return EventConsumed
	}

	idx := t.scrollTop + (my - t.contentY)
	if idx < 0 || idx >= len(t.flatList) {
		return EventIgnored
	}

	if btn&tcell.Button2 != 0 {
		t.selected = idx
		if t.Config.OnMenu != nil {
			t.Config.OnMenu(t.Config.NodeMenu, t.flatList[idx], mx, my)
		}
		return EventConsumed
	}

	if btn&tcell.Button1 != 0 {
		node := t.flatList[idx]
		t.selected = idx

		menuW := t.menuIconWidth()
		if menuW > 0 && mx >= t.contentX+t.contentW-menuW {
			if t.Config.OnMenu != nil {
				t.Config.OnMenu(t.Config.NodeMenu, node, mx, my)
			}
			return EventConsumed
		}

		rightX := t.contentX + t.contentW - 2 - t.menuIconWidth()
		for i := len(node.Actions) - 1; i >= 0; i-- {
			action := node.Actions[i]
			iconW := textwidth.String(action.Icon)
			actionX := rightX - iconW + 1
			if mx >= actionX && mx <= rightX {
				if t.Config.OnCommand != nil {
					t.Config.OnCommand(action.Command, node)
				}
				return EventConsumed
			}
			rightX = actionX - 1
			if i > 0 {
				rightX--
			}
		}

		if !t.Config.SelectOnClick {
			t.ActivateSelected()
		}
		return EventConsumed
	}

	return EventIgnored
}

func (t *TreeWidget) handleKey(ev *tcell.EventKey) EventResult {
	switch ev.Key() {
	case tcell.KeyUp:
		if t.selected > 0 {
			t.selected--
		}
		return EventConsumed
	case tcell.KeyDown:
		if t.selected < len(t.flatList)-1 {
			t.selected++
		}
		return EventConsumed
	case tcell.KeyLeft:
		t.collapseOrParent()
		return EventConsumed
	case tcell.KeyRight:
		t.expandOrChild()
		return EventConsumed
	case tcell.KeyEnter:
		if ev.Modifiers()&tcell.ModShift != 0 {
			if t.Config.OnMenu != nil && t.selected >= 0 && t.selected < len(t.flatList) {
				t.Config.OnMenu(t.Config.NodeMenu, t.flatList[t.selected], t.contentX, t.contentY+t.selected-t.scrollTop)
			}
			return EventConsumed
		}
		t.ActivateSelected()
		return EventConsumed
	case tcell.KeyRune:
		if t.Config.OnKey != nil && t.Config.OnKey(ev, t.Selected()) {
			return EventConsumed
		}
		switch term.KeyRune(ev) {
		case 'j':
			if t.selected < len(t.flatList)-1 {
				t.selected++
			}
			return EventConsumed
		case 'k':
			if t.selected > 0 {
				t.selected--
			}
			return EventConsumed
		case 'l':
			t.expandOrChild()
			return EventConsumed
		case 'h':
			t.collapseOrParent()
			return EventConsumed
		}
		if term.KeyRune(ev) == ' ' {
			t.ActivateSelected()
			return EventConsumed
		}
		if t.handleShortcutKey(term.KeyRune(ev)) == EventConsumed {
			return EventConsumed
		}
	}
	return EventIgnored
}

func (t *TreeWidget) handleShortcutKey(r rune) EventResult {
	node := t.Selected()
	if node == nil {
		return EventIgnored
	}
	for _, action := range node.Actions {
		iconRunes := []rune(action.Icon)
		if len(iconRunes) == 1 && iconRunes[0] == r {
			if t.Config.OnCommand != nil {
				t.Config.OnCommand(action.Command, node)
			}
			return EventConsumed
		}
	}
	return EventIgnored
}

func (n *TreeNode) isExpandable() bool {
	return len(n.Children) > 0 || n.Expandable
}

func (t *TreeWidget) FlatList() []*TreeNode  { return t.flatList }
func (t *TreeWidget) SelectedIndex() int     { return t.selected }
func (t *TreeWidget) SetSelectedIndex(i int) { t.selected = i; t.clampSelected() }
func (t *TreeWidget) ScrollTop() int         { return t.scrollTop }
func (t *TreeWidget) ItemCount() int         { return len(t.flatList) }

func (t *TreeWidget) ActivateSelected() {
	if t.selected < 0 || t.selected >= len(t.flatList) {
		return
	}
	node := t.flatList[t.selected]
	if node.isExpandable() {
		node.Expanded = !node.Expanded
		if node.Expanded && t.Config.OnExpand != nil {
			t.Config.OnExpand(node)
		}
		t.flatten()
	} else if t.Config.OnCommand != nil {
		t.Config.OnCommand("activate", node)
	}
}

func (t *TreeWidget) collapseOrParent() {
	if t.selected < 0 || t.selected >= len(t.flatList) {
		return
	}
	node := t.flatList[t.selected]
	if node.isExpandable() && node.Expanded {
		node.Expanded = false
		t.flatten()
		return
	}
	if node.parent != nil {
		for i, n := range t.flatList {
			if n == node.parent {
				t.selected = i
				return
			}
		}
	}
}

func (t *TreeWidget) expandOrChild() {
	if t.selected < 0 || t.selected >= len(t.flatList) {
		return
	}
	node := t.flatList[t.selected]
	if node.isExpandable() && !node.Expanded {
		node.Expanded = true
		if t.Config.OnExpand != nil {
			t.Config.OnExpand(node)
		}
		t.flatten()
		return
	}
	if node.Expanded && t.selected+1 < len(t.flatList) {
		t.selected++
	}
}
