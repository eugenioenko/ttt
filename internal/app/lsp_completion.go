package app

import (
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) ShowAutocomplete(items []ui.CompletionItem, lspItems []lsp.CompletionItem) {
	a.CompletionItems = items
	a.LspCompletionItems = lspItems
	prefix := a.currentPrefix()
	filtered := ui.FilterCompletions(items, prefix)
	if len(filtered) == 0 {
		return
	}
	a.completion.pathPopup = nil
	ac := ui.NewAutocompleteWidget(filtered, 0, 0)
	ac.Icons = a.Settings.Appearance.Icons
	ac.OnSelect = func(item ui.CompletionItem) {
		a.resolveAndInsert(item)
		a.CloseAutocomplete()
	}
	ac.OnDismiss = a.CloseAutocomplete
	a.EditorGroup.Autocomplete = ac
}

func (a *App) RefreshAutocomplete() {
	if target := a.completion.pathPopup; target != nil {
		if a.currentPathCompletionTarget() != *target {
			a.DismissAutocomplete()
		}
		return
	}
	if a.EditorGroup.Autocomplete == nil || len(a.CompletionItems) == 0 {
		return
	}
	prefix := a.currentPrefix()
	if prefix == "" {
		if !a.isCompletionTrigger(a.charBeforeCursor()) {
			a.DismissAutocomplete()
		}
		return
	}
	filtered := ui.FilterCompletions(a.CompletionItems, prefix)
	if len(filtered) == 0 {
		a.DismissAutocomplete()
		return
	}
	a.EditorGroup.Autocomplete.SetItems(filtered)
}

// isCompletionTrigger reports whether ch is a completion trigger character of
// the active file's language server. Each server has its own: a space that
// triggers TypeScript completions must not open them in Go.
func (a *App) isCompletionTrigger(ch string) bool {
	if ch == "" {
		return false
	}
	path, lang := a.editorPathLang()
	serverKey, _, ok := a.lspResolve(path, lang)
	return ok && slices.Contains(a.LspManager.CompletionTriggerCharacters(serverKey), ch)
}

func (a *App) identStart() (line, start, col int) {
	if !a.EditorGroup.IsEditorActive() {
		return 0, 0, 0
	}
	editor := a.EditorGroup.Editor
	line = editor.Cursor.Line
	col = editor.Cursor.Col
	if line >= len(editor.Buf.Lines) {
		return 0, 0, 0
	}
	runes := []rune(editor.Buf.Lines[line])
	start = col
	if start > len(runes) {
		start = len(runes)
	}
	for start > 0 && isIdentRune(runes[start-1]) {
		start--
	}
	return line, start, col
}

func (a *App) currentPrefix() string {
	if !a.EditorGroup.IsEditorActive() {
		return ""
	}
	editor := a.EditorGroup.Editor
	line, start, col := a.identStart()
	runes := []rune(editor.Buf.Lines[line])
	if col > len(runes) {
		col = len(runes)
	}
	if start > col {
		return ""
	}
	return string(runes[start:col])
}

func (a *App) DismissAutocomplete() {
	a.completion.pathPopup = nil
	a.EditorGroup.Autocomplete = nil
	a.CompletionItems = nil
	a.LspCompletionItems = nil
	a.completion.lines = nil
}

func (a *App) IsAutocompleteActive() bool {
	return a.EditorGroup.Autocomplete != nil
}

func (a *App) resolveAndInsert(item ui.CompletionItem) {
	var lspItem *lsp.CompletionItem
	for i, li := range a.LspCompletionItems {
		if li.Label == item.Label {
			lspItem = &a.LspCompletionItems[i]
			break
		}
	}

	if lspItem != nil && len(lspItem.AdditionalTextEdits) == 0 {
		path, lang := a.editorPathLang()
		serverKey, _, ok := a.lspResolve(path, lang)
		if ok {
			if client := a.LspManager.ReadyClient(serverKey, a.lspWorkDirFor(path, lang)); client != nil {
				resolved, err := client.ResolveCompletion(*lspItem)
				if err == nil && resolved != nil {
					if resolved.TextEdit != nil {
						lspItem = resolved
					}
					for _, edit := range runeTextEdits(a.completion.lines, resolved.AdditionalTextEdits) {
						item.AdditionalEdits = append(item.AdditionalEdits, ui.AdditionalEdit{
							StartLine: edit.Range.Start.Line,
							StartCol:  edit.Range.Start.Character,
							EndLine:   edit.Range.End.Line,
							EndCol:    edit.Range.End.Character,
							NewText:   edit.NewText,
						})
					}
				}
			}
		}
	}

	var textEdit *lsp.TextEdit
	if lspItem != nil && lspItem.TextEdit != nil {
		textEdit = lspItem.TextEdit.ToTextEdit()
	}
	a.insertCompletion(item, textEdit)
}

