package app

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"

	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

type CompletionResult struct {
	Items        []ui.CompletionItem
	LspItems     []lsp.CompletionItem
	TriggerChars []string
}

type LocationResult struct {
	Locations []lsp.Location
}

type HoverResult struct {
	Text    string
	AnchorX int
	AnchorY int
	Gen     uint64
}

type AutocompleteTrigger struct {
	TriggerChar string
}

type DiagnosticsResult struct {
	Path        string
	Diagnostics []ui.Diagnostic
}


type SignatureHelpResult struct {
	Label      string
	ParamStart int
	ParamEnd   int
}

type FormattingResult struct {
	Edits []lsp.TextEdit
}

type ReferencesResult struct {
	Locations []lsp.Location
}

type RenameResult struct {
	Edit *lsp.WorkspaceEdit
}

func ReadLineFromFile(path string, line int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for i := 0; scanner.Scan(); i++ {
		if i == line {
			return scanner.Text()
		}
	}
	return ""
}

func FileURI(path string) string {
	return "file://" + path
}

func URIToPath(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}

func LspToUICompletions(items []lsp.CompletionItem) []ui.CompletionItem {
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

func LspToUIDiagnostics(diags []lsp.Diagnostic) []ui.Diagnostic {
	result := make([]ui.Diagnostic, len(diags))
	for i, d := range diags {
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
			result.ParamStart = offsets[0]
			result.ParamEnd = offsets[1]
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
