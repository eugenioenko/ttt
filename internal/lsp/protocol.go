package lsp

import "encoding/json"

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      string             `json:"rootUri"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Completion         *CompletionClientCapabilities         `json:"completion,omitempty"`
	PublishDiagnostics *PublishDiagnosticsClientCapabilities `json:"publishDiagnostics,omitempty"`
}

type PublishDiagnosticsClientCapabilities struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
}

type CompletionClientCapabilities struct {
	CompletionItem *CompletionItemClientCapabilities `json:"completionItem,omitempty"`
}

type CompletionItemClientCapabilities struct {
	SnippetSupport bool                          `json:"snippetSupport"`
	ResolveSupport *CompletionItemResolveSupport `json:"resolveSupport,omitempty"`
}

type CompletionItemResolveSupport struct {
	Properties []string `json:"properties"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	CompletionProvider              *CompletionOptions       `json:"completionProvider,omitempty"`
	SignatureHelpProvider           *SignatureHelpOptions    `json:"signatureHelpProvider,omitempty"`
	TextDocumentSync                *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	DocumentFormattingProvider      BoolOrObject             `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider BoolOrObject             `json:"documentRangeFormattingProvider,omitempty"`
}

type SignatureHelpOptions struct {
	TriggerCharacters   []string `json:"triggerCharacters,omitempty"`
	RetriggerCharacters []string `json:"retriggerCharacters,omitempty"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type BoolOrObject bool

func (b *BoolOrObject) UnmarshalJSON(data []byte) error {
	var v bool
	if err := json.Unmarshal(data, &v); err == nil {
		*b = BoolOrObject(v)
		return nil
	}
	*b = true
	return nil
}

type TextDocumentSyncOptions struct {
	OpenClose bool `json:"openClose,omitempty"`
	Change    int  `json:"change,omitempty"`
}

func (o *TextDocumentSyncOptions) UnmarshalJSON(data []byte) error {
	var kind int
	if err := json.Unmarshal(data, &kind); err == nil {
		o.Change = kind
		o.OpenClose = true
		return nil
	}
	type alias TextDocumentSyncOptions
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*o = TextDocumentSyncOptions(a)
	return nil
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         string                 `json:"text,omitempty"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type SignatureHelpParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

type SignatureInformation struct {
	Label         string                 `json:"label"`
	Documentation *MarkupContent         `json:"documentation,omitempty"`
	Parameters    []ParameterInformation `json:"parameters,omitempty"`
}

type ParameterInformation struct {
	Label json.RawMessage `json:"label"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      ReferenceContext       `json:"context"`
}

type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Only        []string     `json:"only,omitempty"`
}

type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
}

type CompletionTriggerKind int

const (
	CompletionTriggerInvoked                         CompletionTriggerKind = 1
	CompletionTriggerTriggerCharacter                CompletionTriggerKind = 2
	CompletionTriggerTriggerForIncompleteCompletions CompletionTriggerKind = 3
)

type CompletionContext struct {
	TriggerKind      CompletionTriggerKind `json:"triggerKind"`
	TriggerCharacter string                `json:"triggerCharacter,omitempty"`
}

type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      *CompletionContext     `json:"context,omitempty"`
}

type CompletionItemKind int

const (
	CIKText          CompletionItemKind = 1
	CIKMethod        CompletionItemKind = 2
	CIKFunction      CompletionItemKind = 3
	CIKConstructor   CompletionItemKind = 4
	CIKField         CompletionItemKind = 5
	CIKVariable      CompletionItemKind = 6
	CIKClass         CompletionItemKind = 7
	CIKInterface     CompletionItemKind = 8
	CIKModule        CompletionItemKind = 9
	CIKProperty      CompletionItemKind = 10
	CIKUnit          CompletionItemKind = 11
	CIKValue         CompletionItemKind = 12
	CIKEnum          CompletionItemKind = 13
	CIKKeyword       CompletionItemKind = 14
	CIKSnippet       CompletionItemKind = 15
	CIKColor         CompletionItemKind = 16
	CIKFile          CompletionItemKind = 17
	CIKReference     CompletionItemKind = 18
	CIKFolder        CompletionItemKind = 19
	CIKEnumMember    CompletionItemKind = 20
	CIKConstant      CompletionItemKind = 21
	CIKStruct        CompletionItemKind = 22
	CIKEvent         CompletionItemKind = 23
	CIKOperator      CompletionItemKind = 24
	CIKTypeParameter CompletionItemKind = 25
)

type CompletionItem struct {
	Label               string             `json:"label"`
	Kind                CompletionItemKind `json:"kind,omitempty"`
	Detail              string             `json:"detail,omitempty"`
	InsertText          string             `json:"insertText,omitempty"`
	FilterText          string             `json:"filterText,omitempty"`
	SortText            string             `json:"sortText,omitempty"`
	TextEdit            *TextEdit          `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit         `json:"additionalTextEdits,omitempty"`
	Data                json.RawMessage    `json:"data,omitempty"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

type DocumentRangeFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Options      FormattingOptions      `json:"options"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type HoverResult struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

func (m *MarkupContent) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.Kind = "plaintext"
		m.Value = s
		return nil
	}
	type alias MarkupContent
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*m = MarkupContent(a)
	return nil
}
