package app

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/view"
)

func TestParseWaitForArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        string
		wantText    string
		wantTimeout time.Duration
	}{
		{name: "plain text", args: "Explore", wantText: "Explore", wantTimeout: defaultWaitForTimeout},
		{name: "plain whitespace", args: "working tree clean", wantText: "working tree clean", wantTimeout: defaultWaitForTimeout},
		{name: "numeric suffix is text", args: "build 123", wantText: "build 123", wantTimeout: defaultWaitForTimeout},
		{name: "explicit timeout", args: "ready now timeout=125", wantText: "ready now", wantTimeout: 125 * time.Millisecond},
		{name: "quoted escapes", args: `"  ready \"now\"  " timeout=250`, wantText: `  ready "now"  `, wantTimeout: 250 * time.Millisecond},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text, timeout, err := parseWaitForArgs(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if text != tc.wantText {
				t.Errorf("text = %q, want %q", text, tc.wantText)
			}
			if timeout != tc.wantTimeout {
				t.Errorf("timeout = %s, want %s", timeout, tc.wantTimeout)
			}
		})
	}
}

func TestParseWaitForArgsRejectsInvalidSyntax(t *testing.T) {
	for _, args := range []string{
		"",
		`"unterminated`,
		`"ready" eventually`,
		"ready timeout=0",
		"timeout=100",
	} {
		t.Run(args, func(t *testing.T) {
			if _, _, err := parseWaitForArgs(args); err == nil {
				t.Fatalf("parseWaitForArgs(%q) succeeded, want error", args)
			}
		})
	}
}

func TestParseExecMillisecondsBoundaries(t *testing.T) {
	maximum := strconv.FormatInt(maxExecMilliseconds, 10)
	overflow := strconv.FormatInt(maxExecMilliseconds+1, 10)

	duration, err := parseExecMilliseconds(maximum, false)
	if err != nil {
		t.Fatalf("maximum representable milliseconds rejected: %v", err)
	}
	if want := time.Duration(maxExecMilliseconds) * time.Millisecond; duration != want {
		t.Fatalf("duration = %s, want %s", duration, want)
	}
	if _, err := parseExecMilliseconds(overflow, false); err == nil {
		t.Fatalf("first overflowing millisecond value %s accepted", overflow)
	}
	if duration, err := parseExecMilliseconds("0", true); err != nil || duration != 0 {
		t.Fatalf("wait zero = %s, %v; want accepted zero", duration, err)
	}
	if _, err := parseExecMilliseconds("0", false); err == nil {
		t.Fatal("wait-for accepted zero timeout")
	}
}

func TestExecDurationOverflowIsTypedInvalidInput(t *testing.T) {
	a := newListenTestApp(t)
	overflow := strconv.FormatInt(maxExecMilliseconds+1, 10)
	tests := []string{
		"wait " + overflow,
		`wait-for x timeout=` + overflow,
	}

	for _, script := range tests {
		t.Run(script, func(t *testing.T) {
			start := time.Now()
			result, err := RunExecScript(a, script)
			if result.Completed != 0 {
				t.Fatalf("completed = %d, want 0", result.Completed)
			}
			var execErr *ExecError
			if !errors.As(err, &execErr) || execErr.Kind != ExecErrorInvalid {
				t.Fatalf("error = %T %v, want invalid ExecError", err, err)
			}
			if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
				t.Fatalf("overflow rejection took %s", elapsed)
			}
		})
	}
}

func TestRunExecScriptReturnsTypedPartialResult(t *testing.T) {
	a := newListenTestApp(t)

	result, err := RunExecScript(a, "wait 0; click nope 2; type skipped")

	if result.Completed != 1 {
		t.Fatalf("completed = %d, want 1", result.Completed)
	}
	var execErr *ExecError
	if !errors.As(err, &execErr) {
		t.Fatalf("error = %T %v, want *ExecError", err, err)
	}
	if execErr.Kind != ExecErrorInvalid || execErr.Index != 2 || execErr.Action != "click" {
		t.Fatalf("exec error = %+v, want invalid click at command 2", execErr)
	}
}

func TestRunExecScriptWaitForObservesDelayedVisibleScreen(t *testing.T) {
	a := newListenTestApp(t)
	posted := make(chan error, 1)
	go func() {
		time.Sleep(75 * time.Millisecond)
		posted <- runExecMain(a, func() error {
			a.Status.SetNotification("ASYNC READY", view.NotifyInfo, time.Second)
			return nil
		})
	}()

	result, err := RunExecScript(a, `wait-for "ASYNC READY" timeout=1000`)

	if err != nil {
		t.Fatal(err)
	}
	if result.Completed != 1 {
		t.Fatalf("completed = %d, want 1", result.Completed)
	}
	if err := <-posted; err != nil {
		t.Fatalf("posting visible text: %v", err)
	}
}

func TestRunExecScriptWaitForTimeoutIsBounded(t *testing.T) {
	a := newListenTestApp(t)
	start := time.Now()

	result, err := RunExecScript(a, `wait-for "never visible" timeout=40`)
	elapsed := time.Since(start)

	if result.Completed != 0 {
		t.Fatalf("completed = %d, want 0", result.Completed)
	}
	var execErr *ExecError
	if !errors.As(err, &execErr) || execErr.Kind != ExecErrorTimeout {
		t.Fatalf("error = %T %v, want timeout ExecError", err, err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("wait-for took %s, want bounded near 40ms", elapsed)
	}
}
