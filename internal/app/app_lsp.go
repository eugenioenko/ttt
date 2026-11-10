package app

import (
	"errors"
	"log/slog"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func (a *App) ShowAutocomplete(items []ui.CompletionItem, lspItems []lsp.CompletionItem) {
	a.CompletionItems = items
	a.LspCompletionItems = lspItems
	prefix := a.currentPrefix()
	filtered := ui.FilterCompletions(items, prefix)
	if len(filtered) == 0 {
		return
	}
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
			workDir := a.lspWorkDir(path)
			client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
			if err == nil {
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

// changeDone must be the NotifyLSPChange channel for this same keystroke, so
// the server sees that didChange before the signatureHelp request it triggers.
func (a *App) CheckSignatureHelpTrigger(changeDone <-chan struct{}) {
	if !a.Settings.Autocomplete.Enabled || !a.Settings.Autocomplete.SignatureHelp {
		return
	}
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	editor := a.EditorGroup.Editor
	line := editor.Cursor.Line
	col := editor.Cursor.Col
	if col <= 0 || line >= len(editor.Buf.Lines) {
		return
	}
	runes := []rune(editor.Buf.Lines[line])
	if col > len(runes) {
		return
	}
	ch := string(runes[col-1])
	if ch == ")" {
		a.DismissSignatureHelp()
		return
	}
	path, lang := a.editorPathLang()
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	triggers := a.LspManager.SignatureHelpTriggerCharacters(serverKey)
	for _, tc := range triggers {
		if ch == tc {
			a.RequestSignatureHelp(path, lang, line, col, changeDone)
			return
		}
	}
}

func (a *App) ShowSignatureHelp(result *SignatureHelpResult) {
	w := ui.NewSignatureHelpWidget(result.Label, result.ParamStart, result.ParamEnd)
	a.EditorGroup.SignatureHelp = w
}

func (a *App) editorTabSize() (int, bool) {
	tabSize := a.Settings.Editor.TabSize
	insertSpaces := a.Settings.Editor.InsertSpaces
	if a.EditorGroup.Editor != nil && a.EditorGroup.Editor.TabSize > 0 {
		tabSize = a.EditorGroup.Editor.TabSize
		insertSpaces = !a.EditorGroup.Editor.UseTabs
	}
	return tabSize, insertSpaces
}

func (a *App) RequestFormatting(path, lang string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	workDir := a.lspWorkDir(path)
	tabSize, insertSpaces := a.editorTabSize()
	version := a.editorBufferVersion()
	buf := a.editorBuffer()
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		edits, err := client.Formatting(FileURI(path), tabSize, insertSpaces)
		if err != nil {
			slog.Error("lsp formatting", "err", err)
			return
		}
		if len(edits) > 0 {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&FormattingResult{
				Path: path, Version: version, Buffer: buf, Edits: edits,
			}))
		}
	}()
}

func (a *App) RequestRangeFormatting(path, lang string, startLine, startCol, endLine, endCol int) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	workDir := a.lspWorkDir(path)
	tabSize, insertSpaces := a.editorTabSize()
	version := a.editorBufferVersion()
	buf := a.editorBuffer()
	r := lsp.Range{
		Start: lsp.Position{Line: startLine, Character: a.lspColumn(path, startLine, startCol)},
		End:   lsp.Position{Line: endLine, Character: a.lspColumn(path, endLine, endCol)},
	}
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		edits, err := client.RangeFormatting(FileURI(path), r, tabSize, insertSpaces)
		if err != nil {
			slog.Error("lsp rangeFormatting", "err", err)
			return
		}
		if len(edits) > 0 {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&FormattingResult{
				Path: path, Version: version, Buffer: buf, Edits: edits,
			}))
		}
	}()
}

func (a *App) RunCodeActionsOnSave(path, lang string) {
	if len(a.Settings.LSP.CodeActionsOnSave) == 0 {
		return
	}
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	workDir := a.lspWorkDir(path)
	client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
	if err != nil {
		return
	}
	lineCount := 0
	if a.EditorGroup.Editor != nil {
		lineCount = len(a.EditorGroup.Editor.Buf.Lines)
	}
	fullRange := lsp.Range{
		Start: lsp.Position{Line: 0, Character: 0},
		End:   lsp.Position{Line: lineCount, Character: 0},
	}
	actions, err := client.CodeAction(FileURI(path), fullRange, a.Settings.LSP.CodeActionsOnSave)
	if err != nil {
		slog.Error("lsp codeActionsOnSave", "err", err)
		return
	}
	for _, action := range actions {
		if action.Edit != nil {
			a.applyCodeActionEdit(path, action.Edit)
		}
	}
}

