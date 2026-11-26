package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

type idleTimer struct {
	fire    func()
	stopped bool
}

// fakeIdleTimers replaces lspAfterFunc so a test decides when idle periods end.
func fakeIdleTimers(t *testing.T) *[]*idleTimer {
	t.Helper()
	var timers []*idleTimer
	previous := lspAfterFunc
	lspAfterFunc = func(_ time.Duration, f func()) func() bool {
		timer := &idleTimer{fire: f}
		timers = append(timers, timer)
		return func() bool {
			wasPending := !timer.stopped
			timer.stopped = true
			return wasPending
		}
	}
	t.Cleanup(func() { lspAfterFunc = previous })
	return &timers
}

func expireIdleTimers(timers []*idleTimer) {
	for _, timer := range timers {
		if !timer.stopped {
			timer.stopped = true
			timer.fire()
		}
	}
}

func idleRootApp(t *testing.T) (*App, *term.SimScreen, string, []string) {
	t.Helper()
	t.Setenv(fakePullServerEnv, "intrafile")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"go": {Command: []string{exe}, Languages: map[string]string{".go": "go"}, RootMarkers: []string{"go.mod"}},
	}})
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	a.Screen = term.NewTcellScreenFrom(sim)
	t.Cleanup(func() {
		a.LspManager.Shutdown()
		sim.Fini()
	})

	project := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, name := range []string{"a.go", "b.go"} {
		path := filepath.Join(project, name)
		if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		requireNoError(t, a.EditorGroup.OpenFile(path))
		a.NotifyLSPOpen(path, "go", []string{"package a", ""})
		waitDocumentSent(t, a, path)
		paths = append(paths, path)
	}
	return a, sim, project, paths
}

func waitRootStopped(t *testing.T, sim *term.SimScreen) *LSPRootStopped {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-sim.EventQ():
			if interrupt, ok := ev.(*tcell.EventInterrupt); ok {
				if stopped, ok := interrupt.Data().(*LSPRootStopped); ok {
					return stopped
				}
			}
		case <-deadline:
			t.Fatal("idle root server never stopped")
			return nil
		}
	}
}

func TestIdleRootServerStopsAfterLastDocumentCloses(t *testing.T) {
	timers := fakeIdleTimers(t)
	a, sim, project, paths := idleRootApp(t)

	a.NotifyLSPClose(paths[0], "go")
	if len(*timers) != 0 {
		t.Fatal("idle stop scheduled while a document of its root was still open")
	}

	a.NotifyLSPClose(paths[1], "go")
	expireIdleTimers(*timers)
	stopped := waitRootStopped(t, sim)
	if stopped.Server != "go" || stopped.Root != project {
		t.Fatalf("stopped %+v, want go at %q", stopped, project)
	}
	if a.LspManager.ReadyClient("go", project) != nil {
		t.Fatal("idle server is still registered")
	}
	if a.LspManager.State("go") != lsp.ServerStopped {
		t.Fatalf("state = %v, want stopped", a.LspManager.State("go"))
	}
}

func TestReopenCancelsEarlierIdlePeriod(t *testing.T) {
	timers := fakeIdleTimers(t)
	a, _, project, paths := idleRootApp(t)
	client := a.LspManager.ReadyClient("go", project)

	for _, path := range paths {
		a.NotifyLSPClose(path, "go")
	}
	first := (*timers)[0]
	a.NotifyLSPOpen(paths[0], "go", []string{"package a", ""})
	waitDocumentSent(t, a, paths[0])
	if !first.stopped {
		t.Fatal("reopening did not cancel the pending idle stop")
	}
	if got := a.LspManager.ReadyClient("go", project); got != client {
		t.Fatal("reopening within the idle timeout did not keep the running server")
	}

	// Closing again starts a new idle period: only its timer may stop the server.
	a.NotifyLSPClose(paths[0], "go")
	if len(*timers) != 2 || (*timers)[1].stopped {
		t.Fatalf("closing again did not schedule a fresh idle stop: %d timers", len(*timers))
	}
}

func TestStoppedRootClearsItsDiagnostics(t *testing.T) {
	a, pulled := missingGoServerApp(t)
	kept := filepath.Join(t.TempDir(), "kept.go")
	cleared := filepath.Join(t.TempDir(), "cleared.go")
	diag := []lsp.Diagnostic{{Message: "unused"}}
	a.applyLSPDiagnostics(&DiagnosticsResult{Server: "go", Path: cleared, Root: "/old", Diagnostics: diag})
	a.applyLSPDiagnostics(&DiagnosticsResult{Server: "go", Path: kept, Root: "/other", Diagnostics: diag})
	a.DocVersions[pulled] = 1
	key := pullKey{path: pulled, version: 1}
	if !a.setPulledDiagnostics("go", key, pulledDiagnostics{diags: diag, replace: true, root: "/old"}) {
		t.Fatal("pulled diagnostics were not applied")
	}

	a.handleLSPRootStopped(&LSPRootStopped{Server: "go", Root: "/old"})

	push, pull := lspDiagnosticsSource("go"), lspPullDiagnosticsSource("go")
	if got := a.EditorGroup.DiagnosticsForSource(push, cleared); len(got) != 0 {
		t.Errorf("stopped root's pushed diagnostics = %v, want cleared", got)
	}
	if got := a.EditorGroup.DiagnosticsForSource(pull, pulled); len(got) != 0 {
		t.Errorf("stopped root's pulled diagnostics = %v, want cleared", got)
	}
	if got := a.EditorGroup.DiagnosticsForSource(push, kept); len(got) != 1 {
		t.Errorf("other root's diagnostics = %v, want kept", got)
	}
}
