package dap

import (
	"context"
	"encoding/json"
	"testing"
)

func TestInvalidTerminalRequestsDoNotRun(t *testing.T) {
	s := Session{runInTerminal: func(context.Context, TerminalRequest) (int, error) {
		t.Fatal("started an invalid terminal request")
		return 0, nil
	}}
	for _, args := range []string{`{}`, `{"args":[""]}`, `{"kind":"external","args":["tool"]}`, `{"argsCanBeInterpretedByShell":true,"args":["tool"]}`, `{"args":["tool"],"env":{"A=B":null}}`, `{"args":1}`} {
		p := packet{Type: "request", Command: "runInTerminal", Seq: "4", Arguments: json.RawMessage(args)}
		response := s.reverseResponse(t.Context(), p)
		if response["success"] != false || response["message"] == nil || response["request_seq"] != p.Seq {
			t.Fatalf("arguments %s: %v", args, response)
		}
	}
}
