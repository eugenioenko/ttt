package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
)

const (
	fakePullServerEnv = "TTT_APP_FAKE_PULL_SERVER"
	// fakePullRetriggerEnv names a file whose first pull the fake server
	// cancels with retriggerRequest.
	fakePullRetriggerEnv = "TTT_APP_FAKE_PULL_RETRIGGER"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakePullServerEnv); mode != "" {
		runFakePullServer(mode == "interfile")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runFakePullServer answers each pull with one diagnostic naming the file and
// how many times it has been pulled, e.g. "b.go#2".
func runFakePullServer(interFile bool) {
	codec := lsp.NewCodec(os.Stdin, os.Stdout)
	pulls := map[string]int{}
	retrigger := os.Getenv(fakePullRetriggerEnv)
	reply := func(id json.RawMessage, result any) {
		codec.Send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	for {
		msg, err := codec.Receive()
		if err != nil {
			return
		}
		switch msg.Method {
		case "initialize":
			reply(msg.ID, map[string]any{"capabilities": map[string]any{
				"diagnosticProvider": map[string]any{"interFileDependencies": interFile},
			}})
		case "textDocument/diagnostic":
			var params lsp.DocumentDiagnosticParams
			json.Unmarshal(msg.Params, &params)
			name := filepath.Base(params.TextDocument.URI)
			pulls[name]++
			if name == retrigger && pulls[name] == 1 {
				codec.Send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "error": map[string]any{
					"code": -32802, "message": "loading", "data": map[string]any{"retriggerRequest": true},
				}})
				continue
			}
			reply(msg.ID, map[string]any{"kind": "full", "items": []map[string]any{{
				"range":   map[string]any{"start": map[string]int{}, "end": map[string]int{}},
				"message": fmt.Sprintf("%s#%d", name, pulls[name]),
			}}})
		case "shutdown":
			reply(msg.ID, nil)
		case "exit":
			return
		}
	}
}

// fakePullServersApp opens files on fake pull servers named after their
// extensions, in order, so the last file is active.
func fakePullServersApp(t *testing.T, interFile bool, names ...string) (*App, []string) {
	t.Helper()
	mode := "intrafile"
	if interFile {
		mode = "interfile"
	}
	t.Setenv(fakePullServerEnv, mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	servers := map[string]config.LSPServerConfig{}
	for _, n := range names {
		ext := filepath.Ext(n)
		servers[ext[1:]] = config.LSPServerConfig{Command: []string{exe}, Languages: map[string]string{ext: ext[1:]}}
	}
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: servers})
	t.Cleanup(a.LspManager.Shutdown)
	dir := t.TempDir()
	var paths []string
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("package a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// The second open keeps the tab instead of leaving it as a preview.
		a.EditorGroup.OpenFile(p)
		a.EditorGroup.OpenFile(p)
		a.NotifyLSPOpen(p, "", []string{"package a", ""})
		paths = append(paths, p)
	}
	return a, paths
}

// pullServerApp opens a.go and b.go on a fake pull server, with a.go active.
func pullServerApp(t *testing.T, interFile bool) (a *App, active, background string) {
	t.Helper()
	a, paths := fakePullServersApp(t, interFile, "b.go", "a.go")
	return a, paths[1], paths[0]
}

// queueTaskDelivery makes task results wait for drain, as they wait for the
// event loop in the editor.
func queueTaskDelivery(t *testing.T, a *App) (drain func()) {
	t.Helper()
	queue := make(chan func(), 64)
	a.tasks.post = func(run func()) error {
		queue <- run
		return nil
	}
	for _, task := range a.tasks.tasks {
		task.SetPost(a.tasks.post)
	}
	pending := func() bool {
		for _, task := range a.tasks.tasks {
			if p, ok := task.(interface{ Pending() bool }); ok && p.Pending() {
				return true
			}
		}
		return false
	}
	return func() {
		deadline := time.After(10 * time.Second)
		for {
			select {
			case run := <-queue:
				run()
			case <-deadline:
				t.Fatal("tasks still pending after 10s")
			case <-time.After(10 * time.Millisecond):
				if !pending() {
					return
				}
			}
		}
	}
}

func pulledMessage(a *App, path string) string {
	diags := a.EditorGroup.DiagnosticsByPath()[path]
	if len(diags) != 1 {
		return fmt.Sprintf("%d diagnostics", len(diags))
	}
	return diags[0].Message
}