func (a *App) applyCodeActionEdit(path string, edit *lsp.WorkspaceEdit) {
	batches, ok := a.workspaceTextEdits(edit)
	if !ok {
		return
	}
	for _, batch := range batches {
		if URIToPath(batch.URI) == path && a.EditorGroup.ActiveFilePath() == path {
			a.ApplyTextEdits(batch.Edits)
		}
	}
}

func (a *App) workspaceTextEdits(edit *lsp.WorkspaceEdit) ([]lsp.FileEdits, bool) {
	batches, err := edit.TextEdits()
	if errors.Is(err, lsp.ErrFileOperations) {
		a.StatusWarn(i18n.Text("status.unsupportedWorkspaceEdit"))
		return nil, false
	}
	return batches, true
}

func (a *App) RequestCodeAction(path, lang, kind string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	workDir := a.lspWorkDir(path)
	lineCount := 0
	if a.EditorGroup.Editor != nil {
		lineCount = len(a.EditorGroup.Editor.Buf.Lines)
	}
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		fullRange := lsp.Range{
			Start: lsp.Position{Line: 0, Character: 0},
			End:   lsp.Position{Line: lineCount, Character: 0},
		}
		actions, err := client.CodeAction(FileURI(path), fullRange, []string{kind})
		if err != nil {
			slog.Error("lsp codeAction", "err", err)
			return
		}
		if edit := pickCodeActionEdit(actions); edit != nil {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionResult{Path: path, Edit: edit}))
		}
	}()
}

// pickCodeActionEdit returns the first action's edit that ttt can apply. If
// none can, it returns the first one with file operations, so applying it
// reports why nothing changed.
func pickCodeActionEdit(actions []lsp.CodeAction) *lsp.WorkspaceEdit {
	var unsupported *lsp.WorkspaceEdit
	for _, action := range actions {
		batches, err := action.Edit.TextEdits()
		if errors.Is(err, lsp.ErrFileOperations) {
			if unsupported == nil {
				unsupported = action.Edit
			}
			continue
		}
		if len(batches) > 0 {
			return action.Edit
		}
	}
	return unsupported
}

func (a *App) FormatOnSave(path, lang string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	workDir := a.lspWorkDir(path)
	tabSize, insertSpaces := a.editorTabSize()
	client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
	if err != nil {
		return
	}
	edits, err := client.Formatting(FileURI(path), tabSize, insertSpaces)
	if err != nil {
		slog.Error("lsp formatOnSave", "err", err)
		return
	}
	if len(edits) > 0 {
		a.ApplyTextEdits(edits)
	}
}

func (a *App) RequestRename(path, lang string, line, col int, newName string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	col = a.lspColumn(path, line, col)
	workDir := a.lspWorkDir(path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		edit, err := client.Rename(FileURI(path), line, col, newName)
		if err != nil {
			slog.Error("lsp rename", "err", err)
			return
		}
		if edit != nil {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&RenameResult{Edit: edit}))
		}
	}()
}

func (a *App) ApplyWorkspaceEdit(edit *lsp.WorkspaceEdit) {
	batches, ok := a.workspaceTextEdits(edit)
	if !ok || len(batches) == 0 {
		return
	}
	filesCount, _ := a.applyWorkspaceEditBatches(batches, true)
	a.StatusNotify(i18n.Format("status.renamedAcrossFileS", i18n.Args{"filesCount": filesCount}))
}

