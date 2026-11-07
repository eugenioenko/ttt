package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/terminal"
)

func TestCLIStartupProcess(t *testing.T) {
	if os.Getenv("TTT_TEST_CLI") != "1" {
		return
	}
	os.Args = append([]string{"ttt"}, os.Args[slices.Index(os.Args, "--")+1:]...)
	if err := os.WriteFile(os.Getenv("TTT_TEST_PID_FILE"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("TTT_TEST_STDERR_FILE"); path != "" {
		stderr, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			t.Fatal(err)
		}
		os.Stderr = stderr
	}
	if text, ok := os.LookupEnv("TTT_TEST_STDIN"); ok {
		// Keep the controlling terminal while replacing stdin with a real pipe.
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdin = r
		go func() {
			_, _ = io.WriteString(w, text)
			_ = w.Close()
		}()
	}
	main()
	os.Exit(0)
}

func startCLI(t *testing.T, dir string, env []string, args ...string) (*terminal.Terminal, <-chan struct{}, int) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args = append([]string{"-test.run=^TestCLIStartupProcess$", "--"}, args...)
	pidPath := filepath.Join(t.TempDir(), "pid")
	child, err := terminal.New(executable, 100, 30, 100, append(env, "TTT_TEST_CLI=1", "TTT_TEST_PID_FILE="+pidPath), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	child.OnExit = func() { close(exited) }
	child.Run()
	t.Cleanup(child.Close)
	var pid int
	waitForCLI(t, child, exited, func() bool {
		data, err := os.ReadFile(pidPath)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(string(data))
		return err == nil && pid > 0
	})
	return child, exited, pid
}

func waitForCLI(t *testing.T, child *terminal.Terminal, exited <-chan struct{}, ready func() bool) {
	t.Helper()
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for !ready() {
		select {
		case <-exited:
			if ready() {
				return
			}
			t.Fatalf("editor exited before the expected state:\n%s", child.RawTail())
		case <-timeout.C:
			t.Fatalf("editor did not reach the expected state:\n%s", child.ScreenText())
		case <-tick.C:
		}
	}
}

func TestReuseWindowForwardsTargetsAndPreservesPipedStdin(t *testing.T) {
	// Socket paths must also fit macOS's 104-byte limit.
	tempRoot := ""
	if runtime.GOOS != "windows" {
		tempRoot = "/tmp"
	}
	dir, err := os.MkdirTemp(tempRoot, "ttt-cli-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	configDir := filepath.Join(dir, "config")
	if err := os.Mkdir(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(`{"reuseWindow":true,"lsp":{"enabled":false},"plugins":{"enabled":false}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"forwarded.txt", "notes.txt", "piped-only.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("one\ntwo\nthree"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for key, value := range map[string]string{"TTT_CONFIG_DIR": configDir, "TMPDIR": dir, "TMP": dir, "TEMP": dir} {
		t.Setenv(key, value)
	}
	t.Setenv("TTT_FORWARD_SOCKET", "")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_, port, _ := net.SplitHostPort(address)
	_ = listener.Close()
	t.Setenv("TTT_LISTEN_PORT", port)
	client := &http.Client{Timeout: time.Second}
	post := func(script string) error {
		response, err := client.Post("http://"+address+"/exec", "text/plain", strings.NewReader(script))
		if err != nil {
			return err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("exec returned %d: %s", response.StatusCode, body)
		}
		return nil
	}
	receiver, receiverExited, receiverPID := startCLI(t, dir, nil, "--listen")
	waitForCLI(t, receiver, receiverExited, func() bool { return post("wait 0") == nil })
	sockets, err := filepath.Glob(filepath.Join(dir, "ttt-*", "*.sock"))
	if err != nil || len(sockets) != 1 {
		t.Fatalf("receiver sockets = %v, error = %v, want one socket", sockets, err)
	}
	childTemp := t.TempDir()
	// ConPTY may report exit before the PTY read loop has consumed the final output.
	stderrPath := filepath.Join(t.TempDir(), "forwarded-stderr.txt")
	forwarded, forwardedExited, _ := startCLI(t, dir, []string{
		"TTT_TEST_STDERR_FILE=" + stderrPath, "TTT_FORWARD_SOCKET=" + sockets[0],
		"TMPDIR=" + childTemp, "TMP=" + childTemp, "TEMP=" + childTemp,
	}, "forwarded.txt:3:2")
	select {
	case <-forwardedExited:
	case <-time.After(10 * time.Second):
		t.Fatalf("plain file was not forwarded:\n%s", forwarded.ScreenText())
	}
	stderr, err := os.ReadFile(stderrPath)
	if err != nil {
		t.Fatal(err)
	}
	confirmation := fmt.Sprintf("opened in running editor (pid %d)\n", receiverPID)
	if string(stderr) != confirmation {
		t.Fatalf("forwarding stderr = %q, want %q", stderr, confirmation)
	}
	statePath := filepath.Join(dir, "receiver.json")
	readReceiver := func() app.DebugState {
		t.Helper()
		if err := post(fmt.Sprintf(`debug "%s"`, filepath.ToSlash(statePath))); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		var state app.DebugState
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	state := readReceiver()
	if state.Buffer == nil || state.Buffer.Path != filepath.Join(dir, "forwarded.txt") || state.Cursor.Line != 2 || state.Cursor.Col != 1 {
		t.Fatalf("forwarded file or cursor missing: %+v", state)
	}
	folder := filepath.Join(dir, "added-workspace")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := post("type UNSAVED_1335"); err != nil {
		t.Fatal(err)
	}
	before := readReceiver()
	forward := func(args ...string) {
		t.Helper()
		sender, exited, _ := startCLI(t, folder, []string{"TTT_TEST_STDERR_FILE=" + stderrPath}, args...)
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
			t.Fatalf("targets %v were not forwarded:\n%s", args, sender.ScreenText())
		}
		stderr, err := os.ReadFile(stderrPath)
		if err != nil || string(stderr) != confirmation {
			t.Fatalf("forwarding stderr = %q, error = %v, want %q", stderr, err, confirmation)
		}
	}
	forward(".")
	state = readReceiver()
	if state.Buffer == nil || !state.Buffer.Modified || state.Buffer.Path != before.Buffer.Path || !slices.Equal(state.Buffer.Text, before.Buffer.Text) || state.ActiveTab != before.ActiveTab || !slices.Equal(state.Tabs, before.Tabs) {
		t.Fatalf("adding a folder changed open tabs or unsaved text: %+v", state)
	}
	screenPath := filepath.Join(dir, "folders.txt")
	if err := post(fmt.Sprintf(`screenshot "%s"`, filepath.ToSlash(screenPath))); err != nil {
		t.Fatal(err)
	}
	screen, err := os.ReadFile(screenPath)
	if err != nil || !strings.Contains(string(screen), "added-workspace") {
		t.Fatalf("forwarded folder missing from explorer: %v\n%s", err, screen)
	}
	forward(".", "../notes.txt:2:1")
	state = readReceiver()
	if state.Buffer == nil || state.Buffer.Path != filepath.Join(dir, "notes.txt") || state.Cursor.Line != 1 || state.Cursor.Col != 0 {
		t.Fatalf("mixed request did not open the file at its position: %+v", state)
	}
	if len(state.Tabs) != len(before.Tabs)+1 || !slices.Contains(state.Tabs, app.DebugTab{Path: before.Buffer.Path, Modified: true}) {
		t.Fatalf("mixed request discarded existing tabs: %+v", state.Tabs)
	}
	const pipedText = "PIPE_INPUT_1248"
	sender, senderExited, _ := startCLI(t, dir, []string{"TTT_TEST_STDIN=" + pipedText + "\n"}, "piped-only.txt")
	waitForCLI(t, sender, senderExited, func() bool {
		screen := sender.ScreenText()
		return strings.Contains(screen, pipedText) && strings.Contains(screen, "piped-only.txt") && strings.Contains(screen, "untitled")
	})
	for _, tab := range readReceiver().Tabs {
		if tab.Path == filepath.Join(dir, "piped-only.txt") {
			t.Fatal("piped invocation forwarded its file to the running editor")
		}
	}
}
