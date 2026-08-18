package lsp

import (
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

type Manager struct {
	servers map[string]*Client
	config  *config.LSPSettings
	mu      sync.Mutex

	// states has its own lock: ClientForLanguage holds mu for its whole body
	// and records state transitions as it goes, and sync.Mutex is not reentrant.
	states  map[string]ServerState
	stateMu sync.Mutex

	OnDiagnostics func(params PublishDiagnosticsParams)

	// OnLog receives server-reported messages. Called from the client's stderr
	// and read-loop goroutines, so the handler must be goroutine-safe.
	OnLog func(server, level, message string)

	// OnStateChange fires when a server's state changes, so the UI can refresh
	// without polling. Called from client goroutines; must be goroutine-safe.
	OnStateChange func()
}

func NewManager(cfg *config.LSPSettings) *Manager {
	return &Manager{
		servers: make(map[string]*Client),
		states:  make(map[string]ServerState),
		config:  cfg,
	}
}

// State reports the current state of a server. Safe to call from the UI.
func (m *Manager) State(server string) ServerState {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.states[strings.ToLower(server)]
}

func (m *Manager) setState(server string, state ServerState) {
	m.stateMu.Lock()
	changed := m.states[server] != state
	m.states[server] = state
	m.stateMu.Unlock()
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
	defer m.mu.Unlock()

	if client, ok := m.servers[key]; ok {
		return client, nil
	}

	serverCfg, ok := m.config.Servers[key]
	if !ok {
		return nil, fmt.Errorf("no LSP server configured for %q", lang)
	}
	if len(serverCfg.Command) == 0 {
		return nil, fmt.Errorf("empty command for %q", lang)
	}

	slog.Info("lsp starting server", "language", lang, "command", serverCfg.Command)
	m.log(key, "info", "starting "+strings.Join(serverCfg.Command, " "))
	m.setState(key, ServerStarting)

	client, err := NewClient(serverCfg.Command, workDir, ClientHooks{
		OnLog:  func(level, message string) { m.log(key, level, message) },
		OnExit: func() { m.forget(key) },
	})
	if err != nil {
		m.log(key, "error", err.Error())
		m.setState(key, ServerFailed)
		return nil, fmt.Errorf("start LSP for %s: %w", lang, err)
	}
	client.OnDiagnostics = m.OnDiagnostics

	rootURI := "file://" + workDir
	if err := client.Initialize(rootURI); err != nil {
		client.Close()
		m.log(key, "error", "initialize failed: "+err.Error())
		m.setState(key, ServerFailed)
		return nil, fmt.Errorf("initialize LSP for %s: %w", lang, err)
	}

	m.servers[key] = client
	m.setState(key, ServerReady)
	m.log(key, "info", "ready")
	return client, nil
}

// forget drops a server that died on its own, so the next request starts a
// fresh one instead of writing to a dead pipe.
func (m *Manager) forget(key string) {
	m.mu.Lock()
	delete(m.servers, key)
	m.mu.Unlock()
	m.setState(key, ServerFailed)
}

func (m *Manager) SignatureHelpTriggerCharacters(serverKey string) []string {
	m.mu.Lock()
	client, ok := m.servers[serverKey]
	m.mu.Unlock()
	if ok {
		if chars := client.SignatureHelpTriggerCharacters(); len(chars) > 0 {
			return chars
		}
	}
	return []string{"(", ","}
}

func (m *Manager) ResolveLanguage(filePath, chromaLang string) (serverKey, languageID string, ok bool) {
	ext := strings.ToLower(filepath.Ext(filePath))
	for name, cfg := range m.config.Servers {
		if langID, found := cfg.Languages[ext]; found {
			return name, langID, true
		}
	}
	key := strings.ToLower(chromaLang)
	if _, found := m.config.Servers[key]; found {
		return key, key, true
	}
	return "", "", false
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var wg sync.WaitGroup
	for lang, client := range m.servers {
		wg.Add(1)
		go func(lang string, client *Client) {
			defer wg.Done()
			done := make(chan struct{})
			go func() {
				if err := client.Shutdown(); err != nil {
					slog.Debug("lsp shutdown error", "language", lang, "err", err)
					client.Close()
				}
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				slog.Debug("lsp shutdown timeout, killing", "language", lang)
				client.Close()
			}
		}(lang, client)
	}
	wg.Wait()
	for lang := range m.servers {
		m.setState(lang, ServerStopped)
	}
	m.servers = make(map[string]*Client)
}
