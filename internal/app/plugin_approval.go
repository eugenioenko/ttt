package app

import (
	"errors"
	"log/slog"
	"path/filepath"
	"slices"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) ShowPluginApprovalDialog(p *plugin.Plugin) {
	permEntries := p.Manifest.Permissions.DisplayEntries()

	var kvEntries []widgets.KeyValueEntry
	for _, e := range permEntries {
		kvEntries = append(kvEntries, widgets.KeyValueEntry{
			Key:   e.Name,
			Value: e.Value,
		})
	}
	for _, g := range p.Manifest.Grammars {
		kvEntries = append(kvEntries, widgets.KeyValueEntry{
			Key:   i18n.Text("plugins.grammar"),
			Value: grammarLabel(g),
		})
	}
	for _, language := range p.Manifest.Languages {
		kvEntries = append(kvEntries, widgets.KeyValueEntry{Key: i18n.Text("settings.displayLanguage"), Value: language.Name + " (" + language.Locale + ")"})
	}
	for _, theme := range p.Manifest.Themes {
		kvEntries = append(kvEntries, widgets.KeyValueEntry{Key: i18n.Text("plugins.theme"), Value: theme.ID()})
	}

	permissions := widgets.NewWrappingKeyValueListWidget(kvEntries)
	permissions.InvertStyles = true
	var content widgets.Widget = permissions
	maintained := a.isMaintainedRepo(p.Repo)
	if p.InstalledFromURL {
		maintained = plugin.IsMaintained(p.Repo, nil)
	}
	if !maintained {
		notice := widgets.NewParagraphWidget(i18n.Text("plugins.communityApprovalNotice"))
		notice.Style = term.StyleWarning
		notice.Box.PaddingTop = 1
		content = widgets.NewVStackWidget(permissions, notice)
	}

	dialog := widgets.NewDialogWidget(max(50, permissions.NaturalWidth()+4))
	dialog.Title = i18n.Format("plugins.requests", i18n.Args{"manifestTitle": p.Manifest.Title()})
	dialog.Borders = *a.Borders
	dialog.SetContent(content)
	dialog.Buttons = []widgets.DialogButton{
		{Label: i18n.Text("general.denyButton"), Handler: func() {
			a.DismissDialog()
			a.denyPluginApproval(p)
		}},
		{Label: i18n.Text("general.allowButton"), Handler: func() {
			a.DismissDialog()
			if err := a.PluginManager.ApproveAndLoad(p); err == nil {
				a.WirePlugin(p)
				a.installPluginGrammars(p)
			}
			a.updatePluginDetailButtons()
			if a.PluginsPanel != nil {
				a.PluginsPanel.Refresh()
			}
			a.showNextPluginApproval()
		}},
	}
	dialog.OnDismiss = func() {
		a.DismissDialog()
		a.denyPluginApproval(p)
	}
	dialog.Build()

	if a.approvalDialog != nil {
		a.restoreFocus(a.Root.RemoveOverlay(a.approvalDialog))
	}
	adapter := ui.NewWidgetAdapter(dialog)
	a.approvalDialog = adapter
	a.ShowDialog(adapter)
}

// ShowExecPathApproval asks the user whether plugin p may run the binary at
// path. The full path is shown, wrapped rather than clipped, since it is what
// the user approves.
func (a *App) ShowExecPathApproval(p *plugin.Plugin, path string, done func(granted bool)) error {
	if a.Root.HasModalOverlay() {
		return errors.New("another dialog is open")
	}
	content := widgets.NewParagraphWidget(i18n.Format("plugins.approveExecutable", i18n.Args{"path": path, "filename": filepath.Base(path)}))
	dialog := widgets.NewDialogWidget(max(50, textwidth.String(path)+6))
	dialog.Title = i18n.Format("plugins.requests", i18n.Args{"manifestTitle": p.Manifest.Title()})
	dialog.Borders = *a.Borders
	dialog.SetContent(content)
	deny := func() {
		a.DismissDialog()
		done(false)
	}
	dialog.Buttons = []widgets.DialogButton{
		{Label: i18n.Text("general.denyButton"), Handler: deny},
		{Label: i18n.Text("general.allowButton"), Handler: func() {
			a.DismissDialog()
			if err := a.PluginManager.GrantExecPath(p, path); err != nil {
				a.StatusError(i18n.Format("status.pluginPermissionNotSaved", i18n.Args{"error": err.Error()}))
				done(false)
				return
			}
			done(true)
		}},
	}
	dialog.OnDismiss = deny
	dialog.Build()
	a.ShowDialog(ui.NewWidgetAdapter(dialog))
	return nil
}

func (a *App) requestPluginApproval(p *plugin.Plugin) {
	a.PendingPluginApprovals = slices.DeleteFunc(a.PendingPluginApprovals, func(existing *plugin.Plugin) bool {
		return existing.Name == p.Name
	})
	a.PendingPluginApprovals = slices.Insert(a.PendingPluginApprovals, 0, p)
	a.ShowPluginApprovalDialog(p)
}

// denyPluginApproval permanently declines a pending plugin: it deletes a fresh
// ttt-installed clone (or disables an on-disk one) so it stops prompting,
// resets the install button, and advances to the next pending approval.
func (a *App) denyPluginApproval(p *plugin.Plugin) {
	if err := a.PluginManager.DenyPlugin(p); err != nil {
		slog.Error("deny plugin", "error", err)
	}
	a.resetPluginDetailButton(p.Manifest.Name)
	a.updatePluginDetailButtons()
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
	a.showNextPluginApproval()
}

func (a *App) showNextPluginApproval() {
	if len(a.PendingPluginApprovals) > 0 {
		a.PendingPluginApprovals = a.PendingPluginApprovals[1:]
	}
	if len(a.PendingPluginApprovals) > 0 {
		a.ShowPluginApprovalDialog(a.PendingPluginApprovals[0])
	}
}

func (a *App) ShowPendingPluginApprovals() {
	if len(a.PendingPluginApprovals) > 0 {
		a.ShowPluginApprovalDialog(a.PendingPluginApprovals[0])
	}
}
