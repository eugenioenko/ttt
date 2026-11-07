package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type helpEntry struct {
	Key     string
	KeyText i18n.Message
	Value   i18n.Message
}

func helpItems(entries []helpEntry) []widgets.KeyValueEntry {
	items := make([]widgets.KeyValueEntry, len(entries))
	for i, entry := range entries {
		key := entry.Key
		if key == "" {
			key = entry.KeyText.Text()
		}
		items[i] = widgets.KeyValueEntry{Key: key, Value: entry.Value.Text()}
	}
	return items
}

var explorerHelpEntries = []helpEntry{
	{KeyText: i18n.Key("help.click"), Value: i18n.Key("help.openFileInPreview")},
	{KeyText: i18n.Key("help.doubleClick"), Value: i18n.Key("help.openFilePermanently")},
	{Key: "Enter", Value: i18n.Key("help.openFileOrToggleFolder")},
	{Key: "Space", Value: i18n.Key("help.openFileOrToggleFolder")},
	{Key: "Shift+Enter", Value: i18n.Key("help.openContextMenu")},
	{Key: "Menu*", Value: i18n.Key("help.openContextMenuTerminalDependent")},
	{Key: "r", Value: i18n.Key("help.refreshExplorer")},
	{Key: "Up / k", Value: i18n.Key("help.moveUp")},
	{Key: "Down / j", Value: i18n.Key("help.moveDown")},
	{Key: "Left / h", Value: i18n.Key("help.collapseFolder")},
	{Key: "Right / l", Value: i18n.Key("help.expandFolder")},
}

var searchHelpEntries = []helpEntry{
	{Key: "Enter", Value: i18n.Key("help.activateSelectedResult")},
	{Key: "Up / k", Value: i18n.Key("help.moveUp")},
	{Key: "Down / j", Value: i18n.Key("help.moveDown")},
	{Key: "Tab", Value: i18n.Key("help.nextInputField")},
	{Key: "Shift+Tab", Value: i18n.Key("help.previousInputField")},
	{Key: "Alt+c", Value: i18n.Key("help.toggleCaseSensitivity")},
	{Key: "Alt+r", Value: i18n.Key("help.toggleRegexMode")},
}

var changesHelpEntries = []helpEntry{
	{Key: "Space / s", Value: i18n.Key("help.toggleStageUnstageFile")},
	{Key: "a", Value: i18n.Key("help.stageAllFiles")},
	{Key: "u", Value: i18n.Key("help.unstageAllFiles")},
	{Key: "d", Value: i18n.Key("help.discardSelectedFile")},
	{Key: "D", Value: i18n.Key("help.discardAllFilesInGroup")},
	{Key: "r", Value: i18n.Key("help.refreshChanges")},
	{Key: "o / v", Value: i18n.Key("help.openFile")},
	{Key: "c", Value: i18n.Key("help.openCompactDiff")},
	{Key: "e", Value: i18n.Key("help.openExtendedDiff")},
	{Key: "Enter", Value: i18n.Key("help.openCompactDiff")},
	{Key: "Up / k", Value: i18n.Key("help.moveUp")},
	{Key: "Down / j", Value: i18n.Key("help.moveDown")},
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

var pluginHelpEntries = []helpEntry{
	{Key: "Enter / Space", Value: i18n.Key("help.pluginActions")},
	{Key: "⋮", Value: i18n.Key("help.openThePluginActionsMenu")},
}

func registerHelpCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID:       "plugin.help",
		Title:    i18n.Text("command.pluginsHelp"),
		Keywords: []string{"plugin", "help"},
		Handler: func() {
			app.ShowPanelHelp(i18n.Text("help.plugins"), helpItems(pluginHelpEntries),
				i18n.Text("help.communityPlugins"))
		},
	})

	reg.Register(command.Command{
		ID:       "explorer.help",
		Title:    i18n.Text("command.explorerKeyboardShortcuts"),
		Keywords: []string{"view", "help", "keybindings"},
		Handler: func() {
			app.ShowPanelHelp(i18n.Text("help.explorerShortcuts"), helpItems(explorerHelpEntries))
		},
	})

	reg.Register(command.Command{
		ID:       "search.help",
		Title:    i18n.Text("command.searchKeyboardShortcuts"),
		Keywords: []string{"search", "help", "keybindings"},
		Handler: func() {
			app.ShowPanelHelp(i18n.Text("help.searchShortcuts"), helpItems(searchHelpEntries))
		},
	})

	reg.Register(command.Command{
		ID:       "changes.help",
		Title:    i18n.Text("command.changesKeyboardShortcuts"),
		Keywords: []string{"git", "help", "keybindings"},
		Handler: func() {
			app.ShowPanelHelp(i18n.Text("help.changesShortcuts"), helpItems(changesHelpEntries),
				i18n.Text("help.commitMessage"))
		},
	})
}
