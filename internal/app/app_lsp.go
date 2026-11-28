package app

import (
	"github.com/eugenioenko/ttt/internal/core/buffer"
)

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

func (a *App) applyLSPDiagnostics(v *DiagnosticsResult) {
	if !a.lspDiagnosticsStillCurrent(v.Path, v.Root) {
		return
	}
	source := lspDiagnosticsSource(v.Server)
	a.recordLSPDiagRoot(v.Server, source, v.Path, v.Root, len(v.Diagnostics) > 0)
	var lines []string
	if len(v.Diagnostics) > 0 {
		lines = a.lspLines(v.Path)
	}
	for _, group := range a.editorGroups() {
		group.SetDiagnosticsSource(source, v.Path, LspToUIDiagnostics(v.Diagnostics, lines))
	}
}
