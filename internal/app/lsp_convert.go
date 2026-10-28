package app

import (
	"encoding/json"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

type LocationResult struct {
	Locations []lsp.Location
}

type DiagnosticsResult struct {
	Server      string
	Path        string
	Diagnostics []lsp.Diagnostic
}

type SignatureHelpResult struct {
	Label      string
	ParamStart int
	ParamEnd   int
}

type FormattingResult struct {
	// Path is the file the server formatted and Version the buffer it
	// formatted, so a result that arrives for another file, or for a buffer
	// that changed in the meantime, is not applied to the wrong one (#876).
	// Buffer identifies the buffer instance: closing and reopening the same
	// path starts a new buffer whose version counter can match the old result
	// (#1084).
	Path    string
	Version uint64
	Buffer  *buffer.Buffer
	Edits   []lsp.TextEdit
}

type ReferencesResult struct {
	Locations []lsp.Location
}

type RenameResult struct {
	Edit *lsp.WorkspaceEdit
}

type CodeActionResult struct {
	Path string
	Edit *lsp.WorkspaceEdit
}

func FileURI(path string) string { return lsp.FileURI(path) }

func URIToPath(uri string) string { return lsp.URIToPath(uri) }

func LspToUICompletions(items []lsp.CompletionItem, lines []string) []ui.CompletionItem {
	result := make([]ui.CompletionItem, 0, len(items))
	for _, item := range items {
		uiItem := ui.CompletionItem{
			Label:      item.Label,
			Detail:     item.Detail,
			InsertText: item.InsertText,
			FilterText: item.FilterText,
			SortText:   item.SortText,
			Kind:       lspKindToUI(item.Kind),
		}
		if uiItem.InsertText == "" && item.TextEdit != nil {
			uiItem.InsertText = item.TextEdit.NewText
		}
		for _, edit := range item.AdditionalTextEdits {
			edit.Range = lsp.RangeToRunes(lines, edit.Range)
			uiItem.AdditionalEdits = append(uiItem.AdditionalEdits, ui.AdditionalEdit{
				StartLine: edit.Range.Start.Line,
				StartCol:  edit.Range.Start.Character,
				EndLine:   edit.Range.End.Line,
				EndCol:    edit.Range.End.Character,
				NewText:   edit.NewText,
			})
		}
		result = append(result, uiItem)
	}
	return result
}

func lspKindToUI(kind lsp.CompletionItemKind) ui.CompletionKind {
	switch kind {
	case lsp.CIKFunction, lsp.CIKConstructor:
		return ui.CompletionFunction
	case lsp.CIKMethod:
		return ui.CompletionMethod
	case lsp.CIKVariable:
		return ui.CompletionVariable
	case lsp.CIKConstant, lsp.CIKEnumMember:
		return ui.CompletionConstant
	case lsp.CIKClass, lsp.CIKInterface, lsp.CIKStruct, lsp.CIKEnum, lsp.CIKTypeParameter:
		return ui.CompletionType
	case lsp.CIKField, lsp.CIKProperty:
		return ui.CompletionField
	case lsp.CIKKeyword:
		return ui.CompletionKeyword
	case lsp.CIKSnippet:
		return ui.CompletionSnippet
	case lsp.CIKModule:
		return ui.CompletionModule
	default:
		return ui.CompletionVariable
	}
}

func LspToUIDiagnostics(diags []lsp.Diagnostic, lines []string) []ui.Diagnostic {
	result := make([]ui.Diagnostic, len(diags))
	for i, d := range diags {
		d.Range = lsp.RangeToRunes(lines, d.Range)
		result[i] = ui.Diagnostic{
			StartLine: d.Range.Start.Line,
			StartCol:  d.Range.Start.Character,
			EndLine:   d.Range.End.Line,
			EndCol:    d.Range.End.Character,
			Severity:  ui.DiagnosticSeverity(d.Severity),
			Message:   d.Message,
			Source:    d.Source,
		}
	}
	return result
}

func LspToSignatureHelpResult(sig *lsp.SignatureHelp) *SignatureHelpResult {
	idx := sig.ActiveSignature
	if idx < 0 || idx >= len(sig.Signatures) {
		idx = 0
	}
	info := sig.Signatures[idx]
	result := &SignatureHelpResult{Label: info.Label}

	paramIdx := sig.ActiveParameter
	if paramIdx >= 0 && paramIdx < len(info.Parameters) {
		param := info.Parameters[paramIdx]
		var offsets [2]int
		if err := json.Unmarshal(param.Label, &offsets); err == nil {
			runes := []rune(info.Label)
			result.ParamStart = len(string(runes[:lsp.UTF16ToRune(info.Label, offsets[0])]))
			result.ParamEnd = len(string(runes[:lsp.UTF16ToRune(info.Label, offsets[1])]))
		} else {
			var label string
			if err := json.Unmarshal(param.Label, &label); err == nil {
				start := strings.Index(info.Label, label)
				if start >= 0 {
					result.ParamStart = start
					result.ParamEnd = start + len(label)
				}
			}
		}
	}
	return result
}
