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

func startCLI(t *testing.T, dir string, env []string, args ...string) (*terminal.Terminal, <-chan struct{}) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args = append([]string{"-test.run=^TestCLIStartupProcess$", "--"}, args...)
	child, err := terminal.New(executable, 100, 30, 100, append(env, "TTT_TEST_CLI=1"), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	child.OnExit = func() { close(exited) }
	child.Run()
	t.Cleanup(child.Close)
	return child, exited
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
			t.Fatalf("editor exited before the expected state:\n%s", child.RawTail())
		case <-timeout.C:
			t.Fatalf("editor did not reach the expected state:\n%s", child.ScreenText())
		case <-tick.C:
		}
	}
}

func TestReuseWindowPreservesPipedStdinWithFile(t *testing.T) {
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
	for _, name := range []string{"forwarded.txt", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("one\ntwo\nthree"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for key, value := range map[string]string{"TTT_CONFIG_DIR": configDir, "TMPDIR": dir, "TMP": dir, "TEMP": dir} {
		t.Setenv(key, value)
	}
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
	receiver, receiverExited := startCLI(t, dir, nil, "--listen")
	waitForCLI(t, receiver, receiverExited, func() bool { return post("wait 0") == nil })
	forwarded, forwardedExited := startCLI(t, dir, nil, "forwarded.txt:3:2")
	select {
	case <-forwardedExited:
	case <-time.After(10 * time.Second):
		t.Fatalf("plain file was not forwarded:\n%s", forwarded.ScreenText())
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
	const pipedText = "PIPE_INPUT_1248"
	sender, senderExited := startCLI(t, dir, []string{"TTT_TEST_STDIN=" + pipedText + "\n"}, "notes.txt")
	waitForCLI(t, sender, senderExited, func() bool {
		screen := sender.ScreenText()
		return strings.Contains(screen, pipedText) && strings.Contains(screen, "notes.txt") && strings.Contains(screen, "untitled")
	})
	for _, tab := range readReceiver().Tabs {
		if tab.Path == filepath.Join(dir, "notes.txt") {
			t.Fatal("piped invocation forwarded its file to the running editor")
		}
	}
}
