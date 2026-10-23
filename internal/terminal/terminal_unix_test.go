//go:build !windows

package terminal

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/aymanbagabas/go-pty"
)

var (
	jobPidRe = regexp.MustCompile(`JOBPID=(\d+)`)
	promptRe = regexp.MustCompile(`[$#] $`)
)

type delayedReadPty struct {
	pty.UnixPty
	ready <-chan struct{}
	delay time.Duration
}

func (p *delayedReadPty) Read(b []byte) (int, error) {
	<-p.ready
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
	return p.UnixPty.Read(b[:min(len(b), 3)])
}

func TestExitDrainsOutputBeforeCallback(t *testing.T) {
	term, err := New("/bin/sh", 80, 24, 0, nil, "", "-c", "printf drain-end")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ready := make(chan struct{})
	var release sync.Once
	defer term.Close()
	defer release.Do(func() { close(ready) })
	term.pt = &delayedReadPty{UnixPty: term.pt.(pty.UnixPty), ready: ready}
	exited := make(chan string, 1)
	term.OnExit = func() { exited <- string(term.RawTail()) }
	term.Run()

	select {
	case <-term.waited:
	case <-time.After(5 * time.Second):
		t.Fatal("program never exited")
	}
	release.Do(func() { close(ready) })

	select {
	case got := <-exited:
		want := "drain-end"
		if got != want {
			t.Fatalf("OnExit saw %d output bytes, want %d", len(got), len(want))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnExit never fired after the reader was released")
	}
}

func TestExitKeepsDrainingWhileReaderMakesProgress(t *testing.T) {
	term, err := New("/bin/sh", 80, 24, 0, nil, "", "-c", "printf drain-end")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()
	term.pt = &delayedReadPty{
		UnixPty: term.pt.(pty.UnixPty),
		ready:   term.waited,
		delay:   readLoopGrace / 2,
	}
	exited := make(chan string, 1)
	term.OnExit = func() { exited <- string(term.RawTail()) }
	term.Run()

	select {
	case got := <-exited:
		if got != "drain-end" {
			t.Fatalf("OnExit interrupted an active drain; raw tail: %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnExit never fired after the output was drained")
	}
}

// startShellJob runs cmdline in an interactive shell; cmdline must print
// JOBPID=<pid> once the job is ready to be hung up.
func startShellJob(t *testing.T, shell, cmdline string) (*Terminal, int) {
	t.Helper()
	path, err := exec.LookPath(shell)
	if err != nil {
		t.Skipf("%s not available", shell)
	}
	term, err := New(path, 80, 24, 0, []string{"HOME=" + t.TempDir()}, "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	term.Run()
	term.WriteString(cmdline + "\n")

	deadline := time.Now().Add(5 * time.Second)
	for {
		if m := jobPidRe.FindSubmatch(term.RawTail()); m != nil {
			pid, _ := strconv.Atoi(string(m[1]))
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			return term, pid
		}
		if time.Now().After(deadline) {
			term.Close()
			t.Fatalf("background job never started; output: %q", term.RawTail())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func processGone(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return true
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	return len(fields) > 0 && fields[0] == "Z"
}

func closeWithin(t *testing.T, term *Terminal, limit time.Duration) time.Duration {
	t.Helper()
	start := time.Now()
	closed := make(chan struct{})
	go func() {
		term.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() hung with a background job holding the pty")
	}
	elapsed := time.Since(start)
	if elapsed > limit {
		t.Fatalf("Close() took %v, want under %v", elapsed, limit)
	}
	return elapsed
}

func TestCloseHangsUpBackgroundJobs(t *testing.T) {
	assertCloseHangsUpJob(t)
}

func TestCloseLetsProgramHandlePtyHangup(t *testing.T) {
	if os.Getenv("TTT_TEST_PTY_HANGUP") == "1" {
		signal.Ignore(syscall.SIGHUP)
		_, _ = os.Stdout.WriteString("hangup-ready\n")
		var input [1]byte
		if _, err := os.Stdin.Read(input[:]); err == nil {
			os.Exit(2)
		}
		// Cleanup after PTY EOF must finish before Close escalates to SIGKILL.
		time.Sleep(hangupGrace / 4)
		os.Exit(0)
	}
	term, err := New(os.Args[0], 80, 24, 0, []string{"TTT_TEST_PTY_HANGUP=1", "GORACE=atexit_sleep_ms=0"}, "", "-test.run=^TestCloseLetsProgramHandlePtyHangup$")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()
	ready := make(chan struct{}, 1)
	term.OnUpdate = func() {
		term.AckUpdate()
		if strings.Contains(string(term.RawTail()), "hangup-ready\r\n") {
			select {
			case ready <- struct{}{}:
			default:
			}
		}
	}
	term.Run()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("program did not become ready")
	}
	closeWithin(t, term, 2*hangupGrace+readLoopGrace+time.Second)
	if !term.cmd.ProcessState.Success() {
		t.Fatalf("program was killed before it handled PTY hangup: %v", term.cmd.ProcessState)
	}
}

// nohup and some CI runners start ttt with SIGHUP ignored, which children
// inherit. The disposition must be in place before the Go runtime starts, so
// the test reruns itself under a shell that ignores SIGHUP.
func TestCloseHangsUpJobsWhenParentIgnoresHangup(t *testing.T) {
	if os.Getenv("TTT_TEST_HUP_IGNORED") == "1" {
		for range 2 {
			if !signal.Ignored(syscall.SIGHUP) {
				t.Fatal("expected SIGHUP to be ignored outside shell startup")
			}
			assertCloseHangsUpJob(t)
			assertChildIgnoresHangup(t)
		}
		return
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	cmd := exec.Command("sh", "-c", `trap "" HUP; exec "$0" -test.run='^TestCloseHangsUpJobsWhenParentIgnoresHangup$' -test.count=1 -test.v`, os.Args[0])
	cmd.Env = append(os.Environ(), "TTT_TEST_HUP_IGNORED=1")
	out, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(string(out), "--- SKIP") {
		t.Skipf("rerun with SIGHUP ignored skipped:\n%s", out)
	}
	if err != nil || !strings.Contains(string(out), "--- PASS") {
		t.Fatalf("hangup test with SIGHUP ignored failed: %v\n%s", err, out)
	}
}

func assertChildIgnoresHangup(t *testing.T) {
	t.Helper()
	child := exec.Command("sleep", "5")
	if err := child.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	defer func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	}()
	status, err := os.ReadFile("/proc/" + strconv.Itoa(child.Process.Pid) + "/status")
	if err != nil {
		t.Skipf("no procfs: %v", err)
	}
	for line := range strings.Lines(string(status)) {
		if mask, ok := strings.CutPrefix(line, "SigIgn:"); ok {
			bits, err := strconv.ParseUint(strings.TrimSpace(mask), 16, 64)
			if err != nil {
				t.Fatalf("parse SigIgn %q: %v", mask, err)
			}
			if bits&(1<<(syscall.SIGHUP-1)) == 0 {
				t.Fatalf("child started after the shell does not ignore SIGHUP (SigIgn %s)", strings.TrimSpace(mask))
			}
			return
		}
	}
	t.Fatal("SigIgn missing from /proc status")
}

func signalStatus(pid int) string {
	status, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return err.Error()
	}
	var fields []string
	for line := range strings.Lines(string(status)) {
		if strings.HasPrefix(line, "Sig") || strings.HasPrefix(line, "ShdPnd") || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Pid:") || strings.HasPrefix(line, "NSpgid:") {
			fields = append(fields, strings.Join(strings.Fields(line), " "))
		}
	}
	return strings.Join(fields, ", ")
}

// Close loses the hangup if it lands before bash is reading at its prompt
// again, or before the job has exec'd and dropped bash's signal handlers
// (#1114). Wait for both.
func waitForHangupReady(t *testing.T, term *Terminal, pid int, comm string) {
	t.Helper()
	marker := "JOBPID=" + strconv.Itoa(pid)
	deadline := time.Now().Add(5 * time.Second)
	for {
		tail := string(term.RawTail())
		atPrompt := false
		if i := strings.LastIndex(tail, marker); i >= 0 {
			atPrompt = promptRe.MatchString(tail[i:])
		}
		if atPrompt && procComm(pid, comm) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("shell never settled with job %d running %s; output: %q", pid, comm, tail)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func procComm(pid int, comm string) bool {
	got, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	return err == nil && strings.TrimSpace(string(got)) == comm
}

func assertCloseHangsUpJob(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/proc/self"); err != nil {
		t.Skipf("no procfs; cannot observe the job exec: %v", err)
	}
	term, pid := startShellJob(t, "bash", "sleep 300 & echo JOBPID=$!")
	t.Cleanup(term.Close)
	waitForHangupReady(t, term, pid, "sleep")
	before := "self: " + signalStatus(os.Getpid()) + "\nshell: " + signalStatus(term.cmd.Process.Pid) + "\njob: " + signalStatus(pid)
	elapsed := closeWithin(t, term, 2*hangupGrace+readLoopGrace+time.Second)

	select {
	case <-term.done:
	default:
		t.Fatalf("readLoop still running after Close() took %v (shell: %v)\nbefore close:\n%s\nafter job: %s", elapsed, term.cmd.ProcessState, before, signalStatus(pid))
	}
	deadline := time.Now().Add(2 * time.Second)
	for !processGone(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("background job %d survived Close() taking %v (shell: %v)\nbefore close:\n%s\nafter: %s", pid, elapsed, term.cmd.ProcessState, before, signalStatus(pid))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCloseIsBoundedWhenJobIgnoresHangup(t *testing.T) {
	term, pid := startShellJob(t, "sh", `sh -c 'trap "" HUP; echo JOBPID=$$; exec sleep 300' &`)

	closeWithin(t, term, 2*hangupGrace+readLoopGrace+time.Second)

	if processGone(pid) {
		t.Fatal("expected the HUP-ignoring job to outlive Close()")
	}
	select {
	case <-term.done:
	case <-time.After(5 * time.Second):
		t.Fatal("readLoop did not stop while the background job held the slave open")
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func TestOnExitIsBoundedWhenJobKeepsSlaveOpen(t *testing.T) {
	term, pid := startShellJob(t, "sh", `sh -c 'trap "" HUP; echo JOBPID=$$; exec sleep 300' &`)
	defer term.Close()
	exited := make(chan struct{})
	term.cbMu.Lock()
	term.OnExit = func() { close(exited) }
	term.cbMu.Unlock()
	term.WriteString("exit\n")

	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("OnExit hung with a background job holding the pty")
	}
	if processGone(pid) {
		t.Fatal("expected the HUP-ignoring job to outlive the shell")
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	select {
	case <-term.done:
	case <-time.After(5 * time.Second):
		t.Fatal("readLoop did not finish after the last slave holder exited")
	}
}
