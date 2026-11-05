package app

import (
	"context"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) selectedSearchText() string {
	if a.EditorGroup == nil {
		return ""
	}
	text := a.EditorGroup.SelectedText()
	if text == "" || strings.Contains(text, "\n") || strings.Contains(text, "\r") {
		return ""
	}
	return text
}

func (a *App) prefillSearchInput(input *ui.InputWidget) {
	if text := a.selectedSearchText(); text != "" && text != input.Text {
		input.SetText(text)
	}
}

func (a *App) prefillFindBar(bar *ui.FindBarWidget) {
	navigate := bar.OnNavigate
	// Prefill must preserve the cursor that defines the editor selection.
	bar.OnNavigate = nil
	a.prefillSearchInput(bar.Input)
	bar.OnNavigate = navigate
}

func (a *App) OpenFind() {
	if a.Root.HasOverlay() {
		if fb, ok := a.Root.TopOverlayWidget().(*ui.FindBarWidget); ok {
			a.prefillFindBar(fb)
			fb.Focus()
		}
		return
	}
	dv := a.EditorGroup.ActiveDiffWidget()
	if dv == nil {
		dv = a.EditorGroup.ActiveInlineDiff()
	}
	if dv != nil {
		a.showDiffFindBar(dv)
		return
	}
	findBar := ui.NewFindBarWidget()
	findBar.Borders = a.Borders
	findBar.OnSearch = func(query string, opts ui.SearchOptions) []ui.FindMatch {
		if !a.EditorGroup.IsEditorActive() {
			a.EditorGroup.SetFindSearch(query, nil, opts)
			return nil
		}
		matches, err := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, query, opts)
		if err != nil {
			a.EditorGroup.ClearSearch()
			a.StatusWarn("Invalid regex: " + err.Error())
			return nil
		}
		a.EditorGroup.SetFindSearch(query, matches, opts)
		return matches
	}
	findBar.OnNavigate = func(match ui.FindMatch) {
		a.EditorGroup.SetSearchActive(findBar.Current)
		a.EditorGroup.Editor.Selection.Clear()
		a.EditorGroup.Editor.ExpandFoldContaining(match.Line)
		a.EditorGroup.Editor.Cursor.Line = match.Line
		a.EditorGroup.Editor.Cursor.Col = match.Col
		a.EditorGroup.ScrollToCursor()
	}
	findBar.OnDismiss = func() {
		a.DismissDialog()
		a.EditorGroup.ClearFindHighlights()
	}
	findBar.OnEditorTabChange = func() {
		findBar.Current = 0
		findBar.Refresh()
		a.EditorGroup.SetSearchActive(findBar.Current)
	}
	a.prefillFindBar(findBar)
	a.ShowFindBar(findBar)
}

const findRefreshDelay = 150 * time.Millisecond

// Find Next/Prev refresh first if the debounced update has not run yet.
func (a *App) scheduleFindRefresh() {
	if a.findBar() == nil && a.EditorGroup.Editor.SearchQuery == "" {
		return
	}
	a.findStale = true
	if a.findRefresh == nil {
		a.findRefresh = NewTask[struct{}](a, asynctask.TaskOptions{Delay: findRefreshDelay})
	}
	a.findRefresh.Schedule(func(context.Context) (struct{}, error) {
		return struct{}{}, nil
	}, func(struct{}, error) { a.refreshFind() })
}

func (a *App) refreshFind() {
	if !a.findStale {
		return
	}
	a.findStale = false
	if bar := a.findBar(); bar != nil && bar.Query() != "" {
		bar.Refresh()
		a.EditorGroup.SetSearchActive(bar.CurrentMatch())
	} else if a.EditorGroup.IsEditorActive() {
		a.EditorGroup.Editor.RefreshSearch()
	}
}

type findBar interface {
	Query() string
	Refresh()
	CurrentMatch() int
}

func (a *App) findBar() findBar {
	bar, _ := a.Root.TopOverlayWidget().(findBar)
	return bar
}

func (a *App) prefillReplaceInput(input *ui.InputWidget) {
	if !a.EditorGroup.IsEditorActive() || a.EditorGroup.IsActiveReadOnly() {
		return
	}
	if dv := a.EditorGroup.ActiveInlineDiff(); dv != nil && (dv.KeyPane().ReadOnly || dv.HasReadOnlySelection()) {
		return
	}
	a.prefillSearchInput(input)
}

func (a *App) OpenFindReplace() {
	if a.Root.HasOverlay() {
		if bar, ok := a.Root.TopOverlayWidget().(*ui.ReplaceBarWidget); ok {
			a.prefillReplaceInput(bar.SearchInput)
			bar.Focus()
		}
		return
	}
	bar := ui.NewReplaceBarWidget()
	bar.Borders = a.Borders
	bar.OnSearch = func(query string, opts ui.SearchOptions) []ui.FindMatch {
		matches, err := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, query, opts)
		if err != nil {
			a.EditorGroup.ClearSearch()
			a.StatusWarn("Invalid regex: " + err.Error())
			return nil
		}
		a.EditorGroup.SetSearch(query, opts, matches)
		return matches
	}
	bar.OnNavigate = func(match ui.FindMatch) {
		a.EditorGroup.SetSearchActive(bar.Current)
		a.EditorGroup.Editor.Selection.Clear()
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
	bar.OnDismiss = func() {
		a.DismissDialog()
		a.EditorGroup.ClearSearch()
	}
	a.prefillReplaceInput(bar.SearchInput)
	a.ShowFindBar(bar)
}

func (a *App) ClearGlobalSearch() {
	a.Search.CancelReplacement()
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
		Handler:  func() { app.refreshFind(); app.EditorGroup.FindNext() },
	})

	reg.Register(command.Command{
		ID: "search.findPrev", Title: "Find Previous",
		Keywords: []string{"search", "find"},
		Handler:  func() { app.refreshFind(); app.EditorGroup.FindPrev() },
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
