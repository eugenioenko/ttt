package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
	"io"
	"os"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
)

const fakeServerEnv = "TTT_LSP_FAKE_SERVER"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeServerEnv); mode != "" {
		if err := runFakeServer(mode); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runFakeServer(mode string) error {
	codec := jsonrpc.NewCodec(os.Stdin, os.Stdout)
	reply := func(id json.RawMessage, result any) error {
		return codec.Send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	for {
		msg, err := codec.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch msg.Method {
		case "initialize":
			if mode == "initialize-hang" {
				fmt.Fprintln(os.Stderr, "initializing")
				continue
			}
			if err := reply(msg.ID, map[string]any{"capabilities": map[string]any{}}); err != nil {
				return err
			}
		case "shutdown":
			if mode == "hang" {
				continue
			}
			if err := reply(msg.ID, nil); err != nil {
				return err
			}
		case "exit":
			return nil
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
	case <-first.conn.Done():
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
	case <-client.conn.Done():
	default:
		t.Fatal("hung server's read loop still running after Stop")
	}
}

// A startup notification blocked on a server that stopped reading stdin must
// not hold Stop past start.done: the write is only released by closing the
// client, which stopStart has to do before waiting.
func TestStopStartReleasesBlockedStartupWrite(t *testing.T) {
	_, clientW := io.Pipe() // no reader, so every frame write blocks
	client := newClient(blockingReader(t), clientW, ClientHooks{})
	start := &serverStart{done: make(chan struct{}), cancel: func() {}}
	start.starting.Store(client)

	writeDone := make(chan struct{})
	go func() {
		_ = client.notify("solution/open", roslynSolutionParams{Solution: "file:///App.sln"})
		close(writeDone)
	}()
	go func() {
		<-writeDone
		close(start.done)
	}()

	stopped := make(chan struct{})
	go func() {
		stopStart("roslyn", start)
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("stopStart waited on a startup write blocked on stdin")
	}
}

func TestManagerStopWithoutClient(t *testing.T) {
	m := managerFor("/nonexistent")
	if m.Stop("plaintext") {
		t.Error("Stop reported a server that was never started")
	}
}