func (a *App) insertCompletion(item ui.CompletionItem, textEdit *lsp.TextEdit) {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	editor := a.EditorGroup.Editor

	var text string
	var startLine, startCol, endLine, endCol int

	if textEdit != nil {
		converted := *textEdit
		lines := a.completion.lines
		if lines == nil {
			lines = editor.Buf.Lines
		}
		converted.Range = lsp.RangeToRunes(lines, converted.Range)
		textEdit = &converted
		text = textEdit.NewText
		startLine = textEdit.Range.Start.Line
		startCol = textEdit.Range.Start.Character
		endLine = textEdit.Range.End.Line
		endCol = textEdit.Range.End.Character
		// LSP requires a single-line range, but TypeScript sends a multi-line
		// one for an unterminated JSX attribute string, which would delete the
		// following lines. VS Code clamps too.
		if endLine > startLine && startLine < len(lines) {
			endLine = startLine
			endCol = len([]rune(lines[startLine]))
		}
		if cursorLine, _, _ := a.identStart(); startLine != cursorLine {
			return
		}
		if startLine == endLine && startCol == endCol {
			_, identCol, cursorCol := a.identStart()
			if identCol < endCol || endCol <= cursorCol {
				startCol = identCol
				endCol = cursorCol
			}
		} else if cursorLine, identCol, cursorCol := a.identStart(); startLine == endLine && endLine == cursorLine && identCol <= endCol && endCol < cursorCol {
			// The range covers the word as it was when completion was
			// requested; characters typed since then belong to it too.
			endCol = cursorCol
		}
	} else {
		text = item.InsertText
		if text == "" {
			text = item.Label
		}
		startLine, startCol, endCol = a.identStart()
		endLine = startLine
	}

	if startLine < 0 || endLine < startLine || endLine >= len(editor.Buf.Lines) {
		return
	}

	var snippet lsp.SnippetExpansion
	if item.InsertTextFormat == lsp.InsertTextFormatSnippet {
		snippet = lsp.ExpandSnippet(text)
		text = snippet.Text
	}

	// Every range refers to the text before acceptance, including edits after
	// the completed word on the same line.
	edits := []lsp.TextEdit{{
		Range:   lsp.Range{Start: lsp.Position{Line: startLine, Character: startCol}, End: lsp.Position{Line: endLine, Character: endCol}},
		NewText: text,
	}}
	additional := make([]lsp.TextEdit, 0, len(item.AdditionalEdits))
	for _, edit := range item.AdditionalEdits {
		if lines := a.completion.lines; startLine < len(lines) && startLine == endLine {
			delta := len([]rune(editor.Buf.Lines[startLine])) - len([]rune(lines[startLine]))
			if edit.StartLine == startLine && edit.StartCol >= a.completion.col {
				edit.StartCol += delta
			}
			if edit.EndLine == startLine && edit.EndCol >= a.completion.col {
				edit.EndCol += delta
			}
		}
		additional = append(additional, lsp.TextEdit{
			Range:   lsp.Range{Start: lsp.Position{Line: edit.StartLine, Character: edit.StartCol}, End: lsp.Position{Line: edit.EndLine, Character: edit.EndCol}},
			NewText: edit.NewText,
		})
	}
	sortTextEdits(additional)
	orderedAdditional := slices.Clone(additional)
	reverseSamePositionTextEdits(orderedAdditional)
	cursor := lsp.Position{Line: startLine, Character: startCol}
	for _, edit := range orderedAdditional {
		end := edit.Range.End
		if end.Line > cursor.Line || (end.Line == cursor.Line && end.Character > cursor.Character) {
			continue
		}
		replacement := strings.Split(edit.NewText, "\n")
		if end.Line == cursor.Line {
			cursor.Character -= end.Character
			cursor.Character += len([]rune(replacement[len(replacement)-1]))
			if len(replacement) == 1 {
				cursor.Character += edit.Range.Start.Character
			}
		}
		cursor.Line += len(replacement) - 1 - (end.Line - edit.Range.Start.Line)
	}
	if editor.Selection != nil {
		editor.Selection.Clear()
	}
	if stop, ok := snippet.FirstStop(); ok {
		stopLine, stopCol := snippetPosition(text, cursor.Line, cursor.Character, stop.Offset)
		stopEndLine, stopEndCol := snippetPosition(text, cursor.Line, cursor.Character, stop.Offset+stop.Length)
		if stop.Length > 0 {
			editor.Selection.Start(stopLine, stopCol)
			cursor.Line, cursor.Character = stopEndLine, stopEndCol
		} else {
			cursor.Line, cursor.Character = stopLine, stopCol
		}
	} else {
		cursor.Line, cursor.Character = snippetPosition(text, cursor.Line, cursor.Character, len([]rune(text)))
	}
	a.applyRuneTextEdits(append(edits, additional...), &cursor)
}

// snippetPosition converts a rune offset in a snippet expansion to an absolute
// buffer position. The first line starts at startCol; later lines start at 0.
func snippetPosition(text string, startLine, startCol, offset int) (line, col int) {
	if offset < 0 {
		offset = 0
	}
	for _, r := range text {
		if offset == 0 {
			break
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		offset--
	}
	if line == 0 {
		col += startCol
	}
	return startLine + line, col
}

func (a *App) charBeforeCursor() string {
	if !a.EditorGroup.IsEditorActive() {
		return ""
	}
	editor := a.EditorGroup.Editor
	line := editor.Cursor.Line
	col := editor.Cursor.Col
	if col <= 0 || line >= len(editor.Buf.Lines) {
		return ""
	}
	runes := []rune(editor.Buf.Lines[line])
	if col > len(runes) {
		return ""
	}
	return string(runes[col-1])
}
