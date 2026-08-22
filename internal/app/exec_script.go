package app

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/eugenioenko/ttt/internal/config"

	"github.com/gdamore/tcell/v3"
)

const (
	DefaultExecSeparator  = ";"
	defaultWaitForTimeout = 5 * time.Second
	execRequestTimeout    = 5 * time.Second
	waitForPollInterval   = 25 * time.Millisecond
	maxExecMilliseconds   = int64((1<<63 - 1) / int64(time.Millisecond))
)

type ExecErrorKind string

const (
	ExecErrorInvalid ExecErrorKind = "invalid"
	ExecErrorFailed  ExecErrorKind = "failed"
	ExecErrorTimeout ExecErrorKind = "timeout"
)

type ExecResult struct {
	Completed         int
	ShutdownRequested bool
}

type ExecError struct {
	Kind    ExecErrorKind
	Index   int
	Command string
	Action  string
	Err     error
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("command %d %q: %v", e.Index, e.Command, e.Err)
}

func (e *ExecError) Unwrap() error {
	return e.Err
}

type execActionError struct {
	kind ExecErrorKind
	err  error
}

func (e *execActionError) Error() string {
	return e.err.Error()
}

func (e *execActionError) Unwrap() error {
	return e.err
}

func invalidExec(format string, args ...any) error {
	return &execActionError{kind: ExecErrorInvalid, err: fmt.Errorf(format, args...)}
}

func failedExec(format string, args ...any) error {
	return &execActionError{kind: ExecErrorFailed, err: fmt.Errorf(format, args...)}
}

func timeoutExec(format string, args ...any) error {
	return &execActionError{kind: ExecErrorTimeout, err: fmt.Errorf(format, args...)}
}

type execInputRequest struct {
	Event     tcell.Event
	Lifecycle *execRequestLifecycle
}

type execMainRequest struct {
	Run       func() error
	Lifecycle *execRequestLifecycle
}

type execRequestState uint32

const (
	execRequestPending execRequestState = iota
	execRequestClaimed
	execRequestCanceled
)

type execRequestLifecycle struct {
	state atomic.Uint32
	done  chan error
}

func newExecRequestLifecycle() *execRequestLifecycle {
	return &execRequestLifecycle{done: make(chan error, 1)}
}

func (r *execRequestLifecycle) claim() bool {
	return r.state.CompareAndSwap(uint32(execRequestPending), uint32(execRequestClaimed))
}

func (r *execRequestLifecycle) cancel() bool {
	return r.state.CompareAndSwap(uint32(execRequestPending), uint32(execRequestCanceled))
}

func (r *execRequestLifecycle) complete(err error) {
	select {
	case r.done <- err:
	default:
	}
}

func RunExecScript(a *App, script string) (ExecResult, error) {
	return RunExecScriptSep(a, script, DefaultExecSeparator)
}

