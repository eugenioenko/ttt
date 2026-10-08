package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
)

const fakeServerEnv = "TTT_LSP_FAKE_SERVER"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeServerEnv); mode != "" {
		runFakeServer(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runFakeServer(mode string) {
	codec := NewCodec(os.Stdin, os.Stdout)
	reply := func(id *int, result any) {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *id, "result": result})
		fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}
	for {
		msg, err := codec.Receive()
		if err != nil {
			return
		}
		switch msg.Method {
		case "initialize":
			reply(msg.ID, map[string]any{"capabilities": map[string]any{}})
		case "shutdown":
			if mode == "hang" {
				continue
			}
			reply(msg.ID, nil)
		case "exit":
			return
		}
	}
}

func fakeServerManager(t *testing.T, mode string) *Manager {
	t.Helper()
	t.Setenv(fakeServerEnv, mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return NewManager(&config.LSPSettings{
		Servers: map[string]config.LSPServerConfig{
			"fake": {Command: []string{exe}},
		},
	})
}

func TestManagerStopThenStartsFreshClient(t *testing.T) {
	m := fakeServerManager(t, "ok")
	dir := t.TempDir()

	first, err := m.ClientForLanguage("fake", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Stop("fake") {
		t.Fatal("Stop reported no running server")
	}
	select {
	case <-first.done:
	default:
		t.Fatal("Stop returned before the old read loop exited")
	}
	if got := m.State("fake"); got != ServerStopped {
		t.Errorf("State after Stop = %v, want ServerStopped", got)
	}

	second, err := m.ClientForLanguage("fake", dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Shutdown)
	if second == first {
		t.Fatal("ClientForLanguage returned the stopped client")
	}
	if got := m.State("fake"); got != ServerReady {
		t.Errorf("State after restart = %v, want ServerReady", got)
	}
}

func TestManagerStopKillsHungServer(t *testing.T) {
	prev := shutdownTimeout
	shutdownTimeout = 200 * time.Millisecond
	t.Cleanup(func() { shutdownTimeout = prev })

	m := fakeServerManager(t, "hang")
	client, err := m.ClientForLanguage("fake", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	m.Stop("fake")
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Stop took %v on a hung server", elapsed)
	}
	select {
	case <-client.done:
	default:
		t.Fatal("hung server's read loop still running after Stop")
	}
}

func TestManagerStopWithoutClient(t *testing.T) {
	m := managerFor("/nonexistent")
	if m.Stop("plaintext") {
		t.Error("Stop reported a server that was never started")
	}
}
