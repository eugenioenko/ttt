package app

import (
	"strings"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) DiffSearchSources() []ui.DiffSearchSource {
	seen := map[string]bool{}
	var sources []ui.DiffSearchSource
	for _, group := range append([]*ui.EditorGroupWidget{a.EditorGroup}, a.editorGroups()...) {
		for _, source := range group.DiffTabSources() {
			if !seen[source.TabName] {
				seen[source.TabName] = true
				sources = append(sources, source)
			}
		}
	}
	for _, g := range a.Changes.Groups() {
		if !g.IsPR {
			continue
		}
		for path, diffText := range g.PRDiffs {
			tabName := path + " (diff)"
			if seen[tabName] {
				continue
			}
			sources = append(sources, ui.DiffSearchSource{TabName: tabName, IconPath: path, Lines: ui.DiffCombinedLines(diff.Parse(diffText))})
		}
	}
	return sources
}

func (a *App) NavigateToSearchMatch(path string, line, col int) {
	if strings.HasSuffix(path, " (diff)") {
		if !a.switchToDiffTab(path) {
			filePath := strings.TrimSuffix(path, " (diff)")
			for _, g := range a.Changes.Groups() {
				if !g.IsPR {
					continue
				}
				if diffText, ok := g.PRDiffs[filePath]; ok {
					a.EditorGroup.OpenDiff(filePath, diff.Parse(diffText), nil, nil, false)
					break
				}
			}
		}
		if dv := a.EditorGroup.ActiveDiffWidget(); dv != nil {
			dv.ScrollToLine(line - 1)
			dv.ApplySearchHighlight(a.Search.Input.Text(), a.Search.Options)
		}
		a.Root.SetFocus(a.EditorGroup)
		return
	}
	if err := a.EditorGroup.OpenFile(path); err != nil {
		return
	}
	a.EditorGroup.GoToLine(line)
	if a.EditorGroup.IsEditorActive() {
		text := a.EditorGroup.Editor.Buf.Lines[a.EditorGroup.Editor.Cursor.Line]
		runeCol := utf8.RuneCountInString(text[:min(col, len(text))])
		a.EditorGroup.GoToLineCol(line, runeCol+1)
	}
	if a.Search.Input.Text() != "" {
		matches, _ := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, a.Search.Input.Text(), a.Search.Options)
		a.EditorGroup.SetSearch(a.Search.Input.Text(), a.Search.Options, matches)
	}
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) switchToDiffTab(path string) bool {
	if a.EditorGroup.SwitchToTabByPath(path) {
		return true
	}
	for _, group := range a.editorGroups() {
		if group.DiffWidgetByTab(path) != nil {
			a.activateEditorGroup(group)
			return group.SwitchToTabByPath(path)
		}
	}
	return false
}