func (a *App) applyWorkspaceEditBatches(batches []lsp.FileEdits, saveOnRename bool) (int, bool) {
	currentPath := a.EditorGroup.ActiveFilePath()

	files := make(map[string]bool)
	allApplied := true
	for _, batch := range batches {
		path := URIToPath(batch.URI)
		if err := a.EditorGroup.OpenFile(path); err != nil {
			a.StatusWarn(err.Error())
			allApplied = false
			continue
		}
		if !a.EditorGroup.IsActiveEditableFile(path) {
			a.StatusWarn(i18n.Text("status.workspaceEditNotApplied"))
			allApplied = false
			continue
		}
		files[path] = true
		a.ApplyTextEdits(batch.Edits)

		if buf := a.EditorGroup.ActiveBuffer(); saveOnRename && a.Settings.LSP.SaveOnRename && buf != nil && !buf.ReadOnlyOnDisk {
			if a.EditorGroup.Save() {
				_, lang := a.editorPathLang()
				a.afterFileSave(path, lang)
			}
		}
	}

	a.EditorGroup.OpenFile(currentPath)
	return len(files), allApplied
}

func (a *App) applyInteractiveCodeActionEdit(path string, edit *lsp.WorkspaceEdit, openBuffers map[string]codeActionBufferSnapshot) bool {
	if a.EditorGroup.ActiveFilePath() != path {
		return false
	}
	batches, ok := a.workspaceTextEdits(edit)
	if !ok || len(batches) == 0 {
		return false
	}
	for _, batch := range batches {
		batchPath := URIToPath(batch.URI)
		captured, wasOpen := openBuffers[batchPath]
		current := a.EditorGroup.BufferForPath(batchPath)
		if (!wasOpen && current != nil) ||
			(wasOpen && (current != captured.Buffer || current == nil || current.Version() != captured.Version)) {
			return false
		}
	}
	_, allApplied := a.applyWorkspaceEditBatches(batches, false)
	return allApplied
}

func (a *App) snapshotCodeActionBuffers() map[string]codeActionBufferSnapshot {
	snapshots := make(map[string]codeActionBufferSnapshot)
	for _, path := range a.EditorGroup.OpenFilePaths() {
		if buf := a.EditorGroup.BufferForPath(path); buf != nil {
			snapshots[path] = codeActionBufferSnapshot{Buffer: buf, Version: buf.Version()}
		}
	}
	return snapshots
}

func (a *App) RequestInteractiveCodeActions() {
	if !a.EditorGroup.IsEditorActive() {
		a.StatusWarn(i18n.Text("status.noActiveEditor"))
		return
	}
	path, lang := a.editorPathLang()
	if path == "" {
		a.StatusWarn(i18n.Text("status.noFileOpen"))
		return
	}
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	buf := a.editorBuffer()
	if buf == nil {
		return
	}
	version := buf.Version()
	openBuffers := a.snapshotCodeActionBuffers()
	r := a.currentCodeActionRange(path)
	diagnostics := a.codeActionDiagnostics(path, serverKey, r)
	workDir := a.lspWorkDir(path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionsResult{
				Path: path, Server: serverKey, Version: version, Buffer: buf, OpenBuffers: openBuffers, Err: err,
			}))
			return
		}
		actions, err := client.CodeActionWithDiagnostics(FileURI(path), r, nil, diagnostics)
		if err != nil {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionsResult{
				Path: path, Server: serverKey, Version: version, Buffer: buf, OpenBuffers: openBuffers, Err: err,
			}))
			return
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionsResult{
			Path: path, Server: serverKey, Version: version, Buffer: buf, OpenBuffers: openBuffers,
			Actions: applicableCodeActions(actions),
		}))
	}()
}

func (a *App) codeActionDiagnostics(path, server string, requestRange lsp.Range) []lsp.Diagnostic {
	uiDiags := append(
		a.EditorGroup.DiagnosticsForSource(lspDiagnosticsSource(server), path),
		a.EditorGroup.DiagnosticsForSource(lspPullDiagnosticsSource(server), path)...,
	)
	diagnostics := make([]lsp.Diagnostic, 0, len(uiDiags))
	for _, d := range uiDiags {
		start := lsp.Position{Line: d.StartLine, Character: a.lspColumn(path, d.StartLine, d.StartCol)}
		end := lsp.Position{Line: d.EndLine, Character: a.lspColumn(path, d.EndLine, d.EndCol)}
		if !lspRangesIntersect(lsp.Range{Start: start, End: end}, requestRange) {
			continue
		}
		diagnostics = append(diagnostics, lsp.Diagnostic{
			Range:    lsp.Range{Start: start, End: end},
			Severity: int(d.Severity),
			Source:   d.Source,
			Message:  d.Message,
			Code:     d.Code,
		})
	}
	return diagnostics
}

