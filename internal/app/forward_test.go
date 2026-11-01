//go:build !windows

package app

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

// macOS limits socket paths to 104 bytes, which t.TempDir can exceed.
func setupForwardTest(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tttf")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("TMPDIR", dir)
	saved := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = saved })
	if err := os.Mkdir(forwardDirectory(), 0700); err != nil {
		t.Fatal(err)
	}
}

func fakeForwardReceiver(t *testing.T, start time.Time, reply error) {
	t.Helper()
	ln, err := net.Listen("unix", filepath.Join(forwardDirectory(), forwardSocketName(start, os.Getpid())))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			jsonrpc.NewConn(conn, conn, jsonrpc.Options{
				OnClose:   func(error) { conn.Close() },
				OnRequest: func(string, json.RawMessage) (any, error) { return nil, reply },
			})
		}
	}()
}

func TestForwardSocketNameLengthAndOrder(t *testing.T) {
	t.Setenv("TMPDIR", "/"+strings.Repeat("t", 59))
	var previous string
	for _, start := range []int64{35, 36, 2176782335, 2176782336, 1793050000000, 1793050000001} {
		name := forwardSocketName(time.UnixMilli(start), 2147483647)
		if path := filepath.Join(forwardDirectory(), name); len(path) > 100 {
			t.Errorf("socket path is %d bytes, want at most 100: %s", len(path), path)
		}
		if name <= previous {
			t.Errorf("newer socket %q does not sort after %q", name, previous)
		}
		previous = name
	}
}

