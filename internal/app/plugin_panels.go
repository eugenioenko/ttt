package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
)

// ShowPluginPanel makes plugin name's panel the active tab of area ("bottom"
// or "sidebar") and shows the area. Unless focus is set, it leaves alone an
// area whose active tab has keyboard focus and flags the panel's tab instead,
// reporting false. With focus it also focuses the panel and its autofocus
// input, unless a dialog is open; then it reports false.
func (a *App) ShowPluginPanel(name, area string, focus bool) (bool, error) {
	id := "plugin." + name
	tp, visible := &a.BottomPanel.TabbedPanel, a.ContentSplit.ShowBottom
	if area == "sidebar" {
		tp, visible = &a.Sidebar.TabbedPanel, a.Sidebar.Visible
	}
	if !tp.HasPanel(id) {
		return false, fmt.Errorf("plugin %s has no %s panel", name, area)
	}
	if !focus && visible && tp.ActivePanel != id && a.Root.Focused != nil && a.Root.Focused == tp.ActiveWidget() {
		tp.FlagPanel(id)
		return false, nil
	}
	tp.SetActivePanel(id)
	if area == "sidebar" {
		if !visible {
			a.ShowSidebar()
		}
	} else {
		a.ContentSplit.ShowBottom = true
	}
	if !focus {
		return true, nil
	}
	if a.Root.HasModalOverlay() {
		return false, nil
	}
	w := tp.ActiveWidget()
	a.Root.ClaimFocus(w)
	if adapter, ok := w.(*ui.WidgetAdapter); ok {
		if panel, ok := adapter.W.(*plugin.PluginPanelWidget); ok {
			panel.FocusAutofocusInput()
		}
	}
	return true, nil
}

func (a *App) pluginTab(name, key string) (*ui.EditorGroupWidget, int, *plugin.PluginPanelWidget) {
	for _, group := range a.editorGroups() {
		for i := range group.TabCount() {
			panel, ok := group.TabContent(i).(*plugin.PluginPanelWidget)
			if ok && panel.PluginName() == name && group.PluginTabKey(i) == key {
				return group, i, panel
			}
		}
	}
	return nil, -1, nil
}

func (a *App) switchToPluginTab(name, key string, focus bool) bool {
	group, i, panel := a.pluginTab(name, key)
	if i < 0 {
		return false
	}
	group.SwitchTab(i)
	if focus && !a.Root.HasModalOverlay() {
		a.Root.ClaimFocus(group)
		panel.FocusAutofocusInput()
	}
	return true
}

func (a *App) closePluginTab(name, key string) bool {
	group, i, _ := a.pluginTab(name, key)
	if i < 0 {
		return false
	}
	return group.ClosePluginTab(key)
}

// Panel images live in the terminal until released, so a plugin panel is closed whenever it leaves the UI.
func (a *App) RemovePluginPanels(name string) {
	id := "plugin." + name
	for _, tp := range []*ui.TabbedPanel{&a.Sidebar.TabbedPanel, &a.BottomPanel.TabbedPanel} {
		closePluginPanel(tp.PanelWidget(id))
		tp.RemovePanel(id)
	}
	a.closePluginDrawerAndTabs(name)
}

func (a *App) closePluginDrawerAndTabs(name string) {
	a.closePluginDialog(name, false)
	if a.pluginDrawerPanel != nil && a.pluginDrawerPanel.PluginName() == name {
		a.ClosePluginDrawer()
	}
	// File-handler tabs are left to SyncFileHandlerTabs, which falls back to the binary card.
	for _, group := range a.editorGroups() {
		var keys []string
		for i := range group.TabCount() {
			path, _ := group.TabInfo(i)
			if panel, ok := group.TabContent(i).(*plugin.PluginPanelWidget); ok && panel.PluginName() == name {
				if _, handled := a.fileHandlerTabs[path]; !handled {
					keys = append(keys, group.PluginTabKey(i))
				}
			}
		}
		for _, key := range keys {
			group.ClosePluginTab(key)
		}
	}
}

func (a *App) ApplyPluginPanelIcons() {
	if a.PluginManager == nil || a.Sidebar == nil {
		return
	}
	nerd := icons.ActiveIsNerd()
	for _, reg := range a.PluginManager.SidebarPanels {
		a.Sidebar.SetPanelIcon(reg.ID, reg.Icon.Resolve(nerd))
	}
}

func (a *App) syncPluginPanel(tp *ui.TabbedPanel, id, title string, current *plugin.PluginPanelWidget) {
	switch {
	case current == nil:
		if tp.HasPanel(id) {
			closePluginPanel(tp.PanelWidget(id))
			tp.RemovePanel(id)
		}
	case !tp.HasPanel(id):
		tp.AddPanel(id, title, ui.NewWidgetAdapter(current))
	default:
		a.replaceStalePluginPanel(tp, id, current)
	}
}

func (a *App) replaceStalePluginPanel(tp *ui.TabbedPanel, id string, current *plugin.PluginPanelWidget) {
	old, ok := tp.PanelWidget(id).(*ui.WidgetAdapter)
	if !ok || old.W == current {
		return
	}
	closePluginPanel(old)
	replacement := ui.NewWidgetAdapter(current)
	tp.ReplacePanelWidget(id, replacement)
	if a.Root.Focused == old {
		a.Root.SetFocus(replacement)
	}
}

func closePluginPanel(w ui.Widget) {
	if adapter, ok := w.(*ui.WidgetAdapter); ok {
		if panel, ok := adapter.W.(*plugin.PluginPanelWidget); ok {
			panel.Close()
		}
	}
}