func lspRangesIntersect(a, b lsp.Range) bool {
	compare := func(a, b lsp.Position) int {
		if a.Line < b.Line {
			return -1
		}
		if a.Line > b.Line {
			return 1
		}
		if a.Character < b.Character {
			return -1
		}
		if a.Character > b.Character {
			return 1
		}
		return 0
	}
	if compare(a.Start, a.End) == 0 {
		return compare(b.Start, a.Start) <= 0 && compare(a.Start, b.End) < 0
	}
	if compare(b.Start, b.End) == 0 {
		return compare(a.Start, b.Start) <= 0 && compare(b.Start, a.End) < 0
	}
	return compare(a.Start, b.End) < 0 && compare(b.Start, a.End) < 0
}

func (a *App) currentCodeActionRange(path string) lsp.Range {
	line, col := a.EditorGroup.ActiveCursor()
	startLine, startCol := line, col
	endLine, endCol := line, col
	if active, sl, sc, el, ec := a.EditorGroup.ActiveSelection(); active {
		startLine, startCol, endLine, endCol = sl, sc, el, ec
	}
	return lsp.Range{
		Start: lsp.Position{Line: startLine, Character: a.lspColumn(path, startLine, startCol)},
		End:   lsp.Position{Line: endLine, Character: a.lspColumn(path, endLine, endCol)},
	}
}

func applicableCodeActions(actions []lsp.CodeAction) []lsp.CodeAction {
	var applicable []lsp.CodeAction
	for _, action := range actions {
		if action.Edit == nil && (action.Command == nil || action.Command.Command == "") {
			continue
		}
		if action.Edit == nil {
			applicable = append(applicable, action)
			continue
		}
		batches, err := action.Edit.TextEdits()
		if len(batches) > 0 || errors.Is(err, lsp.ErrFileOperations) ||
			action.Command != nil && action.Command.Command != "" {
			applicable = append(applicable, action)
		}
	}
	return applicable
}

func (a *App) HandleCodeActionsResult(result *CodeActionsResult) {
	if result == nil || result.Path != a.EditorGroup.ActiveFilePath() ||
		result.Buffer != a.editorBuffer() || result.Buffer == nil || result.Version != result.Buffer.Version() {
		return
	}
	if result.Err != nil {
		a.StatusWarn(result.Err.Error())
		return
	}
	if len(result.Actions) == 0 {
		a.StatusNotify(i18n.Text("status.noCodeActionsAvailable"))
		return
	}

	items := make([]widgets.SelectItem, len(result.Actions))
	actions := make(map[string]lsp.CodeAction, len(result.Actions))
	for i, action := range result.Actions {
		id := strconv.Itoa(i)
		items[i] = widgets.SelectItem{ID: id, Label: action.Title}
		actions[id] = action
	}
	a.ShowSelectDialog(i18n.Text("command.codeActions"), items, func(id string) {
		if action, ok := actions[id]; ok && a.EditorGroup.ActiveFilePath() == result.Path &&
			a.editorBuffer() == result.Buffer && result.Buffer.Version() == result.Version {
			if action.Edit != nil {
				if !a.applyInteractiveCodeActionEdit(result.Path, action.Edit, result.OpenBuffers) {
					return
				}
				a.StatusNotify(i18n.Text("status.codeActionApplied"))
			}
			if action.Command != nil {
				a.executeCodeActionCommand(result.Server, result.Path, *action.Command, action.Edit == nil)
			}
		}
	}, nil)
}

func (a *App) executeCodeActionCommand(server, path string, command lsp.Command, notify bool) {
	workDir := a.lspWorkDir(path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(server, workDir)
		if err == nil {
			err = client.ExecuteCommand(command.Command, command.Arguments)
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionCommandResult{Err: err, Notify: notify}))
	}()
}

func (a *App) RequestReferences(path, lang string, line, col int) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	col = a.lspColumn(path, line, col)
	workDir := a.lspWorkDir(path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		locs, err := client.References(FileURI(path), line, col, true)
		if err != nil {
			slog.Error("lsp references", "err", err)
			return
		}
		if len(locs) > 0 {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&ReferencesResult{Locations: locs}))
		} else {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&ReferencesResult{}))
		}
	}()
}

