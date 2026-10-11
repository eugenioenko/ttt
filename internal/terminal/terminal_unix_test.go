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
	"syscall"
	"testing"
	"time"
)

var jobPidRe = regexp.MustCompile(`JOBPID=(\d+)`)

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
		if strings.HasPrefix(line, "Sig") || strings.HasPrefix(line, "ShdPnd") || strings.HasPrefix(line, "State") {
			fields = append(fields, strings.Join(strings.Fields(line), " "))
		}
	}
	return strings.Join(fields, ", ")
}

func assertCloseHangsUpJob(t *testing.T) {
	t.Helper()
	term, pid := startShellJob(t, "bash", "sleep 300 & echo JOBPID=$!")
	before := "self: " + signalStatus(os.Getpid()) + "\nshell: " + signalStatus(term.cmd.Process.Pid) + "\njob: " + signalStatus(pid)

	closeWithin(t, term, readLoopGrace)

	select {
	case <-term.done:
	default:
		t.Fatal("readLoop still running after the background job was hung up")
	}
	deadline := time.Now().Add(2 * time.Second)
	for !processGone(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("background job %d survived Close() (shell: %v)\nbefore close:\n%s\nafter: %s", pid, term.cmd.ProcessState, before, signalStatus(pid))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCloseIsBoundedWhenJobIgnoresHangup(t *testing.T) {
	term, pid := startShellJob(t, "sh", `sh -c 'trap "" HUP; echo JOBPID=$$; exec sleep 300' &`)

	closeWithin(t, term, hangupGrace+readLoopGrace+time.Second)

	if processGone(pid) {
		t.Fatal("expected the HUP-ignoring job to outlive Close()")
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	select {
	case <-term.done:
	case <-time.After(5 * time.Second):
		t.Fatal("readLoop did not finish after the last slave holder exited")
	}
}