func TestActivePullAlsoPullsOtherOpenDocuments(t *testing.T) {
	for _, tt := range []struct {
		name      string
		interFile bool
		// After a second edit to the active document.
		wantBackground string
	}{
		{name: "inter-file server re-pulls every document", interFile: true, wantBackground: "b.go#2"},
		{name: "other servers skip unchanged documents", interFile: false, wantBackground: "b.go#1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, active, background := pullServerApp(t, tt.interFile)

			a.ScheduleDiagnosticPull(true)
			if got := pulledMessage(a, background); got != "b.go#1" {
				t.Fatalf("background document after the first pull = %q, want b.go#1", got)
			}

			a.NotifyLSPChange(active, "go", []string{"package a", ""})
			a.ScheduleDiagnosticPull(true)
			if got := pulledMessage(a, active); got != "a.go#2" {
				t.Fatalf("active document = %q, want a.go#2", got)
			}
			if got := pulledMessage(a, background); got != tt.wantBackground {
				t.Errorf("background document = %q, want %q", got, tt.wantBackground)
			}
		})
	}
}

func TestDiagnosticRefreshPullsEveryOpenDocument(t *testing.T) {
	a, active, background := pullServerApp(t, false)
	a.ScheduleDiagnosticPull(true)

	a.handleDiagnosticRefresh("go")

	if got := pulledMessage(a, active); got != "a.go#2" {
		t.Errorf("active document = %q, want a.go#2", got)
	}
	if got := pulledMessage(a, background); got != "b.go#2" {
		t.Errorf("background document = %q, want b.go#2", got)
	}
}

func TestReopenedDocumentIsPulledAgain(t *testing.T) {
	a, _, background := pullServerApp(t, false)
	a.ScheduleDiagnosticPull(true)

	a.NotifyLSPClose(background, "go")
	a.NotifyLSPOpen(background, "go", []string{"package a", ""})
	a.pullOpenDocuments("go", "", false)

	if got := pulledMessage(a, background); got != "b.go#2" {
		t.Errorf("reopened document = %q, want b.go#2", got)
	}
}

func TestRefreshSurvivesActivePullApplyingDuringIt(t *testing.T) {
	a, active, background := pullServerApp(t, false)
	a.ScheduleDiagnosticPull(true)
	drain := queueTaskDelivery(t, a)

	a.handleDiagnosticRefresh("go")
	key := pullKey{path: active, version: a.DocVersions[active], epoch: a.lspEpochs["go"]}
	a.applyPulledDiagnostics("go", key, pulledDiagnostics{replace: true}, nil)
	drain()

	if got := pulledMessage(a, background); got == "b.go#1" || !strings.HasPrefix(got, "b.go#") {
		t.Errorf("background document = %q, want it pulled again after b.go#1", got)
	}
}

func TestEditOnAnotherServerKeepsBackgroundPull(t *testing.T) {
	a, paths := fakePullServersApp(t, false, "b.go", "a.go", "c.py")
	drain := queueTaskDelivery(t, a)

	a.handleDiagnosticRefresh("go")
	a.cancelBackgroundPull(paths[2], "")
	drain()

	if got := pulledMessage(a, paths[0]); got != "b.go#1" {
		t.Errorf("b.go = %q, want b.go#1", got)
	}
}

func TestBackgroundPullsForTwoServersBothApply(t *testing.T) {
	a, paths := fakePullServersApp(t, false, "a.go", "b.go", "c.py", "d.py")
	drain := queueTaskDelivery(t, a)

	a.pullOpenDocuments("go", paths[0], true)
	a.pullOpenDocuments("py", paths[2], true)
	drain()

	if got := pulledMessage(a, paths[1]); got != "b.go#1" {
		t.Errorf("b.go = %q, want b.go#1", got)
	}
	if got := pulledMessage(a, paths[3]); got != "d.py#1" {
		t.Errorf("d.py = %q, want d.py#1", got)
	}
}

func TestBackgroundPullRetriesWhenServerAsks(t *testing.T) {
	t.Setenv(fakePullRetriggerEnv, "b.go")
	a, _, background := pullServerApp(t, false)

	a.ScheduleDiagnosticPull(true)

	if got := pulledMessage(a, background); got != "b.go#2" {
		t.Errorf("background document = %q, want b.go#2", got)
	}
}

func TestBackgroundPullDoesNotStartARestartingServer(t *testing.T) {
	for _, restarted := range []bool{false, true} {
		t.Run(fmt.Sprintf("restarted=%v", restarted), func(t *testing.T) {
			a, active, background := pullServerApp(t, false)
			// Each open starts the server itself if it is not running.
			<-a.lspSent[active]
			<-a.lspSent[background]
			drain := queueTaskDelivery(t, a)
			var starts atomic.Int32
			a.LspManager.OnLog = func(_, _, message string) {
				if strings.HasPrefix(message, "starting ") {
					starts.Add(1)
				}
			}

			a.pullOpenDocuments("go", active, false)
			// RestartLSPServer's order.
			if restarted {
				a.DocVersionsMu.Lock()
				a.lspEpochs["go"]++
				a.DocVersionsMu.Unlock()
			}
			a.LspManager.Stop("go")
			drain()

			if started := starts.Load() > 0; started != !restarted {
				t.Errorf("server started = %v, want %v", started, !restarted)
			}
		})
	}
}
