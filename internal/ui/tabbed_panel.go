package ui

import (
	"slices"
	"sort"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type panelEntry struct {
	ID    string
	Title string
	W     Widget
}

type TabbedPanel struct {
	ActivePanel    string
	Tabs           *widgets.TabsWidget
	OnPanelChange  func(id string)
	OnPanelReorder func(ids []string)

	panels         []panelEntry
	preferredOrder []string
}

func NewTabbedPanel() TabbedPanel {
	return TabbedPanel{
		Tabs: widgets.NewTabsWidget(widgets.TabsConfig{}),
	}
}

func (tp *TabbedPanel) InitTabClick() {
	tp.Tabs.Config.OnTabClick = func(index int) {
		if index >= 0 && index < len(tp.panels) {
			tp.SetActivePanel(tp.panels[index].ID)
		}
	}
	tp.Tabs.Config.OnReorder = func(from, to int) {
		tp.MovePanel(from, to)
	}
}

func (tp *TabbedPanel) AddPanel(id, title string, w Widget) {
	tp.panels = append(tp.panels, panelEntry{ID: id, Title: title, W: w})
	if tp.ActivePanel == "" {
		tp.ActivePanel = id
	}
	tp.applyPreferredOrder()
	tp.syncTabs()
}

func (tp *TabbedPanel) SetPanelOrder(order []string) {
	tp.preferredOrder = tp.normalizedPanelOrder(order)
	tp.applyPreferredOrder()
	tp.syncTabs()
}

func (tp *TabbedPanel) normalizedPanelOrder(order []string) []string {
	seen := make(map[string]bool, len(order))
	normalized := make([]string, 0, len(order))
	for _, id := range order {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		normalized = append(normalized, id)
	}
	return normalized
}

func (tp *TabbedPanel) applyPreferredOrder() {
	if len(tp.preferredOrder) == 0 || len(tp.panels) < 2 {
		return
	}
	rank := make(map[string]int, len(tp.preferredOrder))
	for i, id := range tp.preferredOrder {
		if _, exists := rank[id]; !exists {
			rank[id] = i
		}
	}
	sort.SliceStable(tp.panels, func(i, j int) bool {
		ri, iKnown := rank[tp.panels[i].ID]
		rj, jKnown := rank[tp.panels[j].ID]
		switch {
		case iKnown && jKnown:
			return ri < rj
		case iKnown:
			return true
		case jKnown:
			return false
		default:
			return false
		}
	})
}

func (tp *TabbedPanel) MovePanel(from, to int) bool {
	if from < 0 || from >= len(tp.panels) || to < 0 || to >= len(tp.panels) || from == to {
		return false
	}
	panel := tp.panels[from]
	tp.panels = append(tp.panels[:from], tp.panels[from+1:]...)
	tp.panels = slices.Insert(tp.panels, to, panel)
	tp.preferredOrder = tp.mergePreferredOrder()
	tp.syncTabs()
	if tp.OnPanelReorder != nil {
		tp.OnPanelReorder(slices.Clone(tp.preferredOrder))
	}
	return true
}

func (tp *TabbedPanel) mergePreferredOrder() []string {
	current := tp.PanelIDs()
	known := make(map[string]bool, len(current))
	for _, id := range current {
		known[id] = true
	}
	merged := make([]string, 0, len(current)+len(tp.preferredOrder))
	next := 0
	for _, id := range tp.preferredOrder {
		if known[id] {
			merged = append(merged, current[next])
			next++
		} else {
			merged = append(merged, id)
		}
	}
	merged = append(merged, current[next:]...)
	return merged
}

func (tp *TabbedPanel) ActivePanelIndex() int {
	for i, panel := range tp.panels {
		if panel.ID == tp.ActivePanel {
			return i
		}
	}
	return -1
}

func (tp *TabbedPanel) CanMoveActivePanel(dir int) bool {
	idx := tp.ActivePanelIndex()
	return idx >= 0 && idx+dir >= 0 && idx+dir < len(tp.panels)
}

func (tp *TabbedPanel) MoveActivePanel(dir int) bool {
	idx := tp.ActivePanelIndex()
	if idx < 0 {
		return false
	}
	return tp.MovePanel(idx, idx+dir)
}

func (tp *TabbedPanel) RemovePanel(id string) {
	for i, p := range tp.panels {
		if p.ID == id {
			tp.panels = append(tp.panels[:i], tp.panels[i+1:]...)
			if tp.ActivePanel == id {
				if len(tp.panels) > 0 {
					idx := i
					if idx >= len(tp.panels) {
						idx = len(tp.panels) - 1
					}
					tp.ActivePanel = tp.panels[idx].ID
				} else {
					tp.ActivePanel = ""
				}
			}
			tp.syncTabs()
			return
		}
	}
}

func (tp *TabbedPanel) SetActivePanel(id string) {
	for _, p := range tp.panels {
		if p.ID == id {
			tp.ActivePanel = id
			tp.syncTabs()
			if tp.OnPanelChange != nil {
				tp.OnPanelChange(id)
			}
			return
		}
	}
}

func (tp *TabbedPanel) ActiveWidget() Widget {
	for _, p := range tp.panels {
		if p.ID == tp.ActivePanel {
			return p.W
		}
	}
	return nil
}

func (tp *TabbedPanel) PanelCount() int {
	return len(tp.panels)
}

func (tp *TabbedPanel) NextPanel() {
	if len(tp.panels) <= 1 {
		return
	}
	for i, p := range tp.panels {
		if p.ID == tp.ActivePanel {
			next := (i + 1) % len(tp.panels)
			tp.SetActivePanel(tp.panels[next].ID)
			return
		}
	}
}

func (tp *TabbedPanel) PrevPanel() {
	if len(tp.panels) <= 1 {
		return
	}
	for i, p := range tp.panels {
		if p.ID == tp.ActivePanel {
			prev := i - 1
			if prev < 0 {
				prev = len(tp.panels) - 1
			}
			tp.SetActivePanel(tp.panels[prev].ID)
			return
		}
	}
}

func (tp *TabbedPanel) HasPanel(id string) bool {
	for _, p := range tp.panels {
		if p.ID == id {
			return true
		}
	}
	return false
}

func (tp *TabbedPanel) PanelIDs() []string {
	ids := make([]string, len(tp.panels))
	for i, p := range tp.panels {
		ids[i] = p.ID
	}
	return ids
}

type PanelInfo struct {
	ID    string
	Title string
}

func (tp *TabbedPanel) PanelEntries() []PanelInfo {
	entries := make([]PanelInfo, len(tp.panels))
	for i, p := range tp.panels {
		entries[i] = PanelInfo{ID: p.ID, Title: p.Title}
	}
	return entries
}

func (tp *TabbedPanel) SetPanelDirty(id string, dirty bool) {
	tp.Tabs.SetDirty(id, dirty)
}

func (tp *TabbedPanel) HiddenTabs() ([]string, []string) {
	var ids, titles []string
	for _, idx := range tp.Tabs.HiddenTabs() {
		if idx >= 0 && idx < len(tp.panels) {
			ids = append(ids, tp.panels[idx].ID)
			titles = append(titles, tp.panels[idx].Title)
		}
	}
	return ids, titles
}

func (tp *TabbedPanel) syncTabs() {
	dirty := make(map[string]bool)
	for _, item := range tp.Tabs.Config.Items {
		if item.Dirty {
			dirty[item.ID] = true
		}
	}
	items := make([]widgets.TabItem, len(tp.panels))
	for i, p := range tp.panels {
		items[i] = widgets.TabItem{
			ID:     p.ID,
			Label:  p.Title,
			Active: p.ID == tp.ActivePanel,
			Dirty:  dirty[p.ID],
		}
	}
	tp.Tabs.Config.Items = items
}

func (tp *TabbedPanel) RenderTabs(surface Surface, r Rect) {
	tp.Tabs.SetRect(Rect{X: r.X, Y: r.Y, W: r.W, H: 1})
	tp.Tabs.Render(surface.Sub(Rect{X: 0, Y: 0, W: r.W, H: 1}))
}

func (tp *TabbedPanel) RenderDivider(surface Surface, y, w int, borders *term.BorderSet) {
	horizontal := '─'
	if borders != nil {
		horizontal = borders.Horizontal
	}
	for x := 0; x < w; x++ {
		surface.SetCell(x, y, term.Cell{Ch: horizontal, Style: term.StyleBorder})
	}
}