func RunExecScriptSep(a *App, script, sep string) (ExecResult, error) {
	if sep == "" {
		sep = DefaultExecSeparator
	}
	var result ExecResult
	commandIndex := 0
	for _, raw := range strings.Split(script, sep) {
		cmd := strings.TrimSpace(raw)
		if cmd == "" {
			continue
		}
		commandIndex++
		action, args := parseExecCommand(cmd)
		slog.Debug("exec_script", "action", action, "args", args)

		if err := runExecAction(a, action, args); err != nil {
			kind := ExecErrorFailed
			var actionErr *execActionError
			if errors.As(err, &actionErr) {
				kind = actionErr.kind
			}
			return result, &ExecError{
				Kind:    kind,
				Index:   commandIndex,
				Command: cmd,
				Action:  action,
				Err:     err,
			}
		}
		result.Completed++
		if action == "quit" || action == "shutdown" {
			result.ShutdownRequested = true
			return result, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return result, nil
}

func runExecAction(a *App, action, args string) error {
	switch action {
	case "click":
		return execClick(a, args)
	case "rclick":
		return execRClick(a, args)
	case "hover":
		return execHover(a, args)
	case "drag":
		return execDrag(a, args)
	case "key":
		return execKey(a, args)
	case "type":
		return execType(a, args)
	case "exec":
		return execCommand(a, args)
	case "screenshot":
		return execScreenshot(a, args)
	case "debug":
		return execDebug(a, args)
	case "wait":
		return execWait(args)
	case "wait-for":
		return execWaitFor(a, args)
	case "paste":
		return execPaste(a, args)
	case "copy":
		if strings.TrimSpace(args) != "" {
			return invalidExec("copy does not accept arguments")
		}
		return runExecMain(a, func() error {
			a.Copy()
			return nil
		})
	case "panel":
		return execPanel(a, args)
	case "quit", "shutdown":
		if strings.TrimSpace(args) != "" {
			return invalidExec("%s does not accept arguments", action)
		}
		return nil
	default:
		return invalidExec("unknown action %q", action)
	}
}

func parseExecCommand(cmd string) (string, string) {
	idx := strings.IndexByte(cmd, ' ')
	if idx < 0 {
		return cmd, ""
	}
	return cmd[:idx], strings.TrimSpace(cmd[idx+1:])
}

func execClick(a *App, args string) error {
	x, y, err := parseCoordinates("click", args, 2)
	if err != nil {
		return err
	}
	if err := postExecInput(a, tcell.NewEventMouse(x[0], y[0], tcell.Button1, tcell.ModNone)); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return postExecInput(a, tcell.NewEventMouse(x[0], y[0], tcell.ButtonNone, tcell.ModNone))
}

func execRClick(a *App, args string) error {
	x, y, err := parseCoordinates("rclick", args, 2)
	if err != nil {
		return err
	}
	if err := postExecInput(a, tcell.NewEventMouse(x[0], y[0], tcell.Button2, tcell.ModNone)); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return postExecInput(a, tcell.NewEventMouse(x[0], y[0], tcell.ButtonNone, tcell.ModNone))
}

func execHover(a *App, args string) error {
	x, y, err := parseCoordinates("hover", args, 2)
	if err != nil {
		return err
	}
	return postExecInput(a, tcell.NewEventMouse(x[0], y[0], tcell.ButtonNone, tcell.ModNone))
}

func execDrag(a *App, args string) error {
	x, y, err := parseCoordinates("drag", args, 4)
	if err != nil {
		return err
	}
	if err := postExecInput(a, tcell.NewEventMouse(x[0], y[0], tcell.Button1, tcell.ModNone)); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)

	const steps = 10
	for i := 1; i <= steps; i++ {
		mx := x[0] + (x[1]-x[0])*i/steps
		my := y[0] + (y[1]-y[0])*i/steps
		if err := postExecInput(a, tcell.NewEventMouse(mx, my, tcell.Button1, tcell.ModNone)); err != nil {
			return err
		}
		time.Sleep(15 * time.Millisecond)
	}

	return postExecInput(a, tcell.NewEventMouse(x[1], y[1], tcell.ButtonNone, tcell.ModNone))
}

func parseCoordinates(action, args string, count int) ([2]int, [2]int, error) {
	var xs, ys [2]int
	parts := strings.Fields(args)
	if len(parts) != count {
		return xs, ys, invalidExec("%s requires %d coordinates", action, count)
	}
	values := make([]int, count)
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return xs, ys, invalidExec("invalid %s coordinate %q: %v", action, part, err)
		}
		values[i] = value
	}
	xs[0], ys[0] = values[0], values[1]
	if count == 4 {
		xs[1], ys[1] = values[2], values[3]
	}
	return xs, ys, nil
}

func execKey(a *App, args string) error {
	combo := strings.TrimSpace(args)
	if combo == "" {
		return invalidExec("key requires a key combo")
	}

	steps, err := config.ParseKeyString(combo)
	if err != nil {
		return invalidExec("invalid key combo %q: %v", combo, err)
	}

	for _, step := range steps {
		key, mod, ch := comboToTcell(step)
		if err := postExecInput(a, tcell.NewEventKey(key, keyEventStr(ch), mod)); err != nil {
			return err
		}
		time.Sleep(30 * time.Millisecond)
	}
	return nil
}

