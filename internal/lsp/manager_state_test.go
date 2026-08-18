package lsp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
)

func fakeServer(t *testing.T, body string) (dir, script string) {
	t.Helper()
	dir = t.TempDir()
	script = filepath.Join(dir, "fake.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0755); err != nil {
		t.Fatal(err)
	}
	return dir, script
}

func managerFor(script string) *Manager {
	return NewManager(&config.LSPSettings{
		Servers: map[string]config.LSPServerConfig{
			"plaintext": {Command: []string{script}},
		},
	})
}

func TestManagerStateStartsStopped(t *testing.T) {
	m := managerFor("/nonexistent")
	if got := m.State("plaintext"); got != ServerStopped {
		t.Errorf("State = %v, want ServerStopped", got)
	}
	if got := m.State("never-configured"); got != ServerStopped {
		t.Errorf("State of unknown server = %v, want ServerStopped", got)
	}
}

func TestManagerStateFailedOnCrash(t *testing.T) {
	dir, script := fakeServer(t, "echo boom >&2\nexit 1\n")
	m := managerFor(script)

	if _, err := m.ClientForLanguage("plaintext", dir); err == nil {
		t.Fatal("expected an error from a server that exits immediately")
	}
	if got := m.State("plaintext"); got != ServerFailed {
		t.Errorf("State = %v, want ServerFailed", got)
	}
}

// A server that dies during initialize must not hold the manager lock for the
// request timeout: the read loop has to release in-flight callers before it
// reports the exit, or ClientForLanguage and the exit handler deadlock.
func TestManagerCrashFailsFast(t *testing.T) {
	dir, script := fakeServer(t, "echo boom >&2\nexit 1\n")
	m := managerFor(script)

	start := time.Now()
	if _, err := m.ClientForLanguage("plaintext", dir); err == nil {
		t.Fatal("expected an error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v, want well under the 10s request timeout", elapsed)
	}
}

func TestManagerStateFailedWhenBinaryMissing(t *testing.T) {
	m := managerFor("/nonexistent-lsp-binary")

	if _, err := m.ClientForLanguage("plaintext", t.TempDir()); err == nil {
		t.Fatal("expected an error")
	}
	if got := m.State("plaintext"); got != ServerFailed {
		t.Errorf("State = %v, want ServerFailed", got)
	}
}

func TestManagerStateChangeNotifies(t *testing.T) {
	dir, script := fakeServer(t, "echo boom >&2\nexit 1\n")
	m := managerFor(script)

	changes := make(chan struct{}, 16)
	m.OnStateChange = func() { changes <- struct{}{} }

	m.ClientForLanguage("plaintext", dir)

	// At least starting and failed.
	if len(changes) < 2 {
		t.Errorf("got %d state notifications, want at least 2", len(changes))
	}
}

func TestManagerUnknownLanguageLeavesStateStopped(t *testing.T) {
	m := managerFor("/nonexistent")

	if _, err := m.ClientForLanguage("elvish", t.TempDir()); err == nil {
		t.Fatal("expected an error for an unconfigured language")
	}
	if got := m.State("elvish"); got != ServerStopped {
		t.Errorf("State = %v, want ServerStopped for an unconfigured language", got)
	}
}
