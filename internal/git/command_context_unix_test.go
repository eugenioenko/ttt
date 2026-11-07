//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommitMessageContextCancellationKillsAndReapsProcessGroup(t *testing.T) {
	binDir := t.TempDir()
	parentPath := filepath.Join(binDir, "parent.pid")
	childPath := filepath.Join(binDir, "child.pid")
	script := "#!/bin/sh\nprintf '%s' \"$$\" > \"$FAKE_GIT_PARENT\"\nsleep 30 &\nprintf '%s' \"$!\" > \"$FAKE_GIT_CHILD\"\nwait\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_GIT_PARENT", parentPath)
	t.Setenv("FAKE_GIT_CHILD", childPath)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := CommitMessageContext(ctx, "/repo", strings.Repeat("a", 40))
		errCh <- err
	}()
	parent := waitFakeGitPID(t, parentPath)
	child := waitFakeGitPID(t, childPath)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled git returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled git command was not reaped")
	}
	waitFakeGitExit(t, parent)
	waitFakeGitExit(t, child)
}

func TestMutationCancellationAllowsLockCleanupAndBoundsUnresponsiveGit(t *testing.T) {
	for _, ignoreTerm := range []bool{false, true} {
		name := "cleanup"
		if ignoreTerm {
			name = "escalation"
		}
		t.Run(name, func(t *testing.T) {
			binDir := t.TempDir()
			lock := filepath.Join(binDir, "index.lock")
			pidPath := filepath.Join(binDir, "parent.pid")
			script := "#!/bin/sh\ntrap 'rm -f \"$FAKE_GIT_LOCK\"; exit 0' TERM\n: > \"$FAKE_GIT_LOCK\"\nprintf '%s' \"$$\" > \"$FAKE_GIT_PARENT\"\nwhile :; do sleep 30; done\n"
			if ignoreTerm {
				script = "#!/bin/sh\ntrap '' TERM\nprintf '%s' \"$$\" > \"$FAKE_GIT_PARENT\"\nexec sleep 30\n"
			}
			if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
			t.Setenv("FAKE_GIT_LOCK", lock)
			t.Setenv("FAKE_GIT_PARENT", pidPath)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			errCh := make(chan error, 1)
			go func() { errCh <- runPathsContext(ctx, binDir, []string{"add", "--"}, []string{"file.txt"}) }()
			pid := waitFakeGitPID(t, pidPath)
			t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })
			cancel()
			select {
			case err := <-errCh:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("mutation did not exit after cancellation")
			}
			if !ignoreTerm {
				if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("lock not cleaned up: %v", err)
				}
			}
			waitFakeGitExit(t, pid)
		})
	}
}

func waitFakeGitPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(string(data))
			if err != nil {
				t.Fatal(err)
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fake git did not write %s", filepath.Base(path))
	return 0
}

func waitFakeGitExit(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("fake git process %d is still alive", pid)
}
