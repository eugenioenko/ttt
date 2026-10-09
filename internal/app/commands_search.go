package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) OpenFind() {
	if a.Root.HasOverlay() {
		if fb, ok := a.Root.TopOverlayWidget().(*ui.FindBarWidget); ok {
			fb.Focus()
		}
		return
	}
	dv := a.EditorGroup.ActiveDiffWidget()
	if dv != nil {
		a.showDiffFindBar(dv)
		return
	}
	findBar := ui.NewFindBarWidget()
	findBar.Borders = a.Borders
	findBar.OnSearch = func(query string, opts ui.SearchOptions) []ui.FindMatch {
		matches, err := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, query, opts)
		if err != nil {
			a.StatusWarn("Invalid regex: " + err.Error())
			return nil
		}
		a.EditorGroup.SetSearch(query, opts, matches)
		return matches
	}
	findBar.OnNavigate = func(match ui.FindMatch) {
		a.EditorGroup.SetSearchActive(findBar.Current)
		a.EditorGroup.Editor.ExpandFoldContaining(match.Line)
		a.EditorGroup.Editor.Cursor.Line = match.Line
		a.EditorGroup.Editor.Cursor.Col = match.Col
		a.EditorGroup.ScrollToCursor()
	}
	a.EditorGroup.Editor.OnSearchRefresh = func(matches []ui.FindMatch) {
		findBar.SetMatches(matches)
		a.EditorGroup.SetSearchActive(findBar.Current)
	}
	findBar.OnDismiss = func() {
		a.EditorGroup.Editor.OnSearchRefresh = nil
		a.DismissDialog()
		a.EditorGroup.ClearSearch()
	}
	a.ShowFindBar(findBar)
}

func (a *App) OpenFindReplace() {
	if a.Root.HasOverlay() {
		return
	}
	bar := ui.NewReplaceBarWidget()
	bar.Borders = a.Borders
	bar.OnSearch = func(query string, opts ui.SearchOptions) []ui.FindMatch {
		matches, err := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, query, opts)
		if err != nil {
			a.StatusWarn("Invalid regex: " + err.Error())
			return nil
		}
		a.EditorGroup.SetSearch(query, opts, matches)
		return matches
	}
	bar.OnNavigate = func(match ui.FindMatch) {
		a.EditorGroup.SetSearchActive(bar.Current)
		a.EditorGroup.Editor.ExpandFoldContaining(match.Line)
		a.EditorGroup.Editor.Cursor.Line = match.Line
		a.EditorGroup.Editor.Cursor.Col = match.Col
		a.EditorGroup.ScrollToCursor()
	}
	bar.OnReplace = func(match ui.FindMatch, replacement string) {
		a.EditorGroup.ReplaceMatch(match, replacement)
	}
	bar.OnReplaceAll = func(query, replacement string, opts ui.SearchOptions) {
		a.EditorGroup.ReplaceAll(query, replacement, opts)
	}
	a.EditorGroup.Editor.OnSearchRefresh = func(matches []ui.FindMatch) {
		bar.SetMatches(matches)
		a.EditorGroup.SetSearchActive(bar.Current)
	}
	bar.OnDismiss = func() {
		a.EditorGroup.Editor.OnSearchRefresh = nil
		a.DismissDialog()
		a.EditorGroup.ClearSearch()
	}
	a.ShowFindBar(bar)
}

func (a *App) ClearGlobalSearch() {
	a.Search.Input.Text = ""
	a.Search.Input.CursorPos = 0
	a.Search.Groups = nil
	a.Search.FlatList = nil
	a.Search.Selected = 0
	a.Search.ScrollTop = 0
	a.EditorGroup.ClearSearch()
}

func registerSearchCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "search.find", Title: "Find",
		Keywords: []string{"search", "find", "locate"},
		Handler:  app.OpenFind,
	})

	reg.Register(command.Command{
		ID: "search.findNext", Title: "Find Next",
		Keywords: []string{"search", "find"},
		Handler:  func() { app.EditorGroup.FindNext() },
	})

	reg.Register(command.Command{
		ID: "search.findPrev", Title: "Find Previous",
		Keywords: []string{"search", "find"},
		Handler:  func() { app.EditorGroup.FindPrev() },
	})

	reg.Register(command.Command{
		ID: "search.clearFind", Title: "Clear Find Highlights",
		Keywords: []string{"search", "find"},
		Handler:  func() { app.EditorGroup.ClearSearch() },
	})

	reg.Register(command.Command{
		ID: "search.replace", Title: "Find and Replace",
		Keywords: []string{"search", "find", "replace", "substitute"},
		Handler:  app.OpenFindReplace,
	})

	reg.Register(command.Command{
		ID: "search.expandAll", Title: "Expand All Search Results",
		Keywords: []string{"search"},
		Handler:  func() { app.Search.ExpandAll() },
	})

	reg.Register(command.Command{
		ID: "search.collapseAll", Title: "Collapse All Search Results",
		Keywords: []string{"search"},
		Handler:  func() { app.Search.CollapseAll() },
	})

	reg.Register(command.Command{
		ID: "search.clear", Title: "Clear Search Results",
		Keywords: []string{"search"},
		Handler:  app.ClearGlobalSearch,
	})
}
