package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

func TestDebugLaunchConfigurationAndThread(t *testing.T) {
	if mode := os.Getenv("TTT_TEST_DAP_CONFIGURE"); mode != "" {
		simulateConfiguringAdapter(mode)
		os.Exit(0)
	}
	for _, mode := range []string{"filters-only", "configuration-done", "superseded"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestDebugLaunchConfigurationAndThread$")
			cmd.Env = append(os.Environ(), "TTT_TEST_DAP_CONFIGURE="+mode, "GORACE=atexit_sleep_ms=0")
			cmd.Stderr = os.Stderr
			run := debugRun{folder: t.TempDir(), points: make(chan map[string][]int, 2)}
			run.points <- map[string][]int{"main.c": {3}}
			run.points <- map[string][]int{"main.c": {7}}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			hit := false
			err := run.launch(ctx, cmd, "fixture", nil, func(*dap.Session) {}, func(_ int, frames []debugFrame) { hit = len(frames) > 0 && frames[0].Line == 7; cancel() }, func() {})
			if !hit || err != context.Canceled || !cmd.ProcessState.Success() {
				t.Fatalf("hit=%v err=%v adapter=%v", hit, err, cmd.ProcessState)
			}
		})
	}
}

func simulateConfiguringAdapter(mode string) {
	codec := jsonrpc.NewCodec(os.Stdin, os.Stdout)
	type request struct {
		Seq       int
		Command   string
		Arguments json.RawMessage
	}
	// The test executable's stdout is the adapter protocol stream.
	fail := func() { os.Exit(2) }
	read := func(command string) request {
		var r request
		if err := codec.ReceiveInto(&r); err != nil || r.Command != command {
			fail()
		}
		return r
	}
	send := func(p any) {
		if err := codec.Send(p); err != nil {
			fail()
		}
	}
	reply := func(r request, body any) {
		send(map[string]any{"type": "response", "request_seq": r.Seq, "success": true, "body": body})
	}
	caps := map[string]any{"supportsConfigurationDoneRequest": mode == "configuration-done", "exceptionBreakpointFilters": []any{map[string]any{"filter": "raised", "default": true}, map[string]any{"filter": "caught"}}}
	reply(read("initialize"), caps)
	launch := read("launch")
	send(map[string]string{"type": "event", "event": "initialized"})
	bp := read("setBreakpoints")
	var args struct{ Breakpoints []struct{ Line int } }
	if json.Unmarshal(bp.Arguments, &args) != nil || len(args.Breakpoints) != 1 || args.Breakpoints[0].Line != 7 {
		fail()
	}
	reply(bp, nil)
	filter := read("setExceptionBreakpoints")
	if string(filter.Arguments) != `{"filters":["raised"]}` {
		fail()
	}
	reply(filter, nil)
	if mode == "configuration-done" {
		reply(read("configurationDone"), nil)
	}
	reply(launch, nil)
	body := map[string]any{"allThreadsStopped": true}
	if mode != "filters-only" {
		body["threadId"] = 0
	}
	send(map[string]any{"type": "event", "event": "stopped", "body": body})
	id := 0
	if mode == "filters-only" {
		id = 7
		reply(read("threads"), map[string]any{"threads": []any{map[string]int{"id": id}}})
	}
	stack := read("stackTrace")
	var trace struct{ ThreadID *int }
	if json.Unmarshal(stack.Arguments, &trace) != nil || trace.ThreadID == nil || *trace.ThreadID != id {
		fail()
	}
	if mode == "superseded" {
		send(map[string]string{"type": "event", "event": "continued"})
		send(map[string]any{"type": "response", "request_seq": stack.Seq, "success": false, "message": "already running"})
		send(map[string]any{"type": "event", "event": "stopped", "body": body})
		stack = read("stackTrace")
	}
	reply(stack, map[string]any{"stackFrames": []any{map[string]any{"line": 7, "source": map[string]string{"path": "main.c"}}}})
	reply(read("disconnect"), nil)
}
