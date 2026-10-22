package jsonrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestCodecSend(t *testing.T) {
	var buf bytes.Buffer
	codec := NewCodec(nil, &buf)

	id := 1
	err := codec.Send(Request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params:  map[string]string{"rootUri": "file:///tmp"},
	})
	if err != nil {
		t.Fatal(err)
	}

	raw := buf.String()
	if raw[:16] != "Content-Length: " {
		t.Fatalf("expected Content-Length header, got %q", raw[:16])
	}

	headerEnd := bytes.Index(buf.Bytes(), []byte("\r\n\r\n"))
	if headerEnd < 0 {
		t.Fatal("missing header separator")
	}
	body := buf.Bytes()[headerEnd+4:]

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if req.Method != "initialize" {
		t.Fatalf("expected method initialize, got %s", req.Method)
	}
	if req.ID == nil || *req.ID != 1 {
		t.Fatal("expected id 1")
	}
}

func TestCodecReceive(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	codec := NewCodec(bytes.NewReader([]byte(msg)), nil)

	resp, err := codec.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.ID) != "1" {
		t.Fatalf("expected id 1, got %s", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestCodecReceiveServerRequestWithStringID(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":"abc","method":"client/registerCapability","params":{"registrations":[]}}`
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	codec := NewCodec(bytes.NewReader([]byte(msg)), nil)

	resp, err := codec.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsRequest() || resp.IsNotification() {
		t.Fatal("expected a server request")
	}
	if string(resp.ID) != `"abc"` {
		t.Fatalf("expected id \"abc\", got %s", resp.ID)
	}
}

func TestCodecReceiveError(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":2,"error":{"code":-32600,"message":"invalid request"}}`
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	codec := NewCodec(bytes.NewReader([]byte(msg)), nil)

	resp, err := codec.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32600 {
		t.Fatalf("expected code -32600, got %d", resp.Error.Code)
	}
}

func TestCodecReceiveNotification(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"window/logMessage","params":{"type":3,"message":"hello"}}`
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	codec := NewCodec(bytes.NewReader([]byte(msg)), nil)

	resp, err := codec.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsNotification() {
		t.Fatal("expected notification")
	}
	if resp.Method != "window/logMessage" {
		t.Fatalf("expected window/logMessage, got %s", resp.Method)
	}
}

func TestCodecRoundTrip(t *testing.T) {
	pr, pw := io.Pipe()
	sender := NewCodec(nil, pw)
	receiver := NewCodec(pr, nil)

	go func() {
		id := 42
		sender.Send(Request{
			JSONRPC: "2.0",
			ID:      &id,
			Method:  "textDocument/completion",
		})
		pw.Close()
	}()

	resp, err := receiver.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Method != "textDocument/completion" {
		t.Fatalf("got method %q", resp.Method)
	}
}

func TestCodecSendNotification(t *testing.T) {
	var buf bytes.Buffer
	codec := NewCodec(nil, &buf)

	err := codec.Send(Request{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params:  map[string]string{"uri": "file:///test.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	headerEnd := bytes.Index(buf.Bytes(), []byte("\r\n\r\n"))
	body := buf.Bytes()[headerEnd+4:]

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if req.ID != nil {
		t.Fatal("notification should have no id")
	}
}

func TestCodecReceiveMissingContentLength(t *testing.T) {
	msg := "Content-Type: application/json\r\n\r\n{}"
	codec := NewCodec(bytes.NewReader([]byte(msg)), nil)

	_, err := codec.Receive()
	if err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}

func TestCodecRejectsOversizedMessageBeforeReading(t *testing.T) {
	codec := NewCodec(bytes.NewReader([]byte("Content-Length: 1000000000\r\n\r\n{}")), nil)
	codec.SetMaxContentLength(1024)
	if _, err := codec.Receive(); err == nil || !strings.Contains(err.Error(), "exceeds the 1024 byte limit") {
		t.Fatalf("error = %v", err)
	}
}

func TestCodecRejectsUnterminatedHeaderWithinReadBudget(t *testing.T) {
	reader := &io.LimitedReader{R: strings.NewReader(strings.Repeat("x", 32768)), N: 32768}
	codec := NewCodec(reader, nil)
	codec.SetMaxContentLength(1024)
	_, err := codec.Receive()
	if err == nil || !strings.Contains(err.Error(), "header") || 32768-reader.N > 8192 {
		t.Fatalf("oversized header: error = %v, consumed = %d bytes; want rejection within 8 KiB", err, 32768-reader.N)
	}
}

func TestCodecHeaderLimitsAndLegacyCompatibility(t *testing.T) {
	trailer := "\r\nContent-Length: 2\r\n\r\n"
	exact := "X: " + strings.Repeat("x", maxHeaderBytes-len("X: ")-len(trailer)) + trailer
	for _, tc := range []struct {
		name, headers string
		limited, fail bool
	}{
		{name: "exact limit", headers: exact, limited: true},
		{name: "long line", headers: "X: " + strings.Repeat("x", maxHeaderBytes) + trailer, limited: true, fail: true},
		{name: "many short lines", headers: strings.Repeat("X: value\r\n", 1024) + "Content-Length: 2\r\n\r\n", limited: true, fail: true},
		{name: "unlimited legacy transport", headers: "X: " + strings.Repeat("x", maxHeaderBytes*2) + trailer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			codec := NewCodec(strings.NewReader(tc.headers+"{}"), nil)
			if tc.limited {
				codec.SetMaxContentLength(1024)
			}
			_, err := codec.Receive()
			if tc.fail {
				if err == nil || !strings.Contains(err.Error(), "headers exceed") {
					t.Fatalf("oversized headers error = %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
