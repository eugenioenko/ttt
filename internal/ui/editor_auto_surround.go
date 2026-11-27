package ui

import (
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/highlight"
)

func (g *EditorGroupWidget) ApplyFileAutoSurround() {
	if g.Editor == nil {
		return
	}
	language := ""
	if t := g.activeTab(); t != nil && t.Buf != nil {
		language = t.LanguageSelection
		if language == "" {
			language = highlight.DetectLanguageID(t.FilePath, t.Buf.Lines)
		}
	}
	settings := config.EditorSettings{AutoSurround: g.AutoSurround, LanguageOverrides: g.LanguageOverrides}
	g.Editor.AutoSurround = settings.ResolveAutoSurround(language)
}

func (e *EditorPaneWidget) surroundSelection(r rune, sel *selection.Selection, line, col *int) *undo.BatchCommand {
	if sel == nil || !sel.Active {
		return nil
	}
	start, end := sel.Range(*line, *col)
	close := e.AutoSurround.Closer(r)
	if start == end || close == 0 {
		return nil
	}
	cmd := &undo.BatchCommand{Commands: []undo.EditCommand{
		&undo.InsertRuneCommand{Line: end.Line, Col: end.Col, Rune: close},
		&undo.InsertRuneCommand{Line: start.Line, Col: start.Col, Rune: r},
	}}
	if *line == start.Line {
		*col++
	}
	if sel.Anchor.Line == start.Line {
		sel.Anchor.Col++
	}
	return cmd
}