func (a *App) ShowReferences(locs []lsp.Location) {
	items := make([]ui.ReferenceItem, 0, len(locs))
	documents := make(map[string][]string)
	for _, loc := range locs {
		path := URIToPath(loc.URI)
		lines, ok := documents[path]
		if !ok {
			lines = a.lspLines(path)
			documents[path] = lines
		}
		var text string
		if line := loc.Range.Start.Line; line >= 0 && line < len(lines) {
			text = lines[line]
		}
		items = append(items, ui.ReferenceItem{
			File: path,
			Line: loc.Range.Start.Line,
			Col:  lsp.UTF16ToRune(text, loc.Range.Start.Character),
			Text: strings.TrimSpace(text),
		})
	}
	a.References.SetItems(items)
	a.BottomPanel.SetActivePanel("references")
	if !a.ContentSplit.ShowBottom {
		a.ContentSplit.ShowBottom = true
	}
}

// RefreshSymbols re-requests the outline for the active file. Cheap no-op
// unless the Outline panel is the active sidebar panel.
func (a *App) RefreshSymbols() {
	if a.Symbols == nil || a.Sidebar.ActivePanel != "outline" {
		a.stopSymbolsTask()
		return
	}
	path, lang := a.editorPathLang()
	if path == "" || !a.EditorGroup.IsEditorActive() || a.EditorGroup.IsActiveVirtual() {
		a.stopSymbolsTask()
		a.Symbols.Path = ""
		a.Symbols.Clear()
		return
	}
	a.RequestDocumentSymbols(path, lang)
}

// ApplySymbols replaces the outline content and syncs its selection to the
// current cursor position.
func (a *App) ApplySymbols(symbols []lsp.DocumentSymbol) {
	a.Symbols.SetSymbols(symbols)
	line, _ := a.EditorGroup.ActiveCursor()
	a.Symbols.SelectNearest(line)
}

func lspStatusMessage(err error) string {
	msg := err.Error()
	if len(msg) > 60 {
		msg = msg[:60] + "…"
	}
	return i18n.Format("lsp.lsp", i18n.Args{"msg": msg})
}

// EnsureLSPOpen sends didOpen for a file that became active without going
// through OnFileOpen (tabs opened from CLI args before callbacks were wired).
// Servers like clangd reject requests for documents they never saw opened.
func (a *App) EnsureLSPOpen(path string) {
	if path == "" || !a.EditorGroup.IsEditorActive() || a.EditorGroup.IsActiveVirtual() {
		return
	}
	a.DocVersionsMu.Lock()
	_, opened := a.DocVersions[path]
	a.DocVersionsMu.Unlock()
	if opened {
		return
	}
	lang := a.EditorGroup.ActiveLanguage()
	a.NotifyLSPOpen(path, lang, a.EditorGroup.Editor.Buf.Lines)
}

// editorBufferVersion is the change counter of the buffer being edited, so a
// result computed for it can be dropped once the text has moved on.
func (a *App) editorBufferVersion() uint64 {
	if a.EditorGroup.Editor == nil {
		return 0
	}
	return a.EditorGroup.Editor.Buf.Version()
}

// editorBuffer is the buffer instance being edited, so a formatting result
// captured for it can be told apart from a reopened buffer of the same path.
func (a *App) editorBuffer() *buffer.Buffer {
	if a.EditorGroup.Editor == nil {
		return nil
	}
	return a.EditorGroup.Editor.Buf
}

// applyFormattingResult applies a formatting result to the file the server
// formatted. A result that arrives for another file, or for a buffer that
// changed while the server was formatting, is dropped: applying it would
// write the edits into the file the user is looking at instead (#876).
func (a *App) applyFormattingResult(v *FormattingResult) {
	editor := a.EditorGroup.Editor
	if editor == nil || len(v.Edits) == 0 {
		return
	}
	path, _ := a.editorPathLang()
	if path != v.Path {
		a.StatusWarn(i18n.Format("status.formatFileClosed", i18n.Args{"filename": filepath.Base(v.Path)}))
		return
	}
	if v.Buffer == nil || editor.Buf != v.Buffer {
		a.StatusWarn(i18n.Text("status.formatFileReopened"))
		return
	}
	if editor.Buf.Version() != v.Version {
		a.StatusWarn(i18n.Text("status.formatFileChanged"))
		return
	}
	a.ApplyTextEdits(v.Edits)
}

