package app

import (
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) ShowDialog(w ui.Widget) {
	a.Root.PushOverlay(ui.Overlay{Widget: w, Modal: true})
	a.Root.SetFocus(w)
}

func (a *App) ShowFindBar(w ui.Widget) {
	a.Root.PushOverlay(ui.Overlay{Widget: w, Modal: false})
}

func (a *App) ShowDrawer(drawer *widgets.DrawerWidget) *ui.WidgetAdapter {
	adapter := ui.NewWidgetAdapter(drawer)
	a.Root.PushOverlay(ui.Overlay{Widget: adapter, Modal: true})
	a.Root.SetFocus(adapter)
	return adapter
}

// ClosePluginDrawer closes the plugin drawer without disturbing overlays stacked above it.
func (a *App) ClosePluginDrawer() {
	if a.pluginDrawer == nil {
		return
	}
	restore := a.Root.Focused == a.pluginDrawer
	prev := a.Root.RemoveOverlay(a.pluginDrawer)
	a.pluginDrawer = nil
	if a.pluginDrawerPanel != nil {
		a.pluginDrawerPanel.Close()
		a.pluginDrawerPanel = nil
	}
	if restore {
		a.restoreFocus(prev)
	}
}

func (a *App) DismissDialog() {
	if a.quickOpen != nil && a.Root.TopOverlayWidget() == a.quickOpen.palette {
		a.releaseTask(a.quickOpen.task)
		a.quickOpen = nil
	}
	a.approvalDialog = nil
	a.restoreFocus(a.Root.PopOverlay())
}

// restoreFocus returns focus to w after an overlay closes, falling back to the
// editor when w was hidden or removed while the overlay was open: focusing a
// hidden terminal would send keystrokes to a PTY the user cannot see.
func (a *App) restoreFocus(w ui.Widget) {
	if a.canRestoreFocus(w) {
		a.Root.SetFocus(w)
		return
	}
	a.FocusEditor()
}

func (a *App) canRestoreFocus(w ui.Widget) bool {
	if w == nil {
		return false
	}
	for _, o := range a.Root.Overlays {
		if o.Widget == w {
			return true
		}
	}
	if a.Sidebar.Visible && (w == a.Sidebar || w == a.Sidebar.ActiveWidget()) {
		return true
	}
	if !a.ContentSplit.ShowBottom || (w != a.BottomPanel && w != a.BottomPanel.ActiveWidget()) {
		return false
	}
	return w != a.TerminalPanel || len(a.Terminals) > 0
}

func (a *App) ShowInputDialog(title, placeholder, initial string, onSubmit func(string)) {
	a.ShowInputDialogEx(title, placeholder, initial, i18n.Text("general.save"), onSubmit)
}

func (a *App) ShowInputDialogEx(title, placeholder, initial, confirmLabel string, onSubmit func(string)) {
	submit := func(text string) {
		if text != "" {
			a.DismissDialog()
			onSubmit(text)
		}
	}
	input := widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: placeholder,
		OnSubmit:    submit,
	})
	input.SetText(initial)

	dialog := widgets.NewDialogWidget(50)
	dialog.Title = title
	dialog.Borders = *a.Borders
	dialog.SetContent(input)
	dialog.Buttons = []widgets.DialogButton{
		{Label: i18n.Text("general.cancelButton"), Handler: func() { a.DismissDialog() }},
		{Label: "&" + confirmLabel, Handler: func() {
			if input.Text() != "" {
				a.DismissDialog()
				onSubmit(input.Text())
			}
		}},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}

func (a *App) ShowInfoDialog(title string, entries []widgets.KeyValueEntry) {
	a.ShowInfoDialogEx(title, entries, false)
}

func (a *App) ShowInfoDialogEx(title string, entries []widgets.KeyValueEntry, invertStyles bool) {
	content := widgets.NewKeyValueListWidget(entries)
	content.InvertStyles = invertStyles

	dialog := widgets.NewDialogWidget(60)
	dialog.Title = title
	dialog.Borders = *a.Borders
	dialog.SetContent(content)
	dialog.Buttons = []widgets.DialogButton{
		{Label: i18n.Text("general.closeButton"), Handler: func() { a.DismissDialog() }},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}

func (a *App) ShowConfirmDialog(message string, buttons []string, callbacks []func()) {
	a.ShowConfirmDialogEx("", message, buttons, callbacks)
}

func (a *App) ShowConfirmDialogEx(title, message string, buttons []string, callbacks []func()) *widgets.DialogWidget {
	content := widgets.NewParagraphWidget(message)

	dialog := widgets.NewDialogWidget(50)
	dialog.Borders = *a.Borders
	dialog.SetContent(content)

	dialogButtons := make([]widgets.DialogButton, len(buttons))
	for i, label := range buttons {
		handler := callbacks[i]
		dialogButtons[i] = widgets.DialogButton{
			Label:   "&" + label,
			Handler: handler,
		}
	}
	dialog.Buttons = dialogButtons
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Title = title
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
	return dialog
}

func (a *App) ShowSelectDialog(title string, items []widgets.SelectItem, onSelect func(id string), onChange func(id string)) {
	a.showSelectDialog(title, items, "", onSelect, onChange)
}

func (a *App) showSelectDialog(title string, items []widgets.SelectItem, selected string, onSelect func(id string), onChange func(id string)) {
	sel := widgets.NewSelectWidget(widgets.SelectConfig{
		Items:       items,
		ShowDivider: true,
		OnSelect: func(id string) {
			a.DismissDialog()
			onSelect(id)
		},
		OnChange:  onChange,
		OnDismiss: func() { a.DismissDialog() },
	})
	if selected != "" {
		sel.SetSelectedID(selected)
	}

	dialog := widgets.NewDialogWidget(50)
	dialog.Title = title
	dialog.Borders = *a.Borders
	dialog.SetContent(sel)
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}
