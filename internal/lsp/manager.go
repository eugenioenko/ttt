package lsp

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
)

// ServerState is what the editor can honestly say about a language server.
// Anything derived from the config alone (the binary exists on disk) is not a
// state: it says nothing about whether the server started or is still alive.
type ServerState int

const (
	ServerStopped ServerState = iota
	ServerStarting
	ServerReady
	ServerFailed
)

type serverStart struct {
	done   chan struct{}
	cancel context.CancelFunc
	client *Client
	err    error
	exited bool
}

type Manager struct {
	servers map[string]*serverStart
	config  *config.LSPSettings
	mu      sync.Mutex

	states map[string]ServerState
	closed bool

	OnDiagnostics func(server string, params PublishDiagnosticsParams)
	OnApplyEdit   func(server string, edit WorkspaceEdit) ApplyWorkspaceEditResult
	// OnDiagnosticRefresh is called off the UI goroutine.
	OnDiagnosticRefresh func(server string)

	// OnLog receives server-reported messages. Called from the client's stderr
	// and read-loop goroutines, so the handler must be goroutine-safe.
	OnLog func(server, level, message string)

	// OnStateChange fires when a server's state changes, so the UI can refresh
	// without polling. Called from client goroutines; must be goroutine-safe.
	OnStateChange func()
}

func NewManager(cfg *config.LSPSettings) *Manager {
	return &Manager{
		servers: make(map[string]*serverStart),
		states:  make(map[string]ServerState),
		config:  cfg,
	}
}

// State reports the current state of a server. Safe to call from the UI.
func (m *Manager) State(server string) ServerState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[strings.ToLower(server)]
}

func (m *Manager) setStateLocked(server string, state ServerState) bool {
	changed := m.states[server] != state
	m.states[server] = state
	return changed
}

func (m *Manager) notifyStateChange(changed bool) {
	if changed && m.OnStateChange != nil {
		m.OnStateChange()
	}
}

func (m *Manager) log(server, level, message string) {
	if m.OnLog != nil {
		m.OnLog(server, level, message)
	}
}

func (m *Manager) ServerConfig(key string) config.LSPServerConfig {
	if cfg, ok := m.config.Servers[strings.ToLower(key)]; ok {
		return cfg
	}
	return config.LSPServerConfig{}
}

func (m *Manager) ClientForLanguage(lang, workDir string) (*Client, error) {
	key := strings.ToLower(lang)
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, fmt.Errorf("language server manager is closed")
	}
	if start, ok := m.servers[key]; ok {
		m.mu.Unlock()
		<-start.done
		m.mu.Lock()
		client, err := start.client, start.err
		if m.servers[key] != start {
			client, err = nil, context.Canceled
		}
		m.mu.Unlock()
		return client, err
	}
	serverCfg, ok := m.config.Servers[key]
	if !ok || len(serverCfg.Command) == 0 {
		m.mu.Unlock()
		return nil, fmt.Errorf("no LSP server command configured for %q", lang)
	}
	ctx, cancel := context.WithCancel(context.Background())
	start := &serverStart{done: make(chan struct{}), cancel: cancel}
	m.servers[key] = start
	changed := m.setStateLocked(key, ServerStarting)
	m.mu.Unlock()
	m.notifyStateChange(changed)

	client, err := m.startClient(ctx, key, lang, workDir, serverCfg, start)
	m.mu.Lock()
	if m.servers[key] != start || m.closed {
		err = context.Canceled
	} else if err == nil && start.exited {
		err = fmt.Errorf("LSP server exited during initialization")
	}
	if err != nil && client != nil {
		m.mu.Unlock()
		client.Close()
		<-client.conn.Done()
		client = nil
		m.mu.Lock()
	}
	start.client, start.err = client, err
	changed = false
	if m.servers[key] == start {
		state := ServerReady
		if err != nil {
			state = ServerFailed
		}
		changed = m.setStateLocked(key, state)
	}
	close(start.done)
	m.mu.Unlock()
	cancel()
	m.notifyStateChange(changed)
	return client, err
}

