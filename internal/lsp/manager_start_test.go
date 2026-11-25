package lsp

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func startingManager(t *testing.T) (*Manager, <-chan error) {
	t.Helper()
	previous := requestTimeout
	requestTimeout = time.Second
	t.Cleanup(func() { requestTimeout = previous })
	m := fakeServerManager(t, "initialize-hang")
	started := make(chan struct{}, 1)
	m.OnLog = func(_, _, message string) {
		if message == "initializing" {
			started <- struct{}{}
		}
	}
	done := make(chan error, 1)
	dir := t.TempDir()
	go func() {
		_, err := m.ClientForLanguage("fake", dir)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not receive initialize")
	}
	t.Cleanup(m.Shutdown)
	return m, done
}

func TestManagerTriggerLookupsDuringInitialization(t *testing.T) {
	m, done := startingManager(t)
	lookedUp := make(chan struct{})
	go func() {
		m.CompletionTriggerCharacters("fake")
		m.SignatureHelpTriggerCharacters("fake")
		close(lookedUp)
	}()
	select {
	case <-lookedUp:
	case <-time.After(200 * time.Millisecond):
		t.Error("typing capability lookup blocked behind initialization")
	}
	if err := <-done; err == nil {
		t.Error("unresponsive server initialized successfully")
	}
	<-lookedUp
}

func TestManagerCachesStartupFailureUntilRestart(t *testing.T) {
	m := managerFor("/nonexistent-lsp-binary")
	var starts atomic.Int32
	m.OnLog = func(_, _, message string) {
		if message == "starting /nonexistent-lsp-binary" {
			starts.Add(1)
		}
	}
	dir := t.TempDir()
	for range 3 {
		if _, err := m.ClientForLanguage("plaintext", dir); err == nil {
			t.Fatal("missing server unexpectedly started")
		}
	}
	if got := starts.Load(); got != 1 {
		t.Errorf("failed startup attempted %d times, want once", got)
	}
	m.Stop("plaintext")
	if _, err := m.ClientForLanguage("plaintext", dir); err == nil {
		t.Fatal("failing server unexpectedly started")
	}
	if got := starts.Load(); got != 2 {
		t.Errorf("explicit restart made %d total attempts, want two", got)
	}
}

func TestManagerStopCancelsInitialization(t *testing.T) {
	m, done := startingManager(t)
	start := time.Now()
	if !m.Stop("fake") {
		t.Error("Stop did not find the starting server")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Stop waited for initialize timeout: %s", elapsed)
	}
	if err := <-done; err == nil {
		t.Error("canceled initialization succeeded")
	}
	if got := m.State("fake"); got != ServerStopped {
		t.Errorf("state = %v, want stopped", got)
	}
}

func TestManagerShutdownCancelsInitialization(t *testing.T) {
	m, done := startingManager(t)
	start := time.Now()
	m.Shutdown()
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Shutdown waited for initialize timeout: %s", elapsed)
	}
	if err := <-done; err == nil {
		t.Error("canceled initialization succeeded")
	}
	if m.ReadyClient("fake") != nil || m.State("fake") != ServerStopped {
		t.Error("shutdown published a late client")
	}
	if _, err := m.ClientForLanguage("fake", t.TempDir()); err == nil {
		t.Error("late request restarted a closed manager")
	}
}

func TestManagerCurrentLookupDoesNotStartStaleServer(t *testing.T) {
	m := fakeServerManager(t, "ok")
	t.Cleanup(m.Shutdown)
	var starts atomic.Int32
	m.OnLog = func(_, _, message string) {
		if strings.HasPrefix(message, "starting ") {
			starts.Add(1)
		}
	}
	client, err := m.ClientForLanguageCurrent("fake", t.TempDir(), func() bool { return false })
	if client != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("client, err = %v, %v; want nil, context.Canceled", client, err)
	}
	if starts.Load() != 0 || m.State("fake") != ServerStopped {
		t.Fatal("a stale lookup started a server")
	}
}

func TestManagerConcurrentStartupSharesClient(t *testing.T) {
	m := fakeServerManager(t, "ok")
	t.Cleanup(m.Shutdown)
	var starts atomic.Int32
	m.OnLog = func(_, _, message string) {
		if message == "ready" {
			starts.Add(1)
		}
	}
	dir := t.TempDir()
	var wg sync.WaitGroup
	clients := make(chan *Client, 8)
	for range cap(clients) {
		wg.Go(func() {
			client, err := m.ClientForLanguage("fake", dir)
			if err != nil {
				t.Error(err)
			}
			clients <- client
		})
	}
	wg.Wait()
	close(clients)
	for client := range clients {
		if client == nil || client != m.ReadyClient("FAKE") {
			t.Error("concurrent request did not share the ready client")
		}
	}
	if starts.Load() != 1 {
		t.Errorf("started %d clients, want one", starts.Load())
	}
}

func TestManagerOldExitCannotRemoveRestartedClient(t *testing.T) {
	m := fakeServerManager(t, "ok")
	t.Cleanup(m.Shutdown)
	dir := t.TempDir()
	if _, err := m.ClientForLanguage("fake", dir); err != nil {
		t.Fatal(err)
	}
	old := m.servers[clientKey("fake", dir)]
	m.Stop("fake")
	current, err := m.ClientForLanguage("fake", dir)
	if err != nil {
		t.Fatal(err)
	}
	m.forget(clientKey("fake", dir), old)
	if m.ReadyClient("fake") != current || m.State("fake") != ServerReady {
		t.Fatal("old exit removed the new client")
	}
}

// A file in another project must get its own process, rooted at that project,
// rather than reusing the first root's server.
func TestManagerStartsClientPerRoot(t *testing.T) {
	m := fakeServerManager(t, "ok")
	t.Cleanup(m.Shutdown)
	firstDir, secondDir := t.TempDir(), t.TempDir()

	first, err := m.ClientForLanguage("fake", firstDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.ClientForLanguage("fake", secondDir)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("second root reused the first root's client")
	}
	if got := m.ReadyClient("fake", firstDir); got != first {
		t.Error("first root did not return its own client")
	}
	if got := m.ReadyClient("fake", secondDir); got != second {
		t.Error("second root did not return its own client")
	}
	again, err := m.ClientForLanguage("fake", firstDir)
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Error("same root did not reuse its client")
	}
	if m.State("fake") != ServerReady {
		t.Errorf("state = %v, want ready", m.State("fake"))
	}
}
