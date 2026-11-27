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
	g.find.matchesBuffer = g.Editor.Buf
	g.find.matchesVersion = g.Editor.Buf.Version()
	g.applySearch(query, opts, matches)
}

func (g *EditorGroupWidget) applySearch(query string, opts SearchOptions, matches []FindMatch) {
	if !g.IsEditorActive() {
		return
	}
	g.Editor.SearchQuery = query
	g.Editor.SearchOptions = opts
	g.Editor.SearchMatches = matches
	g.Editor.SearchActive = 0
	g.Editor.buildSearchIndex()
}

func (g *EditorGroupWidget) SetSearchActive(idx int) {
	if !g.IsEditorActive() {
		return
	}
	g.Editor.SearchActive = idx
	if g.find.query != "" && g.find.matchesBuffer == g.Editor.Buf {
		g.find.activeMatch = idx
	}
}

func (g *EditorGroupWidget) ClearSearch() {
	g.find = findSession{}
	g.clearSearchState()
}

func (g *EditorGroupWidget) ClearFindHighlights() {
	active := g.Editor.SearchActive
	g.clearSearchState()
	g.Editor.SearchActive = active
	g.find.activeMatch = active
}

// clearSearchState also runs while an image or binary tab is active: the
// shared editor keeps the last text tab's highlights and shows them again
// when that tab comes back.
func (g *EditorGroupWidget) clearSearchState() {
	g.Editor.SearchQuery = ""
	g.Editor.SearchOptions = SearchOptions{}
	g.Editor.SearchMatches = nil
	g.Editor.SearchActive = 0
	g.Editor.searchByLine = nil
}

func (g *EditorGroupWidget) refreshFindSearch() bool {
	if !g.IsEditorActive() || g.find.query == "" ||
		(g.find.matchesBuffer == g.Editor.Buf && g.Editor.SearchQuery == g.find.query &&
			g.find.matchesVersion == g.Editor.Buf.Version()) {
		return false
	}
	matches, err := FindInLines(g.Editor.Buf.Lines, g.find.query, g.find.options)
	if err != nil {
		return false
	}
	bufferChanged := g.find.matchesBuffer != g.Editor.Buf
	active := g.find.activeMatch
	if bufferChanged {
		active = -1
	}
	g.find.matchesBuffer = g.Editor.Buf
	g.find.matchesVersion = g.Editor.Buf.Version()
	g.applySearch(g.find.query, g.find.options, matches)
	if len(matches) == 0 {
		g.find.activeMatch = -1
	} else {
		if active < 0 || active >= len(matches) {
			active = 0
		}
		g.find.activeMatch = active
		g.Editor.SearchActive = active
	}
	return bufferChanged
}

func (g *EditorGroupWidget) FindNext() {
	if !g.IsEditorActive() {
		return
	}
	refreshed := g.refreshFindSearch()
	if len(g.Editor.SearchMatches) == 0 {
		return
	}
	g.Editor.ResetCursorGoal()
	cur := g.Editor.SearchActive
	if refreshed {
		cur = -1
	}
	cur = (cur + 1) % len(g.Editor.SearchMatches)
	g.SetSearchActive(cur)
	m := g.Editor.SearchMatches[cur]
	g.Editor.ExpandFoldContaining(m.Line)
	g.Editor.Cursor.Line = m.Line
	g.Editor.Cursor.Col = m.Col
	g.Editor.scrollViewport()
}

func (g *EditorGroupWidget) ReplaceMatch(match FindMatch, replacement string) {
	if !g.IsEditorActive() {
		return
	}
	if match.Line < 0 || match.Line >= len(g.Editor.Buf.Lines) {
		return
	}
	if g.Editor.Undo != nil {
		g.Editor.Undo.BeginTransaction()
		defer g.Editor.Undo.EndTransaction()
	}
	runes := []rune(g.Editor.Buf.Lines[match.Line])
	endCol := min(match.Col+match.Len, len(runes))
	g.Editor.exec(&undo.DeleteSelectionCommand{
		StartLine: match.Line, StartCol: match.Col,
		EndLine: match.Line, EndCol: endCol,
	})
	if replacement != "" {
		g.Editor.exec(&undo.InsertStringCommand{
			Line: match.Line, Col: match.Col, Text: replacement,
		})
	}
	g.Editor.Cursor.Line = match.Line
	g.Editor.Cursor.Col = match.Col + len([]rune(replacement))
	g.Editor.scrollViewport()
}

func (g *EditorGroupWidget) ReplaceAll(query, replacement string, opts SearchOptions) {
	if !g.IsEditorActive() || query == "" {
		return
	}
	matches, _ := FindInLines(g.Editor.Buf.Lines, query, opts)
	if g.Editor.Undo != nil {
		g.Editor.Undo.BeginTransaction()
		defer g.Editor.Undo.EndTransaction()
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
	if len(g.Editor.SearchMatches) == 0 {
		return
	}
	g.Editor.ResetCursorGoal()
	cur := g.Editor.SearchActive
	if refreshed {
		cur = 0
	}
	cur = (cur - 1 + len(g.Editor.SearchMatches)) % len(g.Editor.SearchMatches)
	g.SetSearchActive(cur)
	m := g.Editor.SearchMatches[cur]
	g.Editor.ExpandFoldContaining(m.Line)
	g.Editor.Cursor.Line = m.Line
	g.Editor.Cursor.Col = m.Col
	g.Editor.scrollViewport()
}