func (a *App) ApplyTextEdits(edits []lsp.TextEdit) {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	a.applyRuneTextEdits(runeTextEdits(a.EditorGroup.Editor.Buf.Lines, edits), nil)
}

func sortTextEdits(edits []lsp.TextEdit) {
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].Range.Start.Line != edits[j].Range.Start.Line {
			return edits[i].Range.Start.Line > edits[j].Range.Start.Line
		}
		return edits[i].Range.Start.Character > edits[j].Range.Start.Character
	})
}

// Reverse equal-position groups so descending application preserves LSP array order.
func reverseSamePositionTextEdits(edits []lsp.TextEdit) {
	for start := 0; start < len(edits); {
		end := start + 1
		for end < len(edits) && edits[end].Range.Start == edits[start].Range.Start {
			end++
		}
		slices.Reverse(edits[start:end])
		start = end
	}
}

func (a *App) applyRuneTextEdits(edits []lsp.TextEdit, cursor *lsp.Position) {
	editor := a.EditorGroup.Editor
	sortTextEdits(edits)
	reverseSamePositionTextEdits(edits)

	// One undo step for the whole batch: a rename, a format or a code action
	// is a single change, not one per edited range (#876).
	if editor.Undo != nil {
		editor.Undo.BeginTransaction()
		defer editor.Undo.EndTransaction()
	}
	for _, edit := range edits {
		sl, sc := edit.Range.Start.Line, edit.Range.Start.Character
		el, ec := edit.Range.End.Line, edit.Range.End.Character

		if sl != el || sc != ec {
			editor.ExecCommand(&undo.DeleteSelectionCommand{
				StartLine: sl, StartCol: sc,
				EndLine: el, EndCol: ec,
			})
		}
		if edit.NewText != "" {
			suffix := ""
			if sl < len(editor.Buf.Lines) {
				runes := []rune(editor.Buf.Lines[sl])
				if sc < len(runes) {
					suffix = string(runes[sc:])
				}
			}
			editor.ExecCommand(&undo.PasteCommand{
				Line: sl, Col: sc,
				Text: edit.NewText, Suffix: suffix,
			})
		}
	}
	if cursor != nil {
		editor.Cursor.Line, editor.Cursor.Col = cursor.Line, cursor.Character
		a.EditorGroup.ScrollToCursor()
	}
	editor.FlushOnChange()
}

func (a *App) wordAtCursor() string {
	if !a.EditorGroup.IsEditorActive() {
		return ""
	}
	editor := a.EditorGroup.Editor
	line := editor.Cursor.Line
	col := editor.Cursor.Col
	if line >= len(editor.Buf.Lines) {
		return ""
	}
	runes := []rune(editor.Buf.Lines[line])
	if col > len(runes) {
		col = len(runes)
	}
	start := col
	for start > 0 && isIdentRune(runes[start-1]) {
		start--
	}
	end := col
	for end < len(runes) && isIdentRune(runes[end]) {
		end++
	}
	if start == end {
		return ""
	}
	return string(runes[start:end])
}

func isIdentRune(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func (a *App) lspResolve(path, lang string) (serverKey, languageID string, ok bool) {
	if a.LspManager == nil || !a.Settings.LSP.IsEnabled() {
		return "", "", false
	}
	return a.LspManager.ResolveLanguage(path, lang)
}

func (a *App) RequestDefinition(path, lang string, line, col int) {
	a.requestLocation("textDocument/definition", path, lang, line, col)
}

func (a *App) RequestImplementation(path, lang string, line, col int) {
	a.requestLocation("textDocument/implementation", path, lang, line, col)
}

func (a *App) RequestTypeDefinition(path, lang string, line, col int) {
	a.requestLocation("textDocument/typeDefinition", path, lang, line, col)
}

func (a *App) requestLocation(method, path, lang string, line, col int) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	col = a.lspColumn(path, line, col)
	workDir := a.lspWorkDir(path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		var locs []lsp.Location
		var reqErr error
		switch method {
		case "textDocument/definition":
			locs, reqErr = client.Definition(FileURI(path), line, col)
		case "textDocument/implementation":
			locs, reqErr = client.Implementation(FileURI(path), line, col)
		case "textDocument/typeDefinition":
			locs, reqErr = client.TypeDefinition(FileURI(path), line, col)
		}
		if reqErr != nil {
			slog.Error("lsp "+method, "err", reqErr)
			return
		}
		if len(locs) > 0 {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&LocationResult{Locations: locs}))
		}
	}()
}

