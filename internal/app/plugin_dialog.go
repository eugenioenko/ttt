package app

import (
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
)

type pluginDialogState struct {
	owner    string
	widget   *ui.WidgetAdapter
	panel    *plugin.PluginPanelWidget
	previous ui.Widget
	onClose  func()
}

func (a *App) wirePluginDialogs(p *plugin.Plugin) {
	p.OpenDialog = func(dialog *plugin.PluginDialogWidget, onClose func()) bool {
		if a.Root.HasOverlay() {
			return false
		}
		dialog.Borders = *a.Borders
		adapter := ui.NewWidgetAdapter(dialog)
		a.pluginDialog = &pluginDialogState{p.Name, adapter, dialog.Panel, a.Root.Focused, onClose}
		dialog.OnDismiss = func() { a.closePluginDialog(p.Name, true) }
		a.ShowDialog(adapter)
		dialog.Panel.FocusAutofocusInput()
		return true
	}
	p.CloseDialog = func() { a.closePluginDialog(p.Name, true) }
}

func (a *App) closePluginDialog(owner string, notify bool) {
	dialog := a.pluginDialog
	if dialog == nil || dialog.owner != owner {
		return
	}
	a.pluginDialog = nil
	a.Root.RemoveOverlay(dialog.widget)
	dialog.panel.Close()
	if a.Root.Focused == dialog.widget {
		if notify && dialog.previous != nil {
			a.Root.SetFocus(dialog.previous)
		} else {
			a.FocusEditor()
		}
	}
	if notify && dialog.onClose != nil {
		dialog.onClose()
	}
}
