package ui

import (
	"slices"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

// findSession keeps the editor query after its bar closes and tracks which buffer version the matches describe.
type findSession struct {
	query          string
	options        SearchOptions
	matchesBuffer  *buffer.Buffer
	matchesVersion uint64
	activeMatch    int // Match index in matchesBuffer; -1 when no match is selected.
}

func (g *EditorGroupWidget) SetSearch(query string, opts SearchOptions, matches []FindMatch) {
	g.find = findSession{}
	g.applySearch(query, opts, matches)
}

func (g *EditorGroupWidget) SetFindSearch(query string, matches []FindMatch, opts SearchOptions) {
	g.find = findSession{query: query, options: opts, activeMatch: -1}
	if !g.IsEditorActive() {
		return
	}
	g.find.matchesBuffer = g.SearchPane().Buf
	g.find.matchesVersion = g.SearchPane().Buf.Version()
	g.applySearch(query, opts, matches)
}

func (g *EditorGroupWidget) applySearch(query string, opts SearchOptions, matches []FindMatch) {
	if !g.IsEditorActive() {
		return
	}
	if g.SearchTargetChanged() {
		g.clearSearchState()
	}
	g.searchedPane = g.SearchPane()
	g.SearchPane().SearchQuery = query
	g.SearchPane().SearchOptions = opts
	g.SearchPane().SearchMatches = matches
	g.SearchPane().SearchActive = 0
	g.SearchPane().buildSearchIndex()
}

func (g *EditorGroupWidget) SetSearchActive(idx int) {
	if !g.IsEditorActive() {
		return
	}
	g.SearchPane().SearchActive = idx
	if g.find.query != "" && g.find.matchesBuffer == g.SearchPane().Buf {
		g.find.activeMatch = idx
	}
}

func (g *EditorGroupWidget) ClearSearch() {
	g.find = findSession{}
	g.clearSearchState()
}

func (g *EditorGroupWidget) ClearFindHighlights() {
	active := g.SearchPane().SearchActive
	g.clearSearchState()
	g.SearchPane().SearchActive = active
	g.find.activeMatch = active
}

// clearSearchState also runs while an image or binary tab is active: the
// shared editor keeps the last text tab's highlights and shows them again
// when that tab comes back.
func (g *EditorGroupWidget) clearSearchState() {
	p := g.searchedPane
	if p == nil {
		p = g.SearchPane()
	}
	p.SearchQuery = ""
	p.SearchOptions = SearchOptions{}
	p.SearchMatches = nil
	p.SearchActive = 0
	p.searchByLine = nil
	g.searchedPane = nil
}

func (g *EditorGroupWidget) SearchTargetChanged() bool {
	return g.searchedPane != nil && g.searchedPane != g.SearchPane()
}

func (g *EditorGroupWidget) refreshFindSearch() bool {
	if !g.IsEditorActive() || g.find.query == "" ||
		(g.find.matchesBuffer == g.SearchPane().Buf && g.SearchPane().SearchQuery == g.find.query &&
			g.find.matchesVersion == g.SearchPane().Buf.Version()) {
		return false
	}
	matches, err := FindInLines(g.SearchPane().Buf.Lines, g.find.query, g.find.options)
	if err != nil {
		return false
	}
	bufferChanged := g.find.matchesBuffer != g.SearchPane().Buf
	active := g.find.activeMatch
	if bufferChanged {
		active = -1
	}
	g.find.matchesBuffer = g.SearchPane().Buf
	g.find.matchesVersion = g.SearchPane().Buf.Version()
	g.applySearch(g.find.query, g.find.options, matches)
	if len(matches) == 0 {
		g.find.activeMatch = -1
	} else {
		if active < 0 || active >= len(matches) {
			active = 0
		}
		g.find.activeMatch = active
		g.SearchPane().SearchActive = active
	}
	return bufferChanged
}

func (g *EditorGroupWidget) FindNext() {
	if !g.IsEditorActive() {
		return
	}
	refreshed := g.refreshFindSearch()
	if len(g.SearchPane().SearchMatches) == 0 {
		return
	}
	g.SearchPane().ResetCursorGoal()
	cur := g.SearchPane().SearchActive
	if refreshed {
		cur = -1
	}
	cur = (cur + 1) % len(g.SearchPane().SearchMatches)
	g.SetSearchActive(cur)
	m := g.SearchPane().SearchMatches[cur]
	g.SearchPane().ExpandFoldContaining(m.Line)
	g.SearchPane().Cursor.Line = m.Line
	g.SearchPane().Cursor.Col = m.Col
	g.SearchPane().scrollViewport()
}

func (g *EditorGroupWidget) ReplaceMatch(match FindMatch, replacement string) {
	if !g.IsEditorActive() || g.SearchPane().ReadOnly {
		return
	}
	if match.Line < 0 || match.Line >= len(g.SearchPane().Buf.Lines) {
		return
	}
	if g.SearchPane().Undo != nil {
		g.SearchPane().Undo.BeginTransaction()
		defer g.SearchPane().Undo.EndTransaction()
	}
	if m := g.visualMerge(); m != nil {
		defer m.flushProjection()
	}
	runes := []rune(g.SearchPane().Buf.Lines[match.Line])
	endCol := min(match.Col+match.Len, len(runes))
	g.SearchPane().exec(&undo.DeleteSelectionCommand{
		StartLine: match.Line, StartCol: match.Col,
		EndLine: match.Line, EndCol: endCol,
	})
	if replacement != "" {
		g.SearchPane().exec(&undo.InsertStringCommand{
			Line: match.Line, Col: match.Col, Text: replacement,
		})
	}
	g.SearchPane().Cursor.Line = match.Line
	g.SearchPane().Cursor.Col = match.Col + len([]rune(replacement))
	g.SearchPane().scrollViewport()
}

func (g *EditorGroupWidget) ReplaceAll(query, replacement string, opts SearchOptions) {
	if !g.IsEditorActive() || g.SearchPane().ReadOnly || query == "" {
		return
	}
	matches, _ := FindInLines(g.SearchPane().Buf.Lines, query, opts)
	stack := g.SearchPane().Undo
	if m := g.visualMerge(); m != nil {
		stack = m.result.Undo
	}
	if stack != nil {
		stack.BeginTransaction()
		defer stack.EndTransaction()
	}
	for _, match := range slices.Backward(matches) {
		g.ReplaceMatch(match, replacement)
	}
}

func (g *EditorGroupWidget) FindPrev() {
	if !g.IsEditorActive() {
		return
	}
	refreshed := g.refreshFindSearch()
	if len(g.SearchPane().SearchMatches) == 0 {
		return
	}
	g.SearchPane().ResetCursorGoal()
	cur := g.SearchPane().SearchActive
	if refreshed {
		cur = 0
	}
	cur = (cur - 1 + len(g.SearchPane().SearchMatches)) % len(g.SearchPane().SearchMatches)
	g.SetSearchActive(cur)
	m := g.SearchPane().SearchMatches[cur]
	g.SearchPane().ExpandFoldContaining(m.Line)
	g.SearchPane().Cursor.Line = m.Line
	g.SearchPane().Cursor.Col = m.Col
	g.SearchPane().scrollViewport()
}

func (g *EditorGroupWidget) SearchPane() *EditorPaneWidget {
	if m := g.visualMerge(); m != nil {
		return m.pane(m.focus)
	}
	return g.Editor
}
