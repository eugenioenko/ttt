package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func explorerHelpEntries() []widgets.KeyValueEntry {
	return []widgets.KeyValueEntry{
		{Key: i18n.Text("help.click"), Value: i18n.Text("help.openFileInPreview")},
		{Key: i18n.Text("help.doubleClick"), Value: i18n.Text("help.openFilePermanently")},
		{Key: "Enter", Value: i18n.Text("help.openFileOrToggleFolder")},
		{Key: "Space", Value: i18n.Text("help.openFileOrToggleFolder")},
		{Key: "Shift+Enter", Value: i18n.Text("help.openContextMenu")},
		{Key: "Menu*", Value: i18n.Text("help.openContextMenuTerminalDependent")},
		{Key: "r", Value: i18n.Text("help.refreshExplorer")},
		{Key: "Up / k", Value: i18n.Text("help.moveUp")},
		{Key: "Down / j", Value: i18n.Text("help.moveDown")},
		{Key: "Left / h", Value: i18n.Text("help.collapseFolder")},
		{Key: "Right / l", Value: i18n.Text("help.expandFolder")},
	}
}

func searchHelpEntries() []widgets.KeyValueEntry {
	return []widgets.KeyValueEntry{
		{Key: "Enter", Value: i18n.Text("help.activateSelectedResult")},
		{Key: "Up / k", Value: i18n.Text("help.moveUp")},
		{Key: "Down / j", Value: i18n.Text("help.moveDown")},
		{Key: "Tab", Value: i18n.Text("help.nextInputField")},
		{Key: "Shift+Tab", Value: i18n.Text("help.previousInputField")},
		{Key: "Alt+c", Value: i18n.Text("help.toggleCaseSensitivity")},
		{Key: "Alt+r", Value: i18n.Text("help.toggleRegexMode")},
	}
}

func changesHelpEntries() []widgets.KeyValueEntry {
	return []widgets.KeyValueEntry{
		{Key: "Space / s", Value: i18n.Text("help.toggleStageUnstageFile")},
		{Key: "a", Value: i18n.Text("help.stageAllFiles")},
		{Key: "u", Value: i18n.Text("help.unstageAllFiles")},
		{Key: "d", Value: i18n.Text("help.discardSelectedFile")},
		{Key: "D", Value: i18n.Text("help.discardAllFilesInGroup")},
		{Key: "r", Value: i18n.Text("help.refreshChanges")},
		{Key: "o / v", Value: i18n.Text("help.openFile")},
		{Key: "c", Value: i18n.Text("help.openCompactDiff")},
		{Key: "e", Value: i18n.Text("help.openExtendedDiff")},
		{Key: "Enter", Value: i18n.Text("help.openCompactDiff")},
		{Key: "Up / k", Value: i18n.Text("help.moveUp")},
		{Key: "Down / j", Value: i18n.Text("help.moveDown")},
	}
}

func (a *App) ShowPanelHelp(title string, entries []widgets.KeyValueEntry, description ...string) {
	if len(description) == 0 {
		a.ShowInfoDialog(title, entries)
		return
	}
	kv := widgets.NewKeyValueListWidget(entries)
	para := widgets.NewParagraphWidget(description[0])
	para.Box.MarginTop = 1
	vstack := widgets.NewVStackWidget(kv, para)

	dialog := widgets.NewDialogWidget(60)
	dialog.Title = title
	dialog.Borders = *a.Borders
	dialog.SetContent(vstack)
	dialog.Buttons = []widgets.DialogButton{
		{Label: i18n.Text("general.closeButton"), Handler: func() { a.DismissDialog() }},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}

func pluginHelpEntries() []widgets.KeyValueEntry {
	return []widgets.KeyValueEntry{
		{Key: "Enter / Space", Value: i18n.Text("help.pluginActions")},
		{Key: "⋮", Value: i18n.Text("help.openThePluginActionsMenu")},
	}
}

func registerHelpCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID:       "plugin.help",
		Title:    i18n.Text("command.pluginsHelp"),
		Keywords: []string{"plugin", "help"},
		Handler: func() {
			app.ShowPanelHelp(i18n.Text("help.plugins"), pluginHelpEntries(),
				i18n.Text("help.communityPlugins"))
		},
	})

	reg.Register(command.Command{
		ID:       "explorer.help",
		Title:    i18n.Text("command.explorerKeyboardShortcuts"),
		Keywords: []string{"view", "help", "keybindings"},
		Handler: func() {
			app.ShowPanelHelp(i18n.Text("help.explorerShortcuts"), explorerHelpEntries())
		},
	})

	reg.Register(command.Command{
		ID:       "search.help",
		Title:    i18n.Text("command.searchKeyboardShortcuts"),
		Keywords: []string{"search", "help", "keybindings"},
		Handler: func() {
			app.ShowPanelHelp(i18n.Text("help.searchShortcuts"), searchHelpEntries())
		},
	})

	reg.Register(command.Command{
		ID:       "changes.help",
		Title:    i18n.Text("command.changesKeyboardShortcuts"),
		Keywords: []string{"git", "help", "keybindings"},
		Handler: func() {
			app.ShowPanelHelp(i18n.Text("help.changesShortcuts"), changesHelpEntries(),
				i18n.Text("help.commitMessage"))
		},
	})
}
