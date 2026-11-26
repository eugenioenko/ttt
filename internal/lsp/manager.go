package lsp

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	// starting holds the client while startup is still running, so a stop can
	// close its stdin and release a blocked startup notification.
	starting atomic.Pointer[Client]
	err      error
	exited   bool
}

type Manager struct {
	servers map[string]*serverStart
	config  *config.LSPSettings
	mu      sync.Mutex

	// One entry per running client, keyed like servers. State() aggregates
	// them, since the editor shows a single indicator per server.
	states map[string]ServerState
	closed bool

	// OnDiagnostics receives the root of the client that published, so a
	// result can be rejected once the document has moved to another root.
	OnDiagnostics func(server, root string, params PublishDiagnosticsParams)
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

// clientKey identifies one server process by the root it was initialized in.
// Files under different project roots need their own process: a server roots
// itself at that directory, so a file from another root opened there would be
// analyzed with the wrong project's configuration.
func clientKey(server, workDir string) string {
	if abs, err := filepath.Abs(workDir); err == nil {
		workDir = abs
	}
	return strings.ToLower(server) + "\x00" + filepath.Clean(workDir)
}

func serverForKey(key string) string {
	if i := strings.IndexByte(key, 0); i >= 0 {
		return key[:i]
	}
	return key
}

// State reports the current state of a server. Safe to call from the UI.
func (m *Manager) State(server string) ServerState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.serverStateLocked(strings.ToLower(server))
}

// serverStateLocked combines the states of every root of a server: a root
// still starting is the most informative state, then a ready root (the server
// works for that project), then a failure.
func (m *Manager) serverStateLocked(server string) ServerState {
	state := ServerStopped
	for key, s := range m.states {
		if serverForKey(key) != server {
			continue
		}
		switch s {
		case ServerStarting:
			return ServerStarting
		case ServerReady:
			state = ServerReady
		case ServerFailed:
			if state == ServerStopped {
				state = ServerFailed
			}
		}
	}
	return state
}

func (m *Manager) setStateLocked(key string, state ServerState) bool {
	server := serverForKey(key)
	before := m.serverStateLocked(server)
	m.states[key] = state
	return m.serverStateLocked(server) != before
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
	return m.clientForLanguage(lang, workDir, nil)
}

// ClientForLanguageCurrent refuses to start a server once current reports the
// request as stale. The check runs under the manager lock, so a caller whose
// context can change concurrently cannot have a server started for it in
// between. current must not call back into the manager.
func (m *Manager) ClientForLanguageCurrent(lang, workDir string, current func() bool) (*Client, error) {
	return m.clientForLanguage(lang, workDir, current)
}

func (m *Manager) clientForLanguage(lang, workDir string, current func() bool) (*Client, error) {
	server := strings.ToLower(lang)
	key := clientKey(server, workDir)
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, fmt.Errorf("language server manager is closed")
	}
	if current != nil && !current() {
		m.mu.Unlock()
		return nil, context.Canceled
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
	serverCfg, ok := m.config.Servers[server]
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

	client, err := m.startClient(ctx, server, key, lang, workDir, serverCfg, start)
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
	start.starting.Store(nil)
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

func (m *Manager) startClient(ctx context.Context, server, key, lang, workDir string, serverCfg config.LSPServerConfig, start *serverStart) (*Client, error) {
	slog.Info("lsp starting server", "language", lang, "root", workDir, "command", serverCfg.Command)
	m.log(server, "info", "starting "+strings.Join(serverCfg.Command, " "))

	client, err := NewClient(serverCfg.Command, workDir, ClientHooks{
		OnLog:  func(level, message string) { m.log(server, level, message) },
		OnExit: func() { m.forget(key, start) },
	})
	if err != nil {
		m.log(server, "error", err.Error())
		return nil, fmt.Errorf("start LSP for %s: %w", lang, err)
	}
	start.starting.Store(client)
	client.OnDiagnostics = func(params PublishDiagnosticsParams) {
		if m.OnDiagnostics != nil {
			m.OnDiagnostics(server, workDir, params)
		}
	}
	client.OnApplyEdit = func(edit WorkspaceEdit) ApplyWorkspaceEditResult {
		if m.OnApplyEdit != nil {
			return m.OnApplyEdit(server, edit)
		}
		return ApplyWorkspaceEditResult{FailureReason: "workspace edits are not supported"}
	}
	client.OnDiagnosticRefresh = func() {
		if m.OnDiagnosticRefresh != nil {
			m.OnDiagnosticRefresh(server)
		}
	}

	rootURI := FileURI(workDir)
	if err := client.initialize(ctx, rootURI); err != nil {
		m.log(server, "error", "initialize failed: "+err.Error())
		return client, fmt.Errorf("initialize LSP for %s: %w", lang, err)
	}

	m.log(server, "info", "ready")
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

// ReadyClient never starts a server or waits for initialization. With a
// workDir it returns that root's client; without one, any ready client of the
// server, since capabilities are the same for every root.
func (m *Manager) ReadyClient(key string, workDir ...string) *Client {
	server := strings.ToLower(key)
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(workDir) > 0 && workDir[0] != "" {
		if start := m.servers[clientKey(server, workDir[0])]; start != nil {
			return start.client
		}
		return nil
	}
	keys := make([]string, 0, len(m.servers))
	for k := range m.servers {
		if serverForKey(k) == server {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if client := m.servers[k].client; client != nil {
			return client
		}
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
	server := strings.ToLower(key)
	m.mu.Lock()
	before := m.serverStateLocked(server)
	starts := make(map[string]*serverStart)
	for k, start := range m.servers {
		if serverForKey(k) != server {
			continue
		}
		starts[k] = start
		delete(m.servers, k)
		delete(m.states, k)
	}
	changed := m.serverStateLocked(server) != before
	m.mu.Unlock()
	if len(starts) == 0 {
		return false
	}
	stopStarts(starts)
	m.notifyStateChange(changed)
	m.log(server, "info", "stopped")
	return true
}

// StopIdle stops the client rooted at workDir if idle still reports it unused.
// idle runs under the manager lock, so a lookup cannot hand that client to a
// caller between the check and the stop; idle must not call back into the
// manager.
func (m *Manager) StopIdle(server, workDir string, idle func() bool) bool {
	key := clientKey(server, workDir)
	server = serverForKey(key)
	m.mu.Lock()
	start, ok := m.servers[key]
	if !ok || m.closed || !idle() {
		m.mu.Unlock()
		return false
	}
	before := m.serverStateLocked(server)
	delete(m.servers, key)
	delete(m.states, key)
	changed := m.serverStateLocked(server) != before
	m.mu.Unlock()
	stopStart(key, start)
	m.notifyStateChange(changed)
	m.log(server, "info", "stopped idle server for "+workDir)
	return true
}

func stopStarts(starts map[string]*serverStart) {
	var wg sync.WaitGroup
	for key, start := range starts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stopStart(key, start)
		}()
	}
	wg.Wait()
}

func stopStart(key string, start *serverStart) {
	start.cancel()
	// Cancellation releases initialize, but not a startup notification blocked
	// on a server that stopped reading stdin. Close the in-flight client so the
	// write fails before waiting for startup to finish.
	if client := start.starting.Load(); client != nil {
		select {
		case <-start.done:
			// Startup finished; its client is stopped gracefully below.
		default:
			client.Close()
		}
	}
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
	m.states = make(map[string]ServerState)
	m.mu.Unlock()
	stopStarts(servers)
	m.notifyStateChange(len(servers) > 0)
}
