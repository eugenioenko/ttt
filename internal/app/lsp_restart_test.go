package app

import (
	"fmt"
	"os"
	"path/filepath"
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
	a.EditorGroup.OpenFile(path)
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
