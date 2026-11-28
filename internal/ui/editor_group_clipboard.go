package ui

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
)

// TextSelectionContent is tab content that selects the text it draws.
type TextSelectionContent interface {
	CopyTextSelection() bool
	SelectAllText()
}

func (g *EditorGroupWidget) ActiveTextSelection() TextSelectionContent {
	t := g.activeTab()
	if t == nil || t.Content == nil {
		return nil
	}
	sel, _ := t.Content.(TextSelectionContent)
	return sel
}

func (g *EditorGroupWidget) Copy() {
	t := g.activeTab()
	if t == nil {
		return
	}
	if dv, ok := t.Content.(*DiffEditorWidget); ok {
		if text := dv.CopySelection(); text != "" {
			clipboard.Set(text)
		}
		return
	}
	if detail, ok := t.Content.(*CommitDetailWidget); ok {
		if text := detail.CopySelection(); text != "" {
			clipboard.Set(text)
		}
		return
	}
	if t.Content != nil {
		// Non-editor tab (settings UI, plugin panel, ...): no buffer to copy from.
		return
	}
	if t.Diff != nil {
		if text, ok := t.Diff.HeadSelection(); ok {
			if text != "" {
				clipboard.Set(text)
			}
			return
		}
	}
	e := g.keyEditor()

	// Copy from every cursor when multicursor mode is active.
	if e.isMultiActive() {
		pieces := e.multiCopyPieces()
		if len(pieces) > 0 {
			clipboard.Set(strings.Join(pieces, "\n"))
		}
		return
	}

	if e.Selection == nil || !e.Selection.Active {
		// No selection: copy the whole current line.
		if e.Cursor.Line >= 0 && e.Cursor.Line < len(e.Buf.Lines) {
			clipboard.Set(e.Buf.Lines[e.Cursor.Line] + "\n")
		}
		return
	}

	text := e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col)
	clipboard.Set(text)
}

// SelectedText returns the active selection text, or an empty string if no selection is active.
func (g *EditorGroupWidget) SelectedText() string {
	if g == nil {
		return ""
	}
	t := g.activeTab()
	if t == nil {
		return ""
	}
	if dv, ok := t.Content.(*DiffEditorWidget); ok {
		return dv.SelectedText()
	}
	if detail, ok := t.Content.(*CommitDetailWidget); ok {
		return detail.SelectedText()
	}
	if t.Content != nil {
		return ""
	}
	if t.Diff != nil {
		return t.Diff.SelectedText()
	}
	e := g.keyEditor()
	if e == nil || e.Selection == nil || !e.Selection.Active || e.Buf == nil {
		return ""
	}
	return e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col)
}

func (g *EditorGroupWidget) Cut() {
	t := g.activeTab()
	e := g.editableKeyEditor()
	if t == nil || e == nil {
		return
	}
	if m := g.visualMerge(); m != nil {
		defer m.flushProjection()
	}
	// Cut from every cursor when multicursor mode is active.
	if e.isMultiActive() {
		pieces := e.multiCopyPieces()
		if len(pieces) > 0 {
			clipboard.Set(strings.Join(pieces, "\n"))
		}
		e.multiCut()
		return
	}
	if e.Selection == nil || !e.Selection.Active {
		// No selection: cut the whole current line.
		if e.Cursor.Line >= 0 && e.Cursor.Line < len(e.Buf.Lines) {
			clipboard.Set(e.Buf.Lines[e.Cursor.Line] + "\n")
			e.DeleteLine()
		}
		return
	}

	text := e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col)
	clipboard.Set(text)
	e.deleteSelection()
}

func (g *EditorGroupWidget) Paste() {
	if !g.IsEditorActive() {
		return
	}
	text := clipboard.Get()
	if text == "" {
		return
	}
	g.pasteIntoEditor(text)
}

func (g *EditorGroupWidget) PasteText(text string) {
	if !g.IsEditorActive() {
		return
	}
	if text == "" {
		return
	}
	g.pasteIntoEditor(text)
}

func (g *EditorGroupWidget) pasteIntoEditor(text string) {
	if d := g.activeInlineDiff(); d != nil {
		if d.headFocused() {
			return
		}
		d.revealCursorGap()
	}
	if e := g.editableKeyEditor(); e != nil {
		e.pasteText(text)
		if m := g.visualMerge(); m != nil {
			m.flushProjection()
		}
	}
}