func (m *Manager) startClient(ctx context.Context, key, lang, workDir string, serverCfg config.LSPServerConfig, start *serverStart) (*Client, error) {
	slog.Info("lsp starting server", "language", lang, "command", serverCfg.Command)
	m.log(key, "info", "starting "+strings.Join(serverCfg.Command, " "))

	client, err := NewClient(serverCfg.Command, workDir, ClientHooks{
		OnLog:  func(level, message string) { m.log(key, level, message) },
		OnExit: func() { m.forget(key, start) },
	})
	if err != nil {
		m.log(key, "error", err.Error())
		return nil, fmt.Errorf("start LSP for %s: %w", lang, err)
	}
	client.OnDiagnostics = func(params PublishDiagnosticsParams) {
		if m.OnDiagnostics != nil {
			m.OnDiagnostics(key, params)
		}
	}
	client.OnApplyEdit = func(edit WorkspaceEdit) ApplyWorkspaceEditResult {
		if m.OnApplyEdit != nil {
			return m.OnApplyEdit(key, edit)
		}
		return ApplyWorkspaceEditResult{FailureReason: "workspace edits are not supported"}
	}
	client.OnDiagnosticRefresh = func() {
		if m.OnDiagnosticRefresh != nil {
			m.OnDiagnosticRefresh(key)
		}
	}

	rootURI := FileURI(workDir)
	if err := client.initialize(ctx, rootURI); err != nil {
		m.log(key, "error", "initialize failed: "+err.Error())
		return client, fmt.Errorf("initialize LSP for %s: %w", lang, err)
	}

	m.log(key, "info", "ready")
	return client, nil
}

func (m *Manager) forget(key string, start *serverStart) {
	m.mu.Lock()
	changed := false
	if m.servers[key] == start {
		start.exited = true
		if start.client != nil {
			start.client = nil
			start.err = fmt.Errorf("LSP server %s exited; restart it to retry", key)
			changed = m.setStateLocked(key, ServerFailed)
		}
	}
	m.mu.Unlock()
	m.notifyStateChange(changed)
}

// ReadyClient never starts a server or waits for initialization.
func (m *Manager) ReadyClient(key string) *Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	if start := m.servers[strings.ToLower(key)]; start != nil {
		return start.client
	}
	return nil
}

// CompletionTriggerCharacters returns the characters serverKey advertises as
// completion triggers, or none while it is not running.
func (m *Manager) CompletionTriggerCharacters(serverKey string) []string {
	if client := m.ReadyClient(serverKey); client != nil {
		return client.CompletionTriggerCharacters()
	}
	return nil
}

func (m *Manager) SignatureHelpTriggerCharacters(serverKey string) []string {
	if client := m.ReadyClient(serverKey); client != nil {
		if chars := client.SignatureHelpTriggerCharacters(); len(chars) > 0 {
			return chars
		}
		return []string{"(", ","}
	}
	return nil
}

func (m *Manager) ResolveLanguage(filePath, languageName string) (serverKey, languageID string, ok bool) {
	ext := strings.ToLower(filepath.Ext(filePath))
	for name, cfg := range m.config.Servers {
		if langID, found := cfg.Languages[ext]; found {
			return name, langID, true
		}
	}
	key := strings.ToLower(languageName)
	if _, found := m.config.Servers[key]; found {
		return key, key, true
	}
	return "", "", false
}

var shutdownTimeout = 3 * time.Second

// Stop returns only after the old read loop has exited, so callers can rely on
// no message from the stopped server being delivered afterwards.
func (m *Manager) Stop(key string) bool {
	key = strings.ToLower(key)
	m.mu.Lock()
	start, ok := m.servers[key]
	delete(m.servers, key)
	changed := m.setStateLocked(key, ServerStopped)
	m.mu.Unlock()
	if !ok {
		return false
	}
	stopStart(key, start)
	m.notifyStateChange(changed)
	m.log(key, "info", "stopped")
	return true
}

func stopStart(key string, start *serverStart) {
	start.cancel()
	<-start.done
	if start.client != nil {
		stopClient(key, start.client)
	}
}

func stopClient(key string, client *Client) {
	done := make(chan struct{})
	go func() {
		if err := client.Shutdown(); err != nil {
			slog.Debug("lsp shutdown error", "language", key, "err", err)
			client.Close()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownTimeout):
		slog.Debug("lsp shutdown timeout, killing", "language", key)
		client.Close()
	}
	select {
	case <-client.conn.Done():
	case <-time.After(shutdownTimeout):
	}
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.closed = true
	servers := m.servers
	m.servers = make(map[string]*serverStart)
	changed := false
	for key := range servers {
		changed = m.setStateLocked(key, ServerStopped) || changed
	}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for key, start := range servers {
		wg.Add(1)
		go func() { defer wg.Done(); stopStart(key, start) }()
	}
	wg.Wait()
	m.notifyStateChange(changed)
}
