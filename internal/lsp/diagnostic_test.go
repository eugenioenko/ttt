package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func initializeWith(t *testing.T, serverCaps string) (*Client, *Codec, json.RawMessage) {
	t.Helper()
	c, server := pipeClient(t)
	errCh := make(chan error, 1)
	go func() { errCh <- c.Initialize("file:///tmp") }()

	req := receiveWithin(t, server)
	if req.Method != "initialize" {
		t.Fatalf("method = %q, want initialize", req.Method)
	}
	result := json.RawMessage(`{"capabilities":` + serverCaps + `}`)
	if err := server.Send(Reply{JSONRPC: "2.0", ID: req.ID, Result: result}); err != nil {
		t.Fatal(err)
	}
	if n := receiveWithin(t, server); n.Method != "initialized" {
		t.Fatalf("method = %q, want initialized", n.Method)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	return c, server, req.Params
}

func TestInitializeAdvertisesPullDiagnostics(t *testing.T) {
	_, _, params := initializeWith(t, `{}`)
	var p struct {
		Capabilities struct {
			TextDocument struct {
				Diagnostic *DiagnosticClientCapabilities `json:"diagnostic"`
			} `json:"textDocument"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		t.Fatal(err)
	}
	d := p.Capabilities.TextDocument.Diagnostic
	if d == nil {
		t.Fatalf("initialize params carry no textDocument.diagnostic: %s", params)
	}
	if d.RelatedDocumentSupport {
		t.Error("client claims relatedDocumentSupport")
	}
}

func TestSupportsPullDiagnosticsFollowsServerCapability(t *testing.T) {
	tests := []struct {
		name string
		caps string
		want bool
	}{
		{"push only", `{"textDocumentSync":1}`, false},
		{"pull", `{"diagnosticProvider":{"identifier":"typescript","interFileDependencies":true,"workspaceDiagnostics":false}}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _, _ := initializeWith(t, tt.caps)
			if got := c.SupportsPullDiagnostics(); got != tt.want {
				t.Errorf("SupportsPullDiagnostics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiagnosticReports(t *testing.T) {
	tests := []struct {
		name      string
		result    string
		wantOK    bool
		wantCount int
		wantErr   bool
		retrigger bool
		rpcErr    *RPCError
	}{
		{
			name:      "full report",
			result:    `{"kind":"full","resultId":"1","items":[{"range":{"start":{"line":2,"character":4},"end":{"line":2,"character":9}},"severity":1,"message":"Cannot find name 'foo'."}]}`,
			wantOK:    true,
			wantCount: 1,
		},
		{
			name:   "empty full report clears",
			result: `{"kind":"full","items":[]}`,
			wantOK: true,
		},
		{
			name:   "unchanged report keeps existing",
			result: `{"kind":"unchanged","resultId":"1"}`,
		},
		{
			name:    "unknown kind",
			result:  `{"kind":"partial"}`,
			wantErr: true,
		},
		{
			name:      "server cancelled with retrigger",
			rpcErr:    &RPCError{Code: CodeServerCancelled, Message: "busy", Data: json.RawMessage(`{"retriggerRequest":true}`)},
			wantErr:   true,
			retrigger: true,
		},
		{
			name:    "server cancelled without retrigger",
			rpcErr:  &RPCError{Code: CodeServerCancelled, Message: "busy", Data: json.RawMessage(`{"retriggerRequest":false}`)},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, server, _ := initializeWith(t, `{"diagnosticProvider":{"identifier":"typescript"}}`)
			go func() {
				req, err := server.Receive()
				if err != nil {
					return
				}
				var p DocumentDiagnosticParams
				if err := json.Unmarshal(req.Params, &p); err != nil || req.Method != "textDocument/diagnostic" ||
					p.TextDocument.URI != "file:///a.ts" || p.Identifier != "typescript" {
					server.Send(Reply{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "bad request: " + string(req.Params)}})
					return
				}
				if tt.rpcErr != nil {
					server.Send(Reply{JSONRPC: "2.0", ID: req.ID, Error: tt.rpcErr})
					return
				}
				server.Send(Reply{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(tt.result)})
			}()

			diags, ok, err := c.Diagnostic(context.Background(), "file:///a.ts")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got := errors.Is(err, ErrRetriggerDiagnostic); got != tt.retrigger {
				t.Errorf("retrigger = %v, want %v (err %v)", got, tt.retrigger, err)
			}
			if ok != tt.wantOK || len(diags) != tt.wantCount {
				t.Errorf("got ok=%v count=%d, want ok=%v count=%d", ok, len(diags), tt.wantOK, tt.wantCount)
			}
		})
	}
}
