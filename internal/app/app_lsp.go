package app

import (
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v2"
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
	ac.OnSelect = func(item ui.CompletionItem) {
		a.resolveAndInsert(item)
		a.DismissAutocomplete()
	}
	ac.OnDismiss = func() {
		a.DismissAutocomplete()
	}
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

func (a *App) isCompletionTrigger(ch string) bool {
	for _, tc := range a.CompletionTriggers {
		if ch == tc {
			return true
		}
	}
	return false
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
		path := a.EditorGroup.ActiveFilePath()
		lang := ""
		if a.EditorGroup.Editor != nil && a.EditorGroup.Editor.Highlighter != nil {
			lang = a.EditorGroup.Editor.Highlighter.Language()
		}
		serverKey, _, ok := a.lspResolve(path, lang)
		if ok {
			workDir := a.lspWorkDir(path)
			client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
			if err == nil {
				resolved, err := client.ResolveCompletion(*lspItem)
				if err == nil && resolved != nil {
					for _, edit := range resolved.AdditionalTextEdits {
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
	if lspItem != nil {
		textEdit = lspItem.TextEdit
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
		text = textEdit.NewText
		startLine = textEdit.Range.Start.Line
		startCol = textEdit.Range.Start.Character
		endLine = textEdit.Range.End.Line
		endCol = textEdit.Range.End.Character
	} else {
		text = item.InsertText
		if text == "" {
			text = item.Label
		}
		startLine, startCol, endCol = a.identStart()
		endLine = startLine
	}

	if startLine != endLine || startCol != endCol {
		editor.ExecCommand(&undo.DeleteSelectionCommand{
			StartLine: startLine, StartCol: startCol,
			EndLine: endLine, EndCol: endCol,
		})
	}
	editor.ExecCommand(&undo.InsertStringCommand{
		Line: startLine, Col: startCol, Text: text,
	})
	editor.Cursor.Line = startLine
	editor.Cursor.Col = startCol + len([]rune(text))

	for i := len(item.AdditionalEdits) - 1; i >= 0; i-- {
		edit := item.AdditionalEdits[i]
		linesBefore := len(editor.Buf.Lines)

		if edit.StartLine != edit.EndLine || edit.StartCol != edit.EndCol {
			editor.ExecCommand(&undo.DeleteSelectionCommand{
				StartLine: edit.StartLine, StartCol: edit.StartCol,
				EndLine: edit.EndLine, EndCol: edit.EndCol,
			})
		}
		if edit.NewText != "" {
			suffix := ""
			if edit.StartLine < len(editor.Buf.Lines) {
				runes := []rune(editor.Buf.Lines[edit.StartLine])
				if edit.StartCol < len(runes) {
					suffix = string(runes[edit.StartCol:])
				}
			}
			editor.ExecCommand(&undo.PasteCommand{
				Line: edit.StartLine, Col: edit.StartCol,
				Text: edit.NewText, Suffix: suffix,
			})
		}

		linesAdded := len(editor.Buf.Lines) - linesBefore
		if linesAdded != 0 && edit.StartLine <= editor.Cursor.Line {
			editor.Cursor.Line += linesAdded
		}
	}
	editor.FlushOnChange()
}

func (a *App) ScheduleAutocomplete() {
	if !a.Settings.Autocomplete.Enabled || !a.Settings.Autocomplete.AutoSuggest {
		return
	}
	if a.AutocompleteTimer != nil {
		a.AutocompleteTimer.Stop()
	}
	delay := time.Duration(a.Settings.Autocomplete.Debounce) * time.Millisecond
	a.AutocompleteTimer = time.AfterFunc(delay, func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(&AutocompleteTrigger{}))
	})
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

func (a *App) CheckSignatureHelpTrigger() {
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
	path := a.EditorGroup.ActiveFilePath()
	lang := ""
	if editor.Highlighter != nil {
		lang = editor.Highlighter.Language()
	}
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	triggers := a.LspManager.SignatureHelpTriggerCharacters(serverKey)
	for _, tc := range triggers {
		if ch == tc {
			a.RequestSignatureHelp(path, lang, line, col)
			return
		}
	}
}

func (a *App) RequestSignatureHelp(path, lang string, line, col int) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	workDir := a.lspWorkDir(path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		sig, err := client.SignatureHelp(FileURI(path), line, col)
		if err != nil {
			slog.Error("lsp signatureHelp", "err", err)
			return
		}
		if sig != nil && len(sig.Signatures) > 0 {
			result := LspToSignatureHelpResult(sig)
			if result.Label != "" {
				slog.Debug("lsp signature help response", "label", result.Label)
				a.Screen.PostEvent(tcell.NewEventInterrupt(result))
			}
		}
	}()
}

func (a *App) ShowSignatureHelp(result *SignatureHelpResult) {
	w := ui.NewSignatureHelpWidget(result.Label, result.ParamStart, result.ParamEnd)
	a.EditorGroup.SignatureHelp = w
}

func (a *App) DismissSignatureHelp() {
	a.EditorGroup.SignatureHelp = nil
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
			a.StatusWarn(lang + " language server is not configured")
		}
		return
	}
	workDir := a.lspWorkDir(path)
	tabSize, insertSpaces := a.editorTabSize()
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
			a.Screen.PostEvent(tcell.NewEventInterrupt(&FormattingResult{Edits: edits}))
		}
	}()
}

