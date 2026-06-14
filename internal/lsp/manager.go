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

type Manager struct {
	servers       map[string]*Client
	config        config.LSPSettings
	mu            sync.Mutex
	OnDiagnostics func(params PublishDiagnosticsParams)
}

func NewManager(cfg config.LSPSettings) *Manager {
	return &Manager{
		servers: make(map[string]*Client),
		config:  cfg,
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
	client, err := NewClient(serverCfg.Command, workDir)
	if err != nil {
		return nil, fmt.Errorf("start LSP for %s: %w", lang, err)
	}
	client.OnDiagnostics = m.OnDiagnostics

	rootURI := "file://" + workDir
	if err := client.Initialize(rootURI); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialize LSP for %s: %w", lang, err)
	}

	m.servers[key] = client
	return client, nil
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
	m.servers = make(map[string]*Client)
}
