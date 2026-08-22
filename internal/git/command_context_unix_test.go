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