func (a *App) RequestRangeFormatting(path, lang string, startLine, startCol, endLine, endCol int) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(lang + " language server is not configured")
		}
		return
	}
	workDir := a.lspWorkDir(path)
	tabSize, insertSpaces := a.editorTabSize()
	r := lsp.Range{
		Start: lsp.Position{Line: startLine, Character: startCol},
		End:   lsp.Position{Line: endLine, Character: endCol},
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
			a.Screen.PostEvent(tcell.NewEventInterrupt(&FormattingResult{Edits: edits}))
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
		if action.Edit != nil && len(action.Edit.Changes) > 0 {
			for uri, edits := range action.Edit.Changes {
				if URIToPath(uri) == path {
					a.ApplyTextEdits(edits)
				}
			}
		}
	}
}

func (a *App) RequestCodeAction(path, lang, kind string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(lang + " language server is not configured")
		}
		return
	}
	workDir := a.lspWorkDir(path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
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
		actions, err := client.CodeAction(FileURI(path), fullRange, []string{kind})
		if err != nil {
			slog.Error("lsp codeAction", "err", err)
			return
		}
		for _, action := range actions {
			if action.Edit != nil && len(action.Edit.Changes) > 0 {
				a.Screen.PostEvent(tcell.NewEventInterrupt(&FormattingResult{Edits: action.Edit.Changes[FileURI(path)]}))
				return
			}
		}
	}()
}

type CodeActionsResult struct {
	Actions []lsp.CodeAction
}

func (a *App) RequestCodeActionsAtCursor(path, lang string, line, col int) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		a.StatusNotify("No code actions available")
		return
	}
	workDir := a.lspWorkDir(path)

	sel := a.EditorGroup.Editor.Selection
	var r lsp.Range
	if sel != nil && sel.Active {
		start, end := sel.Range(line, col)
		r = lsp.Range{
			Start: lsp.Position{Line: start.Line, Character: start.Col},
			End:   lsp.Position{Line: end.Line, Character: end.Col},
		}
	} else {
		r = lsp.Range{
			Start: lsp.Position{Line: line, Character: col},
			End:   lsp.Position{Line: line, Character: col},
		}
	}

	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		actions, err := client.CodeAction(FileURI(path), r, nil)
		if err != nil {
			slog.Error("lsp codeAction", "err", err)
			return
		}
		if len(actions) > 0 {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionsResult{Actions: actions}))
		} else {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionsResult{}))
		}
	}()
}

func (a *App) ShowCodeActionsMenu(actions []lsp.CodeAction) {
	if len(actions) == 0 {
		a.StatusNotify("No code actions available")
		return
	}
	path := a.EditorGroup.ActiveFilePath()
	var items []ui.ContextMenuItem
	for _, action := range actions {
		items = append(items, ui.ContextMenuItem{Label: action.Title, Command: action.Title})
	}
	x := a.EditorGroup.Editor.CursorX
	y := a.EditorGroup.Editor.CursorY + 1
	menu := ui.NewContextMenuWidget(items, x, y)
	menu.Borders = a.Borders
	menu.OnExec = func(cmd string) {
		a.Root.PopOverlay()
		for _, action := range actions {
			if action.Title == cmd {
				if action.Edit != nil && len(action.Edit.Changes) > 0 {
					for uri, edits := range action.Edit.Changes {
						if URIToPath(uri) == path {
							a.ApplyTextEdits(edits)
						}
					}
				}
				break
			}
		}
	}
	menu.OnDismiss = func() {
		a.Root.PopOverlay()
	}
	a.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	a.Root.SetFocus(menu)
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
			a.StatusWarn(lang + " language server is not configured")
		}
		return
	}
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
		if edit != nil && len(edit.Changes) > 0 {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&RenameResult{Edit: edit}))
		}
	}()
}

func (a *App) ApplyWorkspaceEdit(edit *lsp.WorkspaceEdit) {
	currentPath := a.EditorGroup.ActiveFilePath()

	for uri, edits := range edit.Changes {
		path := URIToPath(uri)
		a.EditorGroup.OpenFile(path)
		a.ApplyTextEdits(edits)

		if a.Settings.LSP.SaveOnRename {
			a.EditorGroup.Save()
			if a.EditorGroup.Editor != nil && a.EditorGroup.Editor.Highlighter != nil {
				lang := a.EditorGroup.Editor.Highlighter.Language()
				text := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n")
				a.NotifyLSPSave(path, lang, text)
			}
		}
	}

	a.EditorGroup.OpenFile(currentPath)
	fileCount := len(edit.Changes)
	a.StatusNotify(fmt.Sprintf("Renamed across %d file(s)", fileCount))
}

