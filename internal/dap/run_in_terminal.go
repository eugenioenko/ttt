package dap

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type TerminalRequest struct {
	Kind, Title, Cwd            string
	Args                        []string
	Env                         map[string]*string
	ArgsCanBeInterpretedByShell bool
}

type RunInTerminal func(context.Context, TerminalRequest) (int, error)

func (s *Session) SupportsRunInTerminal() bool { return s.runInTerminal != nil }

func (s *Session) reverseResponse(ctx context.Context, p packet) map[string]any {
	response := map[string]any{"type": "response", "request_seq": p.Seq, "command": p.Command, "success": false}
	if p.Command != "runInTerminal" || !s.SupportsRunInTerminal() {
		response["message"] = "unsupported request"
		return response
	}
	var req TerminalRequest
	err := json.Unmarshal(p.Arguments, &req)
	if err == nil {
		err = req.validate()
	}
	if err == nil {
		var pid int
		pid, err = s.runInTerminal(ctx, req)
		if err == nil {
			response["success"] = true
			response["body"] = map[string]int{"processId": pid}
			return response
		}
	}
	response["message"] = err.Error()
	return response
}

func (req TerminalRequest) validate() error {
	if (req.Kind != "" && req.Kind != "integrated") || req.ArgsCanBeInterpretedByShell || len(req.Args) == 0 || req.Args[0] == "" {
		return errors.New("expected an integrated terminal and literal argv")
	}
	for key := range req.Env {
		if key == "" || strings.ContainsAny(key, "=\x00") {
			return errors.New("invalid environment variable name")
		}
	}
	return nil
}
