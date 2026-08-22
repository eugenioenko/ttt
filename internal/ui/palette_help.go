package ui

import (
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
)

type paletteItemKind uint8

// Help favors trustworthy results over fuzzy recall. Weak character-order
// coincidences remain below the absolute floor, while the relative floor keeps
// secondary metadata associations from trailing a much stronger title match.
const (
	paletteHelpScoreFloor          = 200
	paletteHelpRelativeNumerator   = 3
	paletteHelpRelativeDenominator = 4
)

const (
	paletteCommandItem paletteItemKind = iota
	paletteHelpTopicItem
)

type paletteHelpTopic struct {
	ID              string
	Title           string
	Description     string
	CommandPrefixes []string
	CommandIDs      []string
	ChordsOnly      bool
}

// paletteHelpTopics is a deliberately curated new-user onramp. Command titles
// and shortcuts remain derived from the command registry and keybinding config;
// only this small orientation layer is authored by hand and maintained here.
var paletteHelpTopics = []paletteHelpTopic{
	{
		ID:              "workspace",
		Title:           "Workspace map",
		Description:     "Understand folders, tabs, and editor groups",
		CommandPrefixes: []string{"workspace.", "file.", "tab.", "focus."},
		CommandIDs:      []string{"command.palette"},
	},
	{
		ID:              "panels",
		Title:           "Sidebar and panels",
		Description:     "Use Explorer, Search, Changes, and Output",
		CommandPrefixes: []string{"sidebar.", "panel.", "output."},
		CommandIDs:      []string{"explorer.help", "search.help", "changes.help"},
	},
	{
		ID:              "navigation",
		Title:           "Navigate files",
		Description:     "Open files, switch tabs, and move focus",
		CommandPrefixes: []string{"file.", "tab.", "focus."},
		CommandIDs:      []string{"editor.goToLine", "command.palette"},
	},
	{
		ID:              "changes",
		Title:           "Review changes",
		Description:     "Review, stage, and commit workspace changes",
		CommandPrefixes: []string{"changes.", "git.", "pr."},
		CommandIDs:      []string{"sidebar.changes", "changes.help"},
	},
	{
		ID:              "terminal",
		Title:           "Integrated terminal",
		Description:     "Open shells without leaving the editor",
		CommandPrefixes: []string{"terminal."},
		CommandIDs:      []string{"focus.terminal"},
	},
	{
		ID:          "chords",
		Title:       "Keybinding chords",
		Description: "Use multi-step chords and edit shortcuts",
		CommandIDs:  []string{"view.keybindings", "keybindings.open"},
		ChordsOnly:  true,
	},
}

func buildPaletteHelpItems(commands []command.Command) ([]PaletteItem, []PaletteItem) {
	topics := make([]PaletteItem, 0, len(paletteHelpTopics))
	for _, topic := range paletteHelpTopics {
		topics = append(topics, PaletteItem{
			Label:       topic.Title,
			ID:          "help:" + topic.ID,
			Description: topic.Description,
			kind:        paletteHelpTopicItem,
			topicID:     topic.ID,
			searchText:  []string{topic.Title, topic.Description, topic.ID},
		})
	}

	items := make([]PaletteItem, 0, len(commands))
	for _, cmd := range commands {
		description := paletteCommandDescription(cmd.ID)
		searchText := []string{cmd.Title, cmd.ID, cmd.Shortcut, description}
		searchText = append(searchText, cmd.Keywords...)
		items = append(items, PaletteItem{
			Label:       cmd.Title,
			Detail:      cmd.Shortcut,
			ID:          cmd.ID,
			Description: description,
			kind:        paletteCommandItem,
			searchText:  searchText,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})
	return topics, items
}

func paletteCommandDescription(id string) string {
	switch {
	case strings.HasPrefix(id, "workspace."):
		return "Manage folders and saved workspaces"
	case strings.HasPrefix(id, "file."):
		return "Create, open, save, or find a file"
	case strings.HasPrefix(id, "tab."), strings.HasPrefix(id, "focus."):
		return "Move between open files and editor groups"
	case strings.HasPrefix(id, "sidebar."), strings.HasPrefix(id, "panel."), strings.HasPrefix(id, "output."):
		return "Show, hide, or focus an editor panel"
	case strings.HasPrefix(id, "changes."), strings.HasPrefix(id, "git."), strings.HasPrefix(id, "pr."):
		return "Inspect or act on version-control changes"
	case strings.HasPrefix(id, "terminal."):
		return "Use the integrated terminal"
	case strings.HasPrefix(id, "settings."), strings.HasPrefix(id, "theme."), strings.HasPrefix(id, "options."), strings.HasPrefix(id, "keybindings."):
		return "Customize behavior and shortcuts"
	default:
		return "Run this editor command"
	}
}

func paletteHelpTopicByID(id string) (paletteHelpTopic, bool) {
	for _, topic := range paletteHelpTopics {
		if topic.ID == id {
			return topic, true
		}
	}
	return paletteHelpTopic{}, false
}

func paletteTopicMatchesCommand(topic paletteHelpTopic, item PaletteItem) bool {
	for _, id := range topic.CommandIDs {
		if item.ID == id {
			return true
		}
	}
	for _, prefix := range topic.CommandPrefixes {
		if strings.HasPrefix(item.ID, prefix) {
			return true
		}
	}
	return topic.ChordsOnly && strings.Contains(item.Detail, " ")
}

func scorePaletteHelpItem(query string, item PaletteItem) (bool, int) {
	bestOK, bestScore := fuzzyMatch(query, item.Label)
	for _, text := range item.searchText {
		if ok, score := fuzzyMatch(query, text); ok {
			if text != item.Label {
				score /= 2
			}
			if !bestOK || score > bestScore {
				bestOK = true
				bestScore = score
			}
		}
	}
	return bestOK, bestScore
}