func execType(a *App, args string) error {
	text := stripQuotes(args)
	for _, r := range text {
		if err := postExecInput(a, tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone)); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func execPaste(a *App, args string) error {
	text := stripQuotes(args)
	if text == "" {
		return invalidExec("paste requires text")
	}
	return runExecMain(a, func() error {
		a.PasteText(text)
		a.FlushEditorOnChange()
		return nil
	})
}

func execCommand(a *App, args string) error {
	title := stripQuotes(strings.TrimSpace(args))
	if title == "" {
		return invalidExec("exec requires a command title")
	}

	return runExecMain(a, func() error {
		cmd, ok := a.Reg.FindByTitle(title)
		if !ok {
			return invalidExec("command %q not found", title)
		}
		if !a.Reg.Execute(cmd.ID) {
			return failedExec("command %q disappeared before execution", title)
		}
		a.FlushEditorOnChange()
		return nil
	})
}

func execScreenshot(a *App, args string) error {
	path := stripQuotes(strings.TrimSpace(args))
	if path == "" {
		return invalidExec("screenshot requires a file path")
	}
	return runExecMain(a, func() error {
		if err := a.DumpScreenshot(path); err != nil {
			return failedExec("screenshot %q failed: %v", path, err)
		}
		return nil
	})
}

func execDebug(a *App, args string) error {
	path := stripQuotes(strings.TrimSpace(args))
	if path == "" {
		return invalidExec("debug requires a file path")
	}
	return runExecMain(a, func() error {
		if err := a.DumpDebugState(path); err != nil {
			return failedExec("debug dump %q failed: %v", path, err)
		}
		return nil
	})
}

func execWait(args string) error {
	ms := strings.TrimSpace(args)
	if ms == "" {
		return invalidExec("wait requires milliseconds")
	}
	duration, err := parseExecMilliseconds(ms, true)
	if err != nil {
		return invalidExec("invalid wait duration %q", ms)
	}
	time.Sleep(duration)
	return nil
}

func execWaitFor(a *App, args string) error {
	text, timeout, err := parseWaitForArgs(args)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for {
		var visible bool
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return timeoutExec("screen text %q not visible after %s", text, timeout)
		}
		requestTimeout := min(remaining, execRequestTimeout)
		if err := runExecMainTimeout(a, func() error {
			visible = strings.Contains(a.Screenshot(), text)
			return nil
		}, requestTimeout); err != nil {
			return err
		}
		if visible {
			return nil
		}
		remaining = time.Until(deadline)
		if remaining <= 0 {
			return timeoutExec("screen text %q not visible after %s", text, timeout)
		}
		time.Sleep(min(waitForPollInterval, remaining))
	}
}

func parseWaitForArgs(args string) (string, time.Duration, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return "", 0, invalidExec("wait-for requires screen text")
	}

	text := args
	option := ""
	if strings.HasPrefix(args, "\"") {
		end := quotedStringEnd(args)
		if end < 0 {
			return "", 0, invalidExec("wait-for has an unterminated quoted string")
		}
		quoted := args[:end]
		unquoted, err := strconv.Unquote(quoted)
		if err != nil {
			return "", 0, invalidExec("invalid wait-for text: %v", err)
		}
		text = unquoted
		option = strings.TrimSpace(args[end:])
	} else {
		fields := strings.Fields(args)
		last := fields[len(fields)-1]
		if strings.HasPrefix(last, "timeout=") {
			option = last
			text = strings.TrimSpace(strings.TrimSuffix(args, last))
		}
	}

	if text == "" {
		return "", 0, invalidExec("wait-for requires non-empty screen text")
	}
	timeout := defaultWaitForTimeout
	if option != "" {
		if strings.ContainsAny(option, " \t\r\n") || !strings.HasPrefix(option, "timeout=") {
			return "", 0, invalidExec("wait-for expects optional timeout=MS after the text")
		}
		ms := strings.TrimPrefix(option, "timeout=")
		parsed, err := parseExecMilliseconds(ms, false)
		if err != nil {
			return "", 0, invalidExec("invalid wait-for timeout %q", ms)
		}
		timeout = parsed
	}
	return text, timeout, nil
}

func parseExecMilliseconds(value string, allowZero bool) (time.Duration, error) {
	milliseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || milliseconds < 0 || (!allowZero && milliseconds == 0) || milliseconds > maxExecMilliseconds {
		return 0, fmt.Errorf("milliseconds out of range")
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
}

func quotedStringEnd(s string) int {
	escaped := false
	for i := 1; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		switch s[i] {
		case '\\':
			escaped = true
		case '"':
			return i + 1
		}
	}
	return -1
}

