package app

import (
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) appsOpenWithMenu() ui.ContextMenuItem {
	var items []ui.ContextMenuItem
	for _, app := range a.apps {
		if app.OpenWith {
			items = append(items, ui.ContextMenuItem{Label: appIcon(app, a.Settings.Appearance.Icons) + " " + app.Name, Command: "apps.openWith:" + app.Name})
		}
	}
	if len(items) > 0 {
		items = append(items, ui.MenuSep())
	}
	items = append(items, ui.ContextMenuItem{Label: i18n.Text("apps.add"), Command: "apps.addOpenWith"})
	return ui.ContextMenuItem{Label: i18n.Text("menu.openWith"), Submenu: items}
}

func (a *App) AddApp() { a.showAddApp(false) }

func (a *App) AddOpenWithApp() {
	a.ExplorerContextNode = nil
	a.showAddApp(true)
}

func (a *App) showAddApp(openWith bool) {
	if a.Root.HasOverlay() {
		return
	}
	name := widgets.NewInputWidget(widgets.InputConfig{})
	command := widgets.NewInputWidget(widgets.InputConfig{Placeholder: i18n.Text("apps.commandHint")})
	cwd := widgets.NewInputWidget(widgets.InputConfig{Placeholder: "{workspace}"})
	icon := widgets.NewInputWidget(widgets.InputConfig{})
	nerdIcon := widgets.NewInputWidget(widgets.InputConfig{})
	panel := widgets.NewCheckboxWidget(widgets.CheckboxConfig{Label: i18n.Text("apps.inPanel")})
	keepOpen := widgets.NewCheckboxWidget(widgets.CheckboxConfig{Label: i18n.Text("apps.keepOpen")})
	explorer := widgets.NewCheckboxWidget(widgets.CheckboxConfig{Label: i18n.Text("apps.openWith"), Checked: openWith})
	errorLabel := widgets.NewLabelWidget(widgets.LabelConfig{Style: term.StyleDanger})
	row := func(text string, input *widgets.InputWidget) widgets.Widget {
		label := widgets.NewLabelWidget(widgets.LabelConfig{Text: text})
		label.FixedWidth = 16
		r := widgets.NewHStackWidget(label, input)
		r.FixedHeight = 1
		return r
	}
	content := widgets.NewVStackWidget(
		row(i18n.Text("apps.name"), name), row(i18n.Text("apps.command"), command), row(i18n.Text("apps.cwd"), cwd),
		row(i18n.Text("apps.icon"), icon), row(i18n.Text("apps.nerdIcon"), nerdIcon),
		panel, keepOpen, explorer, errorLabel,
	)
	dialog := widgets.NewDialogWidget(66)
	dialog.Title = i18n.Text("apps.add")
	dialog.Borders = *a.Borders
	dialog.SetContent(content)
	dialog.OnDismiss = a.DismissDialog
	dialog.Buttons = []widgets.DialogButton{
		{Label: i18n.Text("general.cancelButton"), Handler: a.DismissDialog},
		{Label: i18n.Text("general.add"), Handler: func() {
			app := config.Application{Name: strings.TrimSpace(name.Text()), Command: strings.TrimSpace(command.Text()), Cwd: cwd.Text(), Icon: icon.Text(), NerdIcon: nerdIcon.Text(), OpenWith: explorer.Config.Checked, KeepOpen: keepOpen.Config.Checked}
			if app.Name == "" || app.Command == "" {
				errorLabel.Config.Text = i18n.Text("apps.required")
				return
			}
			if slices.ContainsFunc(a.apps, func(existing config.Application) bool { return existing.Name == app.Name }) {
				errorLabel.Config.Text = i18n.Text("apps.nameTaken")
				return
			}
			if panel.Config.Checked {
				app.Location = "panel"
			}
			s := *a.Settings
			s.Applications = append(slices.Clone(s.Applications), app)
			if err := config.SaveSettings(s); err != nil {
				errorLabel.Config.Text = i18n.Format("apps.saveFailed", i18n.Args{"error": err.Error()})
				return
			}
			a.ApplySettings(s)
			a.DismissDialog()
			a.Apps.SearchInput.SetText("")
			a.Apps.Filter("")
			a.ShowApps()
		}},
	}
	dialog.Build()
	a.ShowDialog(ui.NewWidgetAdapter(dialog))
}
