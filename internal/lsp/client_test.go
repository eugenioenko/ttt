package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

func pipeClient(t *testing.T) (*Client, *Codec) {
	t.Helper()
	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()
	t.Cleanup(func() {
		serverW.Close()
		clientW.Close()
	})
	c := &Client{
		codec:   NewCodec(clientR, clientW),
		nextID:  1,
		pending: make(map[int]chan Response),
		done:    make(chan struct{}),
	}
	go c.readLoop()
	return c, NewCodec(serverR, serverW)
}

func receiveWithin(t *testing.T, codec *Codec) Response {
	t.Helper()
	type result struct {
		msg Response
		err error
	}
	ch := make(chan result, 1)
	go func() {
		msg, err := codec.Receive()
		ch <- result{msg, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatal(r.err)
		}
		return r.msg
	case <-time.After(5 * time.Second):
		t.Fatal("no message from client")
		return Response{}
	}
}

func TestClientCallAndResponse(t *testing.T) {
	c, server := pipeClient(t)
	go func() {
		req, err := server.Receive()
		if err != nil {
			return
		}
		server.Send(Reply{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"status":"ok"}`)})
	}()

	result, err := c.call("test/method", map[string]string{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", parsed["status"])
	}
}

func TestHoverContextCancellationDropsLateResponse(t *testing.T) {
	c, server := pipeClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := c.HoverContext(ctx, "file:///test.go", 1, 2)
		done <- err
	}()
	request := receiveWithin(t, server)
	if request.Method != "textDocument/hover" {
		t.Fatalf("request method = %q", request.Method)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("hover error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("hover cancellation waited for the server to read stdin")
	}
	message := receiveWithin(t, server)
	var params struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		t.Fatal(err)
	}
	if message.Method != "$/cancelRequest" || string(params.ID) != string(request.ID) {
		t.Fatalf("cancellation = %s %s, want request %s", message.Method, message.Params, request.ID)
	}
	c.mu.Lock()
	pending := len(c.pending)
	c.mu.Unlock()
	if pending != 0 {
		t.Fatalf("cancelled request left %d pending calls", pending)
	}
	if err := server.Send(Reply{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{"contents":"stale"}`)}); err != nil {
		t.Fatal(err)
	}
	go func() {
		req, err := server.Receive()
		if err == nil {
			server.Send(Reply{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"contents":"latest"}`)})
		}
	}()
	hover, err := c.Hover("file:///test.go", 2, 3)
	if err != nil || hover == nil || hover.Contents.Value != "latest" {
		t.Fatalf("next hover = %v, error = %v", hover, err)
	}
}

func TestHoverContextAlreadyCancelledSendsNothing(t *testing.T) {
	var output bytes.Buffer
	c := &Client{codec: NewCodec(nil, &output)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.HoverContext(ctx, "file:///test.go", 0, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("hover error = %v, want context cancellation", err)
	}
	if output.Len() != 0 {
		t.Fatal("cancelled hover sent a request")
	}
}

func TestCompletionAndSignatureHelpRespectCancellation(t *testing.T) {
	var output bytes.Buffer
	c := &Client{codec: NewCodec(nil, &output)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Completion(ctx, "file:///test.go", 0, 0, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("completion error = %v, want context cancellation", err)
	}
	if _, err := c.SignatureHelp(ctx, "file:///test.go", 0, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("signature help error = %v, want context cancellation", err)
	}
	if output.Len() != 0 {
		t.Fatal("cancelled requests were sent")
	}
}

func TestDocumentSymbolsRespectCancellation(t *testing.T) {
	var output bytes.Buffer
	c := &Client{codec: NewCodec(nil, &output)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.DocumentSymbols(ctx, "file:///test.go"); !errors.Is(err, context.Canceled) {
		t.Fatalf("document symbols error = %v, want context cancellation", err)
	}
	if output.Len() != 0 {
		t.Fatal("cancelled document symbols request was sent")
	}
}

func TestDocumentSymbolsCancellationSendsCancelRequest(t *testing.T) {
	c, server := pipeClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := c.DocumentSymbols(ctx, "file:///test.go")
		done <- err
	}()
	request := receiveWithin(t, server)
	if request.Method != "textDocument/documentSymbol" {
		t.Fatalf("request method = %q", request.Method)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("document symbols error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("document symbols cancellation waited for the server reply")
	}
	message := receiveWithin(t, server)
	var params struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		t.Fatal(err)
	}
	if message.Method != "$/cancelRequest" || string(params.ID) != string(request.ID) {
		t.Fatalf("cancellation = %s %s, want request %s", message.Method, message.Params, request.ID)
	}
}

func TestClientAnswersServerRequests(t *testing.T) {
	tests := []struct {
		name      string
		request   string
		wantID    string
		wantRes   string
		wantError int
	}{
		{
			name:    "register capability with string id",
			request: `{"jsonrpc":"2.0","id":"reg-1","method":"client/registerCapability","params":{"registrations":[]}}`,
			wantID:  `"reg-1"`,
			wantRes: `null`,
		},
		{
			name:    "unregister capability",
			request: `{"jsonrpc":"2.0","id":7,"method":"client/unregisterCapability","params":{"unregisterations":[]}}`,
			wantID:  `7`,
			wantRes: `null`,
		},
		{
			name:    "configuration returns one null per item",
			request: `{"jsonrpc":"2.0","id":"cfg","method":"workspace/configuration","params":{"items":[{"section":"typescript"},{"section":"javascript"}]}}`,
			wantID:  `"cfg"`,
			wantRes: `[null,null]`,
		},
		{
			name:    "diagnostic refresh",
			request: `{"jsonrpc":"2.0","id":3,"method":"workspace/diagnostic/refresh"}`,
			wantID:  `3`,
			wantRes: `null`,
		},
		{
			name:      "unknown method",
			request:   `{"jsonrpc":"2.0","id":"x","method":"window/showDocument","params":{}}`,
			wantID:    `"x"`,
			wantError: CodeMethodNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, server := pipeClient(t)
			if err := server.Send(json.RawMessage(tt.request)); err != nil {
				t.Fatal(err)
			}
			reply := receiveWithin(t, server)
			if string(reply.ID) != tt.wantID {
				t.Errorf("reply id = %s, want %s", reply.ID, tt.wantID)
			}
			if tt.wantError != 0 {
				if reply.Error == nil || reply.Error.Code != tt.wantError {
					t.Errorf("reply error = %v, want code %d", reply.Error, tt.wantError)
				}
				if reply.Result != nil {
					t.Errorf("error reply carries result %s", reply.Result)
				}
			} else if string(reply.Result) != tt.wantRes || reply.Error != nil {
				t.Errorf("reply = result %s error %v, want result %s", reply.Result, reply.Error, tt.wantRes)
			}
		})
	}
}

func TestClientDiagnosticRefreshCallsHook(t *testing.T) {
	c, server := pipeClient(t)
	refreshed := make(chan struct{}, 1)
	c.OnDiagnosticRefresh = func() { refreshed <- struct{}{} }
	if err := server.Send(json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"workspace/diagnostic/refresh"}`)); err != nil {
		t.Fatal(err)
	}
	receiveWithin(t, server)
	select {
	case <-refreshed:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh hook not called")
	}
}

