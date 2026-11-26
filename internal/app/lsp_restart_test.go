package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
)

func missingGoServerApp(t *testing.T) (*App, string) {
	t.Helper()
	a := buildTestApp(t, config.DefaultSettings())
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	a.Screen = term.NewTcellScreenFrom(sim)
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"go": {Command: []string{"ttt-test-missing-server"}, Languages: map[string]string{".go": "go"}},
	}})
	path := filepath.Join(t.TempDir(), "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	requireNoError(t, a.EditorGroup.OpenFile(path))
	return a, path
}

func TestRestartReopensAfterTheInFlightSend(t *testing.T) {
	a, path := missingGoServerApp(t)
	inFlight := make(chan struct{})
	a.DocVersions[path] = 3
	a.lspSent[path] = inFlight

	a.handleLSPServerStopped(&LSPServerStopped{Server: "go", WorkDir: filepath.Dir(path)})
	a.DocVersionsMu.Lock()
	reopened := a.lspSent[path]
	a.DocVersionsMu.Unlock()
	select {
	case <-reopened:
		t.Fatal("document was reopened before the send already in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(inFlight)
	select {
	case <-reopened:
	case <-time.After(5 * time.Second):
		t.Fatal("document never reopened after the in-flight send")
	}
}

func TestOpenFromBeforeRestartIsNotSent(t *testing.T) {
	a, path := missingGoServerApp(t)
	oldRoot := filepath.Join(t.TempDir(), "old")
	newRoot := filepath.Join(t.TempDir(), "new")
	a.pinLSPRoot(path, "go", oldRoot)
	a.DocVersionsMu.Lock()
	epoch := a.lspEpochs["go"]
	a.DocVersionsMu.Unlock()

	cases := []struct {
		name   string
		root   string
		epoch  int
		pinned string
		want   bool
	}{
		{"current", oldRoot, epoch, oldRoot, true},
		{"restarted", oldRoot, epoch + 1, oldRoot, false},
		{"moved root", oldRoot, epoch + 1, newRoot, false},
		{"closed", oldRoot, epoch, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a.DocVersionsMu.Lock()
			a.lspEpochs["go"] = tc.epoch
			a.DocVersionsMu.Unlock()
			a.unpinLSPRoot(path)
			if tc.pinned != "" {
				a.pinLSPRoot(path, "go", tc.pinned)
			}

			if got := a.lspOpenStillCurrent("go", path, tc.root, epoch); got != tc.want {
				t.Errorf("lspOpenStillCurrent = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDiagnosticsFromAnotherRootAreRejected(t *testing.T) {
	a, path := missingGoServerApp(t)
	oldRoot := filepath.Join(t.TempDir(), "old")
	newRoot := filepath.Join(t.TempDir(), "new")

	cases := []struct {
		name   string
		root   string
		pinned string
		want   bool
	}{
		{"current root", newRoot, newRoot, true},
		{"old root", oldRoot, newRoot, false},
		{"no pin", oldRoot, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a.unpinLSPRoot(path)
			if tc.pinned != "" {
				a.pinLSPRoot(path, "go", tc.pinned)
			}
			if got := a.lspDiagnosticsStillCurrent(path, tc.root); got != tc.want {
				t.Errorf("lspDiagnosticsStillCurrent = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPulledDiagnosticsFromAnotherRootAreRejected(t *testing.T) {
	a, path := missingGoServerApp(t)
	a.DocVersions[path] = 1
	a.pinLSPRoot(path, "go", "/new")
	key := pullKey{path: path, version: 1}
	diags := []lsp.Diagnostic{{Message: "stale"}}

	if a.setPulledDiagnostics("go", key, pulledDiagnostics{diags: diags, replace: true, root: "/old"}) {
		t.Fatal("applied a pull from a root the document left")
	}
	if !a.setPulledDiagnostics("go", key, pulledDiagnostics{diags: diags, replace: true, root: "/new"}) {
		t.Fatal("rejected a pull from the document's own root")
	}
}

// fakeRestartApp opens a.go on the fake server, which records every did* it
// receives, so a reopen can be observed.
func fakeRestartApp(t *testing.T) (*App, string, string) {
	t.Helper()
	t.Setenv(fakePullServerEnv, "intrafile")
	logPath := filepath.Join(t.TempDir(), "sync.jsonl")
	t.Setenv(fakePullSyncLogEnv, logPath)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	a.Screen = term.NewTcellScreenFrom(sim)
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"go": {Command: []string{exe}, Languages: map[string]string{".go": "go"}},
	}})
	t.Cleanup(func() {
		a.LspManager.Shutdown()
		sim.Fini()
	})
	path := filepath.Join(t.TempDir(), "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	requireNoError(t, a.EditorGroup.OpenFile(path))
	return a, path, logPath
}

func waitDocumentSent(t *testing.T, a *App, path string) {
	t.Helper()
	a.DocVersionsMu.Lock()
	sent := a.lspSent[path]
	a.DocVersionsMu.Unlock()
	select {
	case <-sent:
	case <-time.After(5 * time.Second):
		t.Fatal("document send never finished")
	}
}

func fakeSyncMethods(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var methods []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var msg struct{ Method string }
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, msg.Method)
	}
	return methods
}

// waitFakeSyncMethod waits for the fake server to log method: a notification
// has no reply, so a finished send does not mean the server has read it.
func waitFakeSyncMethod(t *testing.T, path, method string) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		methods := fakeSyncMethods(t, path)
		if slices.Contains(methods, method) {
			return methods
		}
		if time.Now().After(deadline) {
			t.Fatalf("fake server never logged %s; got %v", method, methods)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStopFromSupersededRestartDoesNotReopen(t *testing.T) {
	a, path, logPath := fakeRestartApp(t)
	a.NotifyLSPOpen(path, "go", []string{"package a", ""})
	waitDocumentSent(t, a, path)
	opened := waitFakeSyncMethod(t, logPath, "textDocument/didOpen")

	a.DocVersionsMu.Lock()
	stale := a.lspEpochs["go"]
	a.lspEpochs["go"] = stale + 1
	sent := a.lspSent[path]
	a.DocVersionsMu.Unlock()
	a.handleLSPServerStopped(&LSPServerStopped{Server: "go", WorkDir: filepath.Dir(path), Epoch: stale})

	a.DocVersionsMu.Lock()
	after := a.lspSent[path]
	a.DocVersionsMu.Unlock()
	if after != sent {
		t.Fatal("a stop from a superseded restart reopened the document")
	}
	if got := fakeSyncMethods(t, logPath); len(got) != len(opened) {
		t.Fatalf("sync messages = %v, want only the original open %v", got, opened)
	}
}

func TestReopenSupersededWhileWaitingDoesNotSend(t *testing.T) {
	a, path, logPath := fakeRestartApp(t)
	root := a.lspWorkDirFor(path, "go")
	inFlight := make(chan struct{})
	a.DocVersionsMu.Lock()
	a.DocVersions[path] = 1
	a.lspSent[path] = inFlight
	a.DocVersionsMu.Unlock()

	a.handleLSPServerStopped(&LSPServerStopped{Server: "go", WorkDir: root, Epoch: 0})
	a.DocVersionsMu.Lock()
	reopened := a.lspSent[path]
	a.DocVersionsMu.Unlock()

	// A later restart begins while the reopen waits for an in-flight send.
	a.DocVersionsMu.Lock()
	a.lspEpochs["go"] = 1
	a.DocVersionsMu.Unlock()
	close(inFlight)
	select {
	case <-reopened:
	case <-time.After(5 * time.Second):
		t.Fatal("superseded reopen never released the document")
	}

	if client := a.LspManager.ReadyClient("go", root); client != nil {
		t.Fatal("an obsolete reopen started a client")
	}
	if got := fakeSyncMethods(t, logPath); len(got) != 0 {
		t.Fatalf("an obsolete reopen sent %v", got)
	}
}

func TestChangeFromBeforeRestartIsNotSent(t *testing.T) {
	for _, restarted := range []bool{false, true} {
		t.Run(fmt.Sprintf("restarted=%v", restarted), func(t *testing.T) {
			a, path := missingGoServerApp(t)
			previous := make(chan struct{})
			a.DocVersions[path] = 1
			a.lspSent[path] = previous

			done := a.NotifyLSPChange(path, "go", []string{"package a", "var x"})
			if restarted {
				// The change is still queued while the old server stops.
				a.RestartLSPServer()
			}
			close(previous)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("change never finished")
			}

			// Sending asks the manager for a client, which fails for the
			// missing binary; a dropped change never gets that far.
			tried := a.LspManager.State("go") != lsp.ServerStopped
			if tried == restarted {
				t.Errorf("tried to send = %v after restarted = %v", tried, restarted)
			}
		})
	}
}