func (a *App) lspWorkDir(path string) string {
	workDir := a.Workspace.Primary()
	if folder := a.Workspace.FolderForFile(path); folder != nil {
		workDir = folder.Path
	}
	if workDir == "" && path != "" {
		// Single-file launch: no workspace folder exists. Root the server at
		// the file's directory instead of an empty (invalid) rootURI.
		workDir = filepath.Dir(path)
	}
	return workDir
}

func (a *App) NotifyLSPOpen(path, lang string, lines []string) {
	serverKey, langID, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}

	serverCfg := a.LspManager.ServerConfig(serverKey)
	if len(serverCfg.Command) > 0 {
		if _, err := exec.LookPath(serverCfg.Command[0]); err != nil {
			if !a.LspNotified[serverKey] && a.Settings.LSP.ShouldNotifyAvailability() {
				a.LspNotified[serverKey] = true
				msg := i18n.Format("lsp.installServerHint", i18n.Args{"lang": lang})
				anchor := serverKey
				a.Status.SetNotificationWithAction(msg, view.NotifyWarning, 10*time.Second, i18n.Text("status.docs"), func() {
					OpenURL("https://tttedit.dev/guides/lsp/#" + anchor)
				})
				a.Status.SecondaryLabel = i18n.Text("lsp.donTShowAgain")
				a.Status.SecondaryAction = func() {
					v := false
					a.Settings.LSP.NotifyAvailability = &v
					config.SaveSettings(*a.Settings)
				}
				time.AfterFunc(10*time.Second, func() {
					a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
				})
			}
			return
		}
	}

	text := strings.Join(lines, "\n")
	workDir := a.lspWorkDir(path)
	sent := make(chan struct{})
	a.DocVersionsMu.Lock()
	a.DocVersions[path] = 1
	a.lspSent[path] = sent
	a.DocVersionsMu.Unlock()
	slog.Debug("lsp didOpen", "path", path, "language", langID)
	go func() {
		defer close(sent)
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return
		}
		client.DidOpen(FileURI(path), langID, text)
	}()
}

// The returned channel closes once the didChange notification has been sent,
// so callers that must not race ahead of it (e.g. a signature-help request
// for the same keystroke) can wait on it.
func (a *App) NotifyLSPChange(path, lang string, lines []string) <-chan struct{} {
	done := make(chan struct{})
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		close(done)
		return done
	}
	text := strings.Join(lines, "\n")
	workDir := a.lspWorkDir(path)
	a.DocVersionsMu.Lock()
	a.DocVersions[path]++
	version := a.DocVersions[path]
	epoch := a.lspEpochs[serverKey]
	previous := a.lspSent[path]
	a.lspSent[path] = done
	a.DocVersionsMu.Unlock()
	go func() {
		defer close(done)
		// Each change carries the full text, so one sent out of order would
		// leave the server on an older version.
		if previous != nil {
			<-previous
		}
		// A restart reopens the document with its current text; a change from
		// before it would reach a server that has not opened the document yet.
		a.DocVersionsMu.Lock()
		restarted := a.lspEpochs[serverKey] != epoch
		a.DocVersionsMu.Unlock()
		if restarted {
			return
		}
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return
		}
		client.DidChange(FileURI(path), text, version)
	}()
	return done
}

func (a *App) NotifyLSPSave(path, lang string, lines []string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	text := strings.Join(lines, "\n")
	workDir := a.lspWorkDir(path)
	sent := make(chan struct{})
	a.DocVersionsMu.Lock()
	previous := a.lspSent[path]
	a.lspSent[path] = sent
	a.DocVersionsMu.Unlock()
	go func() {
		defer close(sent)
		if previous != nil {
			<-previous
		}
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return
		}
		client.DidSave(FileURI(path), text)
	}()
}

func (a *App) NotifyLSPClose(path, lang string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	workDir := a.lspWorkDir(path)
	a.DocVersionsMu.Lock()
	delete(a.DocVersions, path)
	delete(a.lspSent, path)
	a.DocVersionsMu.Unlock()
	// Reopening starts the version over at 1, which would match this entry.
	delete(a.lastPulled, path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return
		}
		client.DidClose(FileURI(path))
	}()
}
