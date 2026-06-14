package lsp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

type Client struct {
	cmd     *exec.Cmd
	codec   *Codec
	nextID  int
	pending map[int]chan Response
	mu      sync.Mutex
	done    chan struct{}

	completionTriggers   []string
	signatureTriggers    []string
	signatureRetriggers  []string

	OnDiagnostics func(params PublishDiagnosticsParams)
}

func NewClient(command []string, workDir string) (*Client, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = workDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", command[0], err)
	}

	c := &Client{
		cmd:     cmd,
		codec:   NewCodec(stdout, stdin),
		nextID:  1,
		pending: make(map[int]chan Response),
		done:    make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

func (c *Client) readLoop() {
	defer close(c.done)
	for {
		resp, err := c.codec.Receive()
		if err != nil {
			slog.Debug("lsp read loop exit", "err", err)
			c.mu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = make(map[int]chan Response)
			c.mu.Unlock()
			return
		}
		if resp.IsNotification() {
			slog.Debug("lsp notification", "method", resp.Method)
			if resp.Method == "textDocument/publishDiagnostics" && c.OnDiagnostics != nil {
				var params PublishDiagnosticsParams
				if err := json.Unmarshal(resp.Params, &params); err == nil {
					c.OnDiagnostics(params)
				}
			}
			continue
		}
		if resp.ID != nil {
			c.mu.Lock()
			ch, ok := c.pending[*resp.ID]
			if ok {
				delete(c.pending, *resp.ID)
			}
			c.mu.Unlock()
			if ok {
				ch <- resp
			}
		}
	}
}

func (c *Client) call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan Response, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	err := c.codec.Send(Request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	resp, ok := <-ch
	if !ok {
		return nil, fmt.Errorf("connection closed")
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

func (c *Client) notify(method string, params any) error {
	return c.codec.Send(Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

func (c *Client) Initialize(rootURI string) error {
	result, err := c.call("initialize", InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{
				Completion: &CompletionClientCapabilities{
					CompletionItem: &CompletionItemClientCapabilities{
						SnippetSupport: false,
						ResolveSupport: &CompletionItemResolveSupport{
							Properties: []string{"additionalTextEdits"},
						},
					},
				},
				PublishDiagnostics: &PublishDiagnosticsClientCapabilities{},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}
	slog.Debug("lsp initialized", "capabilities", initResult.Capabilities)

	caps := initResult.Capabilities
	if caps.CompletionProvider != nil {
		c.completionTriggers = caps.CompletionProvider.TriggerCharacters
	}
	if caps.SignatureHelpProvider != nil {
		c.signatureTriggers = caps.SignatureHelpProvider.TriggerCharacters
		c.signatureRetriggers = caps.SignatureHelpProvider.RetriggerCharacters
	}

	return c.notify("initialized", struct{}{})
}

func (c *Client) DidOpen(uri, languageID, text string) error {
	return c.notify("textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	})
}

func (c *Client) DidChange(uri, text string, version int) error {
	return c.notify("textDocument/didChange", DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{{Text: text}},
	})
}

func (c *Client) DidSave(uri, text string) error {
	return c.notify("textDocument/didSave", DidSaveTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Text:         text,
	})
}

func (c *Client) DidClose(uri string) error {
	return c.notify("textDocument/didClose", DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})
}

func (c *Client) CompletionTriggerCharacters() []string {
	return c.completionTriggers
}

func (c *Client) SignatureHelpTriggerCharacters() []string {
	return c.signatureTriggers
}

func (c *Client) SignatureHelpRetriggerCharacters() []string {
	return c.signatureRetriggers
}

func (c *Client) Completion(uri string, line, col int, ctx *CompletionContext) ([]CompletionItem, error) {
	result, err := c.call("textDocument/completion", CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
		Context:      ctx,
	})
	if err != nil {
		return nil, err
	}

	var list CompletionList
	if err := json.Unmarshal(result, &list); err != nil {
		var items []CompletionItem
		if err2 := json.Unmarshal(result, &items); err2 != nil {
			return nil, fmt.Errorf("parse completion result: %w", err)
		}
		return items, nil
	}
	return list.Items, nil
}

func (c *Client) ResolveCompletion(item CompletionItem) (*CompletionItem, error) {
	result, err := c.call("completionItem/resolve", item)
	if err != nil {
		return nil, err
	}
	var resolved CompletionItem
	if err := json.Unmarshal(result, &resolved); err != nil {
		return nil, fmt.Errorf("parse resolve result: %w", err)
	}
	return &resolved, nil
}

func (c *Client) SignatureHelp(uri string, line, col int) (*SignatureHelp, error) {
	result, err := c.call("textDocument/signatureHelp", SignatureHelpParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	var sig SignatureHelp
	if err := json.Unmarshal(result, &sig); err != nil {
		return nil, fmt.Errorf("parse signatureHelp result: %w", err)
	}
	return &sig, nil
}

func (c *Client) Hover(uri string, line, col int) (*HoverResult, error) {
	result, err := c.call("textDocument/hover", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	var hover HoverResult
	if err := json.Unmarshal(result, &hover); err != nil {
		return nil, fmt.Errorf("parse hover result: %w", err)
	}
	return &hover, nil
}

func (c *Client) Definition(uri string, line, col int) ([]Location, error) {
	return c.locationRequest("textDocument/definition", uri, line, col)
}

func (c *Client) Implementation(uri string, line, col int) ([]Location, error) {
	return c.locationRequest("textDocument/implementation", uri, line, col)
}

func (c *Client) TypeDefinition(uri string, line, col int) ([]Location, error) {
	return c.locationRequest("textDocument/typeDefinition", uri, line, col)
}

func (c *Client) locationRequest(method, uri string, line, col int) ([]Location, error) {
	result, err := c.call(method, TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
	})
	if err != nil {
		return nil, err
	}
	var locs []Location
	if err := json.Unmarshal(result, &locs); err != nil {
		var single Location
		if err2 := json.Unmarshal(result, &single); err2 != nil {
			return nil, fmt.Errorf("parse %s result: %w", method, err)
		}
		return []Location{single}, nil
	}
	return locs, nil
}

func (c *Client) CodeAction(uri string, r Range, kinds []string) ([]CodeAction, error) {
	result, err := c.call("textDocument/codeAction", CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        r,
		Context:      CodeActionContext{Only: kinds},
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	var actions []CodeAction
	if err := json.Unmarshal(result, &actions); err != nil {
		return nil, fmt.Errorf("parse codeAction result: %w", err)
	}
	return actions, nil
}

func (c *Client) Rename(uri string, line, col int, newName string) (*WorkspaceEdit, error) {
	result, err := c.call("textDocument/rename", RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
		NewName:      newName,
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	var edit WorkspaceEdit
	if err := json.Unmarshal(result, &edit); err != nil {
		return nil, fmt.Errorf("parse rename result: %w", err)
	}
	return &edit, nil
}

func (c *Client) References(uri string, line, col int, includeDeclaration bool) ([]Location, error) {
	result, err := c.call("textDocument/references", ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
		Context:      ReferenceContext{IncludeDeclaration: includeDeclaration},
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	var locs []Location
	if err := json.Unmarshal(result, &locs); err != nil {
		return nil, fmt.Errorf("parse references result: %w", err)
	}
	return locs, nil
}

func (c *Client) Formatting(uri string, tabSize int, insertSpaces bool) ([]TextEdit, error) {
	result, err := c.call("textDocument/formatting", DocumentFormattingParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Options:      FormattingOptions{TabSize: tabSize, InsertSpaces: insertSpaces},
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	var edits []TextEdit
	if err := json.Unmarshal(result, &edits); err != nil {
		return nil, fmt.Errorf("parse formatting result: %w", err)
	}
	return edits, nil
}

func (c *Client) RangeFormatting(uri string, r Range, tabSize int, insertSpaces bool) ([]TextEdit, error) {
	result, err := c.call("textDocument/rangeFormatting", DocumentRangeFormattingParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        r,
		Options:      FormattingOptions{TabSize: tabSize, InsertSpaces: insertSpaces},
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	var edits []TextEdit
	if err := json.Unmarshal(result, &edits); err != nil {
		return nil, fmt.Errorf("parse rangeFormatting result: %w", err)
	}
	return edits, nil
}

func (c *Client) Shutdown() error {
	_, err := c.call("shutdown", nil)
	if err != nil {
		return err
	}
	_ = c.notify("exit", nil)
	c.cmd.Wait()
	return nil
}

func (c *Client) Close() {
	c.cmd.Process.Kill()
	c.cmd.Wait()
}
