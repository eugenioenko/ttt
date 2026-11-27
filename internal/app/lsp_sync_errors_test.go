package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/gdamore/tcell/v3"
)

const fakeSyncFailureEnv = "TTT_APP_FAKE_SYNC_FAILURE"

func runFakeSyncFailureServer() {
	codec := jsonrpc.NewCodec(os.Stdin, os.Stdout)
	for {
		msg, err := codec.Receive()
		if err != nil {
			return
		}
		switch msg.Method {
		case "initialize":
			if err := codec.Send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": map[string]any{"capabilities": map[string]any{}}}); err != nil {
				return
			}
		case "initialized":
			if err := os.Stdin.Close(); err != nil {
				return
			}
			// Keep stdout open so the write failure occurs on a ready client,
			// before the manager has any server-exit notification.
			if _, err := fmt.Fprintln(os.Stderr, "stdin closed"); err != nil {
				return
			}
			time.Sleep(time.Minute)
			return
		}
	}
}

func syncFailureApp(t *testing.T) (*App, *term.SimScreen, string) {
	t.Helper()
	t.Setenv(fakeSyncFailureEnv, "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	path := filepath.Join(t.TempDir(), "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"go": {Command: []string{exe}, Languages: map[string]string{".go": "go"}},
	}})
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	a.Screen = term.NewTcellScreenFrom(sim)
	t.Cleanup(func() {
		a.LspManager.Shutdown()
		a.CloseTasks()
		a.Repository.Close()
		a.EditorGroup.CloseAllTabs()
		sim.Fini()
	})
	ready := make(chan struct{}, 1)
	a.LspManager.OnLog = func(_, _, message string) {
		if message == "stdin closed" {
			ready <- struct{}{}
		}
	}
	if _, err := a.LspManager.ClientForLanguage("go", filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("fake server did not close stdin")
	}
	return a, sim, path
}

func syncFailureEvent(t *testing.T, sim *term.SimScreen) any {
	t.Helper()
	select {
	case ev := <-sim.EventQ():
		interrupt, ok := ev.(*tcell.EventInterrupt)
		if !ok {
			t.Fatalf("unexpected event: %T", ev)
		}
		return interrupt.Data()
	case <-time.After(5 * time.Second):
		t.Fatal("no synchronization failure reported")
		return nil
	}
}

func TestLSPSyncErrorsReachServerOutput(t *testing.T) {
	for _, method := range []string{"didOpen", "didChange", "didSave", "didClose"} {
		t.Run(method, func(t *testing.T) {
			a, sim, path := syncFailureApp(t)
			var done <-chan struct{}
			switch method {
			case "didOpen":
				a.NotifyLSPOpen(path, "go", []string{"package a"})
			case "didChange":
				done = a.NotifyLSPChange(path, "go", []string{"package a"})
			case "didSave":
				a.NotifyLSPSave(path, "go", []string{"package a"})
			case "didClose":
				a.NotifyLSPClose(path, "go")
			}
			event := syncFailureEvent(t, sim)
			output, ok := event.(*OutputLineResult)
			if !ok {
				t.Fatalf("event = %T, want output line", event)
			}
			if output.Source != "lsp:go" || output.Level != "error" || !strings.Contains(output.Message, method+" "+path+": ") {
				t.Fatalf("sync error missing context: %+v", output)
			}
			if method != "didClose" {
				a.DocVersionsMu.Lock()
				sent := a.lspSent[path]
				a.DocVersionsMu.Unlock()
				select {
				case <-sent:
				case <-time.After(5 * time.Second):
					t.Fatal("failed send left document queue blocked")
				}
			}
			if done != nil {
				select {
				case <-done:
				default:
					t.Fatal("change waiter was not released")
				}
			}
			if a.Status.NotifyLevel == view.NotifyError || a.Status.NotifyLevel == view.NotifyWarning {
				t.Fatal("background sync failure replaced the status notification")
			}
		})
	}
}

func TestLSPRestartReportsDocumentReopenFailure(t *testing.T) {
	a, sim, path := syncFailureApp(t)
	a.handleLSPServerStopped(&LSPServerStopped{Server: "go", WorkDir: filepath.Dir(path)})
	event := syncFailureEvent(t, sim)
	output, logged := event.(*OutputLineResult)
	if logged {
		event = syncFailureEvent(t, sim)
	}
	restarted, ok := event.(*LSPServerRestarted)
	if !ok || restarted.Err == nil || !strings.Contains(restarted.Err.Error(), "didOpen "+FileURI(path)) {
		t.Fatalf("restart result lost reopen error: %+v", event)
	}
	if !logged || output.Source != "lsp:go" || output.Level != "error" {
		t.Fatalf("restart did not log reopen failure: %+v", output)
	}
	a.DocVersionsMu.Lock()
	sent := a.lspSent[path]
	a.DocVersionsMu.Unlock()
	select {
	case <-sent:
	default:
		t.Fatal("failed restart left document queue blocked")
	}
	a.handleLSPServerRestarted(restarted)
	want := i18n.Format("status.failedToRestart", i18n.Args{"server": "go", "err": restarted.Err})
	if a.Status.NotifyLevel != view.NotifyWarning || a.Status.Notification != want {
		t.Fatalf("restart failure notice = %q, want %q", a.Status.Notification, want)
	}
}

func TestLSPRestartCanRecoverAfterDocumentReopenFailure(t *testing.T) {
	a, sim, path := syncFailureApp(t)
	a.handleLSPServerStopped(&LSPServerStopped{Server: "go", WorkDir: filepath.Dir(path)})
	_ = syncFailureEvent(t, sim)
	failed, ok := syncFailureEvent(t, sim).(*LSPServerRestarted)
	if !ok || failed.Err == nil {
		t.Fatalf("expected initial reopen failure: %+v", failed)
	}
	a.LspManager.Stop("go")
	t.Setenv(fakeSyncFailureEnv, "")
	t.Setenv(fakePullServerEnv, "intrafile")
	logPath := filepath.Join(t.TempDir(), "sync.jsonl")
	t.Setenv(fakePullSyncLogEnv, logPath)
	a.handleLSPServerStopped(&LSPServerStopped{Server: "go", WorkDir: filepath.Dir(path)})
	retried, ok := syncFailureEvent(t, sim).(*LSPServerRestarted)
	if !ok || retried.Err != nil {
		t.Fatalf("restart retry failed: %+v", retried)
	}
	client := a.LspManager.ReadyClient("go")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := client.Diagnostic(ctx, FileURI(path)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var message struct {
		Method string
		Params lsp.DidOpenTextDocumentParams
	}
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatal(err)
	}
	if message.Method != "textDocument/didOpen" || message.Params.TextDocument.URI != FileURI(path) || message.Params.TextDocument.Version != 1 || message.Params.TextDocument.Text != strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n") {
		t.Fatalf("restart did not reopen current document: %+v", message)
	}
}
