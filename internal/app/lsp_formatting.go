package app

import (
	"log/slog"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/lsp"

	"github.com/gdamore/tcell/v3"
)

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
	workDir := a.lspWorkDirFor(path, lang)
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
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&FormattingResult{
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
	workDir := a.lspWorkDirFor(path, lang)
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
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&FormattingResult{
				Path: path, Version: version, Buffer: buf, Edits: edits,
			}))
		}
	}()
}

func (a *App) FormatOnSave(path, lang string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	tabSize, insertSpaces := a.editorTabSize()
	client := a.LspManager.ReadyClient(serverKey, a.lspWorkDirFor(path, lang))
	if client == nil {
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
