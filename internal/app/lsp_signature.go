package app

import (
	"github.com/eugenioenko/ttt/internal/ui"
)

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