func (a *App) RequestReferences(path, lang string, line, col int) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(lang + " language server is not configured")
		}
		return
	}
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
	for _, loc := range locs {
		path := URIToPath(loc.URI)
		text := ReadLineFromFile(path, loc.Range.Start.Line)
		items = append(items, ui.ReferenceItem{
			File: path,
			Line: loc.Range.Start.Line,
			Col:  loc.Range.Start.Character,
			Text: strings.TrimSpace(text),
		})
	}
	a.References.SetItems(items)
	a.BottomPanel.SetActivePanel("references")
	if !a.ContentSplit.ShowBottom {
		a.ContentSplit.ShowBottom = true
	}
}

func (a *App) ApplyTextEdits(edits []lsp.TextEdit) {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	editor := a.EditorGroup.Editor

	sort.Slice(edits, func(i, j int) bool {
		if edits[i].Range.Start.Line != edits[j].Range.Start.Line {
			return edits[i].Range.Start.Line > edits[j].Range.Start.Line
		}
		return edits[i].Range.Start.Character > edits[j].Range.Start.Character
	})

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

func (a *App) RequestCompletions(path, lang string, line, col int, triggerChar string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	workDir := a.lspWorkDir(path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		var ctx *lsp.CompletionContext
		if triggerChar != "" {
			for _, tc := range client.CompletionTriggerCharacters() {
				if triggerChar == tc {
					ctx = &lsp.CompletionContext{
						TriggerKind:      lsp.CompletionTriggerTriggerCharacter,
						TriggerCharacter: triggerChar,
					}
					break
				}
			}
		}
		if ctx == nil {
			ctx = &lsp.CompletionContext{TriggerKind: lsp.CompletionTriggerInvoked}
		}
		slog.Debug("lsp completion request", "path", path, "line", line, "col", col, "triggerChar", triggerChar)
		items, err := client.Completion(FileURI(path), line, col, ctx)
		if err != nil {
			slog.Error("lsp completion", "err", err)
			return
		}
		slog.Debug("lsp completion response", "count", len(items))
		uiItems := LspToUICompletions(items)
		if len(uiItems) > 0 {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&CompletionResult{
				Items:        uiItems,
				LspItems:     items,
				TriggerChars: client.CompletionTriggerCharacters(),
			}))
		}
	}()
}

func (a *App) RequestHover(path, lang string, line, col, anchorX, anchorY int) {
	diagText := ""
	if a.EditorGroup.Editor != nil {
		if d := a.EditorGroup.Editor.DiagnosticAt(line, col); d != nil {
			diagText = d.Message
		}
	}

	gen := a.HoverGen
	post := func(text string) {
		a.Screen.PostEvent(tcell.NewEventInterrupt(&HoverResult{Text: text, AnchorX: anchorX, AnchorY: anchorY, Gen: gen}))
	}

	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if diagText != "" {
			post(diagText)
		}
		return
	}
	workDir := a.lspWorkDir(path)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			if diagText != "" {
				post(diagText)
			}
			return
		}
		hover, err := client.Hover(FileURI(path), line, col)
		if err != nil {
			slog.Error("lsp hover", "err", err)
			if diagText != "" {
				post(diagText)
			}
			return
		}
		text := ""
		if hover != nil {
			text = hover.Contents.Value
			slog.Debug("lsp hover response", "length", len(text))
		}
		if diagText != "" {
			if text != "" {
				text = diagText + "\n---\n" + text
			} else {
				text = diagText
			}
		}
		if text != "" {
			post(text)
		}
	}()
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
			a.StatusWarn(lang + " language server is not configured")
		}
		return
	}
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
	return workDir
}

func (a *App) NotifyLSPOpen(path, lang, text string) {
	serverKey, langID, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}

	serverCfg := a.LspManager.ServerConfig(serverKey)
	if len(serverCfg.Command) > 0 {
		if _, err := exec.LookPath(serverCfg.Command[0]); err != nil {
			if !a.LspNotified[serverKey] && a.Settings.LSP.ShouldNotifyAvailability() {
				a.LspNotified[serverKey] = true
				msg := fmt.Sprintf("%s autocomplete support is available. Click Docs for installation instructions.", lang)
				anchor := serverKey
				a.Status.SetNotificationWithAction(msg, view.NotifyWarning, 10*time.Second, "Docs", func() {
					OpenURL("https://tttedit.dev/guides/lsp/#" + anchor)
				})
				a.Status.SecondaryLabel = "Don't show again"
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

	workDir := a.lspWorkDir(path)
	a.DocVersionsMu.Lock()
	a.DocVersions[path] = 1
	a.DocVersionsMu.Unlock()
	slog.Debug("lsp didOpen", "path", path, "language", langID)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return
		}
		client.DidOpen(FileURI(path), langID, text)
	}()
}

func (a *App) NotifyLSPChange(path, lang, text string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	workDir := a.lspWorkDir(path)
	a.DocVersionsMu.Lock()
	a.DocVersions[path]++
	version := a.DocVersions[path]
	a.DocVersionsMu.Unlock()
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return
		}
		client.DidChange(FileURI(path), text, version)
	}()
}

func (a *App) NotifyLSPSave(path, lang, text string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	workDir := a.lspWorkDir(path)
	go func() {
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
	a.DocVersionsMu.Unlock()
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return
		}
		client.DidClose(FileURI(path))
	}()
}