func TestForwardFilesRoundTrip(t *testing.T) {
	setupForwardTest(t)
	a := newListenTestApp(t)
	if err := runExecMain(a, func() error { a.Root.SetFocus(a.Search); return nil }); err != nil {
		t.Fatal(err)
	}
	defer StartForward(a)()

	dir := t.TempDir()
	first := filepath.Join(dir, "first-\xff.txt")
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(second, []byte("one\ntwo\nthree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ForwardFiles([]FileTarget{{Path: first}, {Path: second, Line: 3, Col: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := runExecMain(a, func() error {
		if paths := a.EditorGroup.OpenFilePaths(); !slices.Contains(paths, first) || !slices.Contains(paths, second) {
			t.Errorf("open paths = %v, want both forwarded files", paths)
		}
		if cursor := a.EditorGroup.Editor.Cursor; cursor.Line != 2 || cursor.Col != 1 {
			t.Errorf("cursor = %v, want line 2 col 1", cursor)
		}
		if a.Root.Focused != a.Search {
			t.Error("forwarding moved focus with editor.focusOnOpen off")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRunEventLoopStartsForwardAfterStartupWork(t *testing.T) {
	setupForwardTest(t)
	a := prepareListenTestApp(t)
	startupPath := filepath.Join(t.TempDir(), "startup.txt")
	if err := os.WriteFile(startupPath, []byte("first\nsecond"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.EditorGroup.OpenFile(startupPath); err != nil {
		t.Fatal(err)
	}
	a.PendingFileTargets = []FileTarget{{Path: startupPath, Line: 2, Col: 3}}
	forwardStarted := make(chan struct{})
	var stopForward func()
	startListenTestEventLoop(t, a, func() {
		if len(a.PendingFileTargets) != 0 {
			t.Errorf("pending file targets = %v, want startup targets applied before forwarding starts", a.PendingFileTargets)
		}
		if !slices.Contains(a.EditorGroup.OpenFilePaths(), startupPath) {
			t.Errorf("startup file %q was not open before forwarding started", startupPath)
		}
		if cursor := a.EditorGroup.Editor.Cursor; cursor.Line != 1 || cursor.Col != 2 {
			t.Errorf("startup cursor = %v, want line 1 col 2 before forwarding starts", cursor)
		}
		stopForward = StartForward(a)
		close(forwardStarted)
	})
	select {
	case <-forwardStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("event loop did not finish startup")
	}
	t.Cleanup(func() {
		if stopForward != nil {
			stopForward()
		}
	})

	forwardedPath := filepath.Join(t.TempDir(), "forwarded.txt")
	if err := os.WriteFile(forwardedPath, []byte("forwarded"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ForwardFiles([]FileTarget{{Path: forwardedPath}}); err != nil {
		t.Fatalf("ForwardFiles() after startup = %v", err)
	}
	if err := runExecMain(a, func() error {
		if !slices.Contains(a.EditorGroup.OpenFilePaths(), forwardedPath) {
			t.Errorf("forwarded file %q was not opened", forwardedPath)
		}
		return nil
	}); err != nil {
		t.Fatalf("checking forwarded file = %v", err)
	}
}

func TestForwardFilesSilentReceiver(t *testing.T) {
	setupForwardTest(t)
	saved := forwardCallTimeout
	forwardCallTimeout = 50 * time.Millisecond
	t.Cleanup(func() { forwardCallTimeout = saved })
	ln, err := net.Listen("unix", filepath.Join(forwardDirectory(), forwardSocketName(time.Now(), os.Getpid())))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	result := make(chan error, 1)
	targets := make([]FileTarget, 1400)
	for i := range targets {
		targets[i].Path = strings.Repeat("x", 500)
	}
	go func() { result <- ForwardFiles(targets) }()
	select {
	case conn := <-accepted:
		t.Cleanup(func() { conn.Close() })
	case <-time.After(5 * time.Second):
		t.Fatal("receiver did not accept the connection")
	}
	select {
	case err := <-result:
		if err == nil || err.Error() != "the running editor did not respond" || errors.Is(err, ErrForwardUnavailable) {
			t.Fatalf("ForwardFiles = %v, want non-fallback timeout error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ForwardFiles blocked on a connected silent receiver past the call deadline")
	}
}

func TestForwardFilesPartialFailure(t *testing.T) {
	a := newListenTestApp(t)

	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	if err := os.WriteFile(good, []byte("one\ntwo\nthree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad")
	if err := os.Mkdir(bad, 0700); err != nil {
		t.Fatal(err)
	}
	err := runExecMain(a, func() error {
		return a.openForwarded([]FileTarget{{Path: bad}, {Path: good}, {Path: bad, Line: 3, Col: 2}})
	})
	if err == nil || !strings.Contains(err.Error(), bad) {
		t.Fatalf("forward error = %v, want failed path %s", err, bad)
	}
	if err := runExecMain(a, func() error {
		if paths := a.EditorGroup.OpenFilePaths(); !slices.Contains(paths, good) || slices.Contains(paths, bad) {
			t.Errorf("open paths = %v, want only %s", paths, good)
		}
		if cursor := a.EditorGroup.Editor.Cursor; cursor.Line != 0 || cursor.Col != 0 {
			t.Errorf("rejected target moved cursor to %v", cursor)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestForwardFilesReceiverChoice(t *testing.T) {
	for _, tc := range []struct {
		name    string
		replies []error
		want    string
	}{
		{"newest wins by one millisecond", []error{errors.New("older"), errors.New("newer")}, "newer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupForwardTest(t)
			for i, reply := range tc.replies {
				fakeForwardReceiver(t, time.UnixMilli(1793050000000+int64(i)), reply)
			}
			if err := ForwardFiles([]FileTarget{{Path: filepath.Join(t.TempDir(), "file.txt")}}); err == nil || err.Error() != tc.want {
				t.Fatalf("ForwardFiles = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestForwardFilesLeftoverSockets(t *testing.T) {
	setupForwardTest(t)
	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for i, pid := range []int{dead.Process.Pid, os.Getpid()} {
		path := filepath.Join(forwardDirectory(), forwardSocketName(time.UnixMilli(int64(i)), pid))
		ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
		if err != nil {
			t.Fatal(err)
		}
		ln.SetUnlinkOnClose(false)
		ln.Close()
		paths = append(paths, path)
	}
	if err := ForwardFiles([]FileTarget{{Path: filepath.Join(t.TempDir(), "file.txt")}}); !errors.Is(err, ErrForwardUnavailable) {
		t.Fatalf("no live receiver: %v", err)
	}
	if _, err := os.Lstat(paths[0]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("dead-pid socket was not removed: %v", err)
	}
	if _, err := os.Lstat(paths[1]); err != nil {
		t.Errorf("live-pid socket was removed: %v", err)
	}
}

func TestForwardUnsafeDirectory(t *testing.T) {
	setupForwardTest(t)
	a := prepareListenTestApp(t)
	if err := os.Chmod(forwardDirectory(), 0777); err != nil {
		t.Fatal(err)
	}
	defer StartForward(a)()
	if err := ForwardFiles([]FileTarget{{Path: filepath.Join(t.TempDir(), "file.txt")}}); !errors.Is(err, ErrForwardUnavailable) {
		t.Fatalf("unsafe directory: %v", err)
	}
	if !strings.Contains(a.Status.Notification, "unsafe forwarding directory") {
		t.Fatalf("missing status notice: %q", a.Status.Notification)
	}
	if entries, err := os.ReadDir(forwardDirectory()); err != nil || len(entries) != 0 {
		t.Fatalf("listened in unsafe directory: %v, %v", entries, err)
	}
}
