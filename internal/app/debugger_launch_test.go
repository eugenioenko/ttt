package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

type delayedContinueStream struct {
	io.Reader
	io.WriteCloser
	processed chan struct{}
	ctx       context.Context
}

func (s delayedContinueStream) Write(data []byte) (int, error) {
	n, err := s.WriteCloser.Write(data)
	if err == nil && bytes.Contains(data, []byte(`"command":"continue"`)) {
		select {
		case <-s.processed:
		case <-s.ctx.Done():
		}
	}
	return n, err
}

func TestDebugLaunchConfigurationAndThread(t *testing.T) {
	if mode := os.Getenv("TTT_TEST_DAP_CONFIGURE"); mode != "" {
		simulateConfiguringAdapter(mode)
		os.Exit(0)
	}
	for _, mode := range []string{"filters-only", "configuration-done", "superseded", "individual", "continue-response", "continue-partial", "continue-event", "continue-new-stop", "continue-partial-new-stop", "continue-write-stop"} {
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
			var session *dap.Session
			continued := false
			replacing := false
			order := make(chan string, 2)
			run.onBindings = func(_ string, _ []int, _ []debugBinding) { order <- "response" }
			err := run.runSession(ctx, func(_ context.Context, onEvent func(dap.Event)) (*dap.Session, error) {
				if mode == "continue-write-stop" {
					in, err := cmd.StdinPipe()
					if err != nil {
						return nil, err
					}
					out, err := cmd.StdoutPipe()
					if err != nil {
						return nil, err
					}
					if err = cmd.Start(); err != nil {
						return nil, err
					}
					stream := delayedContinueStream{out, in, make(chan struct{}), ctx}
					return dap.NewSession(stream, func(e dap.Event) {
						if e.Name == "breakpoint" {
							order <- "event"
						}
						onEvent(e)
						if e.Name == "stopped" && bytes.Contains(e.Body, []byte(`"threadId":3`)) {
							close(stream.processed)
						}
					}), nil
				}
				return dap.StartStdio(cmd, func(event dap.Event) {
					if event.Name == "breakpoint" {
						order <- "event"
					}
					onEvent(event)
				}, nil)
			}, dap.Target{Type: "fixture", Request: "launch"}, nil, func(s *dap.Session) { session = s }, func(id int, frames []debugFrame, threads []debugThread) {
				if strings.HasPrefix(mode, "continue-") {
					if !continued {
						if len(threads) != 2 {
							return
						}
						continued = true
						go func() { _ = run.resumeControl(ctx, session, "continue", id) }()
						return
					}
					want := 3
					if mode == "continue-partial" {
						want = 0
					}
					if mode == "continue-partial" && !replacing {
						t.Error("replacement stack was delivered before invalidating the resumed thread")
					}
					wantThreads := 1
					if mode == "continue-partial-new-stop" {
						wantThreads = 2
						for _, thread := range threads {
							if thread.ID == 2 {
								t.Error("resumed thread still paused")
							}
						}
					}
					if id != want || len(threads) != wantThreads {
						return
					}
				}
				if mode == "individual" && (id != 2 || len(threads) != 2) {
					return
				}
				hit = len(frames) > 0 && frames[0].Line == 7
				cancel()
			}, func(pending bool) { replacing = pending })
			if len(order) != 2 || <-order != "response" || <-order != "event" {
				t.Error("breakpoint event delivered before response")
			}
			if mode == "continue-write-stop" {
				_ = cmd.Wait()
			}
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
	send(map[string]any{"type": "response", "success": true, "request_seq": bp.Seq})
	send(map[string]any{"type": "event", "event": "breakpoint"})
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
	if mode == "individual" || strings.HasPrefix(mode, "continue-") {
		body["allThreadsStopped"] = false
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
	if mode == "individual" || strings.HasPrefix(mode, "continue-") {
		body["threadId"] = 2
		send(map[string]any{"type": "event", "event": "stopped", "body": body})
		reply(stack, nil)
		stack = read("stackTrace")
	}
	if mode == "superseded" {
		send(map[string]string{"type": "event", "event": "continued"})
		send(map[string]any{"type": "response", "request_seq": stack.Seq, "success": false, "message": "already running"})
		send(map[string]any{"type": "event", "event": "stopped", "body": body})
		stack = read("stackTrace")
	}
	reply(stack, map[string]any{"stackFrames": []any{map[string]any{"line": 7, "source": map[string]string{"path": "main.c"}}}})
	if strings.HasPrefix(mode, "continue-") {
		control := read("continue")
		switch mode {
		case "continue-event":
			send(map[string]any{"type": "event", "event": "continued", "body": map[string]int{"threadId": 2}})
			reply(control, nil)
		case "continue-write-stop":
			send(map[string]any{"type": "event", "event": "stopped", "body": map[string]int{"threadId": 3}})
			pending := read("stackTrace")
			reply(control, nil)
			reply(pending, nil)
			probe := read("stackTrace")
			send(map[string]any{"type": "response", "request_seq": probe.Seq, "success": false, "message": "stack temporarily unavailable"})
		case "continue-new-stop", "continue-partial-new-stop":
			send(map[string]any{"type": "event", "event": "stopped", "body": map[string]int{"threadId": 3}})
			pending := read("stackTrace")
			reply(control, map[string]bool{"allThreadsContinued": mode == "continue-new-stop"})
			reply(pending, nil)
		case "continue-partial":
			reply(control, map[string]bool{"allThreadsContinued": false})
		default:
			reply(control, nil)
		}
		if mode != "continue-write-stop" && mode != "continue-partial" && !strings.HasSuffix(mode, "new-stop") {
			send(map[string]any{"type": "event", "event": "stopped", "body": map[string]int{"threadId": 3}})
		}
		reply(read("stackTrace"), map[string]any{"stackFrames": []any{map[string]int{"line": 7}}})
	} else if mode != "individual" {
		reply(read("threads"), map[string]any{"threads": []any{map[string]int{"id": id}}})
	}
	reply(read("disconnect"), nil)
}
