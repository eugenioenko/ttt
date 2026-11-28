package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func (a *App) RunCodeActionsOnSave(path, lang string) {
	if len(a.Settings.LSP.CodeActionsOnSave) == 0 {
		return
	}
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	client := a.LspManager.ReadyClient(serverKey, a.lspWorkDirFor(path, lang))
	if client == nil {
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

func (a *App) RequestCodeAction(path, lang, kind string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	workDir := a.lspWorkDirFor(path, lang)
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
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionResult{Path: path, Edit: edit}))
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

func (a *App) applyInteractiveCodeActionEdit(path string, edit *lsp.WorkspaceEdit, openBuffers map[string]codeActionBufferSnapshot) (bool, bool) {
	if a.EditorGroup.ActiveFilePath() != path {
		return false, false
	}
	batches, ok := a.workspaceTextEdits(edit)
	if !ok {
		return false, false
	}
	if len(batches) == 0 {
		return false, true
	}
	for _, batch := range batches {
		batchPath := URIToPath(batch.URI)
		captured, wasOpen := openBuffers[batchPath]
		current := a.editorBufferForPath(batchPath)
		if (!wasOpen && current != nil) ||
			(wasOpen && (current != captured.Buffer || current == nil || current.Version() != captured.Version)) {
			return false, false
		}
	}
	filesCount, allApplied := a.applyWorkspaceEditBatches(batches, false)
	return filesCount > 0 && allApplied, allApplied
}

func (a *App) snapshotCodeActionBuffers() map[string]codeActionBufferSnapshot {
	snapshots := make(map[string]codeActionBufferSnapshot)
	for _, group := range a.editorGroups() {
		for _, path := range group.OpenFilePaths() {
			if buf := a.editorBufferForPath(path); buf != nil {
				snapshots[path] = codeActionBufferSnapshot{Buffer: buf, Version: buf.Version()}
			}
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
	workDir := a.lspWorkDirFor(path, lang)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionsResult{
				Path: path, Server: serverKey, Version: version, Buffer: buf, OpenBuffers: openBuffers, Err: err,
			}))
			return
		}
		actions, err := client.CodeActionWithDiagnostics(FileURI(path), r, nil, diagnostics)
		if err != nil {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionsResult{
				Path: path, Server: serverKey, Version: version, Buffer: buf, OpenBuffers: openBuffers, Err: err,
			}))
			return
		}
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionsResult{
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
			applied := false
			if action.Edit != nil {
				var ok bool
				applied, ok = a.applyInteractiveCodeActionEdit(result.Path, action.Edit, result.OpenBuffers)
				if !ok {
					return
				}
				if applied {
					a.StatusNotify(i18n.Text("status.codeActionApplied"))
				}
			}
			if action.Command != nil {
				a.executeCodeActionCommand(result.Server, result.Path, *action.Command, !applied)
			}
		}
	}, nil)
}

type codeActionCommandClient interface {
	ExecuteCommand(string, []json.RawMessage) error
}

func (a *App) codeActionClient(server, workDir string) (codeActionCommandClient, error) {
	if a.lookupCodeActionClient != nil {
		return a.lookupCodeActionClient(server, workDir)
	}
	return a.LspManager.ClientForLanguage(server, workDir)
}

func (a *App) executeCodeActionCommand(server, path string, command lsp.Command, notify bool) {
	workDir := a.lspWorkDirForServer(path, server)
	go func() {
		client, err := a.codeActionClient(server, workDir)
		if err == nil {
			err = client.ExecuteCommand(command.Command, command.Arguments)
		}
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&CodeActionCommandResult{Err: err, Notify: notify}))
	}()
}