func execPanel(a *App, args string) error {
	id := stripQuotes(strings.TrimSpace(args))
	if id == "" {
		return invalidExec("panel requires a panel ID")
	}
	return runExecMain(a, func() error {
		if !a.BottomPanel.HasPanel(id) {
			return invalidExec("panel %q not found", id)
		}
		a.BottomPanel.SetActivePanel(id)
		if !a.ContentSplit.ShowBottom {
			r := a.ContentSplit.GetRect()
			maxH := r.H - 4
			if a.ContentSplit.BottomH <= 1 || a.ContentSplit.BottomH > maxH {
				a.ContentSplit.BottomH = min(r.H/2, maxH)
			}
			a.ContentSplit.ShowBottom = true
		}
		if w := a.BottomPanel.ActiveWidget(); w != nil {
			a.Root.SetFocus(w)
		}
		return nil
	})
}

func postExecInput(a *App, event tcell.Event) error {
	return postExecInputTimeout(a, event, execRequestTimeout)
}

func postExecInputTimeout(a *App, event tcell.Event, timeout time.Duration) error {
	lifecycle := newExecRequestLifecycle()
	if err := a.Screen.PostEvent(tcell.NewEventInterrupt(&execInputRequest{Event: event, Lifecycle: lifecycle})); err != nil {
		return failedExec("failed to post input event: %v", err)
	}
	return awaitExecRequest(lifecycle, timeout)
}

func runExecMain(a *App, run func() error) error {
	return runExecMainTimeout(a, run, execRequestTimeout)
}

func runExecMainTimeout(a *App, run func() error, timeout time.Duration) error {
	lifecycle := newExecRequestLifecycle()
	if err := a.Screen.PostEvent(tcell.NewEventInterrupt(&execMainRequest{Run: run, Lifecycle: lifecycle})); err != nil {
		return failedExec("failed to post main-thread action: %v", err)
	}
	return awaitExecRequest(lifecycle, timeout)
}

func awaitExecRequest(lifecycle *execRequestLifecycle, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-lifecycle.done:
		return err
	case <-timer.C:
		if lifecycle.cancel() {
			return timeoutExec("event loop did not acknowledge action within %s", timeout)
		}
		return <-lifecycle.done
	}
}

func StopExecLoop(a *App) error {
	return runExecMain(a, func() error {
		*a.Running = false
		return nil
	})
}

func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func ExecScriptUsage() string {
	return fmt.Sprintf(`--exec "commands"  Execute semicolon-separated commands after startup

Supported commands:
  click X Y          Simulate left mouse click at coordinates
  rclick X Y         Simulate right mouse click at coordinates
  hover X Y          Simulate mouse hover (move) at coordinates
  drag X1 Y1 X2 Y2   Simulate a mouse drag between two points
  key COMBO          Simulate key press (e.g., key ctrl+p, key enter)
  type TEXT           Type a string of text
  paste TEXT          Simulate bracketed paste (terminal paste)
  copy                Copy selection to clipboard
  exec "Command"     Run a command palette command by title
  screenshot PATH    Save screen text to file
  debug PATH         Save debug state JSON to file
  wait MS            Wait milliseconds
  wait-for TEXT [timeout=MS]
                     Wait until visible screen text appears (default 5000ms)
  panel ID           Show and focus a bottom panel by ID
  quit               Exit the editor
  shutdown           Alias for quit, for use over --listen

Invalid actions fail --exec with a nonzero exit and POST /exec with a non-2xx
response. Quote wait-for text to preserve surrounding whitespace or escapes.

--listen starts an HTTP command server on 127.0.0.1:4242 that accepts the
same script format over POST /exec, so a running editor can be driven
interactively instead of scripted in advance:
  curl -X POST --data "type hi; wait-for hi; screenshot /tmp/s1.txt" http://127.0.0.1:4242/exec

Example:
  %s --exec "wait-for Explore; screenshot /tmp/s1.txt; quit"`, os.Args[0])
}