func TestClientServerRequestDoesNotResolvePendingCall(t *testing.T) {
	c, server := pipeClient(t)
	go func() {
		call, err := server.Receive()
		if err != nil {
			return
		}
		// Same integer id as the outstanding call, but a request.
		server.Send(json.RawMessage(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"workspace/configuration","params":{"items":[{}]}}`, call.ID)))
		if _, err := server.Receive(); err != nil {
			return
		}
		server.Send(Reply{JSONRPC: "2.0", ID: call.ID, Result: json.RawMessage(`"real"`)})
	}()

	result, err := c.call("test/method", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != `"real"` {
		t.Fatalf("call result = %s, want the server's reply", result)
	}
}

func TestClientNullIDNotificationIsDelivered(t *testing.T) {
	c, server := pipeClient(t)
	got := make(chan string, 1)
	c.OnDiagnostics = func(p PublishDiagnosticsParams) { got <- p.URI }
	if err := server.Send(json.RawMessage(`{"jsonrpc":"2.0","id":null,"method":"textDocument/publishDiagnostics","params":{"uri":"file:///a.ts","diagnostics":[]}}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case uri := <-got:
		if uri != "file:///a.ts" {
			t.Fatalf("diagnostics for %s, want file:///a.ts", uri)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("notification with null id was not delivered")
	}
}

func TestClientNotify(t *testing.T) {
	_, serverW := io.Pipe()
	serverR, clientW := io.Pipe()

	c := &Client{
		codec:   NewCodec(nil, clientW),
		nextID:  1,
		pending: make(map[int]chan Response),
		done:    make(chan struct{}),
	}

	serverCodec := NewCodec(serverR, serverW)

	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := serverCodec.Receive()
		if err != nil {
			t.Error(err)
			return
		}
		if resp.ID != nil {
			t.Error("notification should have no id")
		}
		if resp.Method != "textDocument/didOpen" {
			t.Errorf("expected didOpen, got %s", resp.Method)
		}
	}()

	err := c.notify("textDocument/didOpen", map[string]string{"uri": "file:///test.go"})
	if err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestClientConnectionClosed(t *testing.T) {
	clientR, serverW := io.Pipe()
	var buf bytes.Buffer

	c := &Client{
		codec:   NewCodec(clientR, &buf),
		nextID:  1,
		pending: make(map[int]chan Response),
		done:    make(chan struct{}),
	}
	go c.readLoop()

	serverW.Close()

	_, err := c.call("test/method", nil)
	if err == nil {
		t.Fatal("expected error on closed connection")
	}
}

func TestParseDocumentSymbolsHierarchical(t *testing.T) {
	data := json.RawMessage(`[
		{"name": "Server", "kind": 23, "range": {"start": {"line": 4, "character": 0}, "end": {"line": 9, "character": 1}},
		 "selectionRange": {"start": {"line": 4, "character": 5}, "end": {"line": 4, "character": 11}},
		 "children": [
			{"name": "Start", "kind": 6, "range": {"start": {"line": 6, "character": 0}, "end": {"line": 8, "character": 1}},
			 "selectionRange": {"start": {"line": 6, "character": 17}, "end": {"line": 6, "character": 22}}}
		 ]}
	]`)
	symbols, err := parseDocumentSymbols(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(symbols))
	}
	s := symbols[0]
	if s.Name != "Server" || s.Kind != SKStruct {
		t.Errorf("unexpected root symbol: %+v", s)
	}
	if s.SelectionRange.Start.Line != 4 || s.SelectionRange.Start.Character != 5 {
		t.Errorf("unexpected selectionRange: %+v", s.SelectionRange)
	}
	if len(s.Children) != 1 || s.Children[0].Name != "Start" || s.Children[0].Kind != SKMethod {
		t.Errorf("unexpected children: %+v", s.Children)
	}
}

func TestParseDocumentSymbolsFlat(t *testing.T) {
	data := json.RawMessage(`[
		{"name": "helper", "kind": 12, "containerName": "main",
		 "location": {"uri": "file:///a.go", "range": {"start": {"line": 10, "character": 5}, "end": {"line": 12, "character": 1}}}},
		{"name": "main", "kind": 12,
		 "location": {"uri": "file:///a.go", "range": {"start": {"line": 2, "character": 5}, "end": {"line": 8, "character": 1}}}}
	]`)
	symbols, err := parseDocumentSymbols(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}
	if symbols[0].Name != "main" || symbols[1].Name != "helper" {
		t.Errorf("expected document order main, helper; got %s, %s", symbols[0].Name, symbols[1].Name)
	}
	if symbols[0].SelectionRange.Start.Line != 2 {
		t.Errorf("expected selectionRange from location, got %+v", symbols[0].SelectionRange)
	}
}

func TestParseDocumentSymbolsEmpty(t *testing.T) {
	symbols, err := parseDocumentSymbols(json.RawMessage(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 0 {
		t.Fatalf("expected no symbols, got %d", len(symbols))
	}
}

func TestCodeActionSendsEmptyDiagnosticsArray(t *testing.T) {
	c, server := pipeClient(t)
	go func() {
		_, _ = c.CodeAction("file:///a.ts", Range{}, []string{"source.organizeImports"})
	}()

	req := receiveWithin(t, server)
	if req.Method != "textDocument/codeAction" {
		t.Fatalf("expected textDocument/codeAction, got %s", req.Method)
	}

	// Kept raw: decoding into []Diagnostic would turn null into a nil slice and
	// erase the distinction this test exists to catch. TypeScript 7 rejects a
	// null diagnostics field with -32602.
	var params struct {
		Context struct {
			Diagnostics json.RawMessage `json:"diagnostics"`
		} `json:"context"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if string(params.Context.Diagnostics) != "[]" {
		t.Fatalf("expected diagnostics to serialize as [], got %s", params.Context.Diagnostics)
	}

	server.Send(Reply{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage("[]")})
}
