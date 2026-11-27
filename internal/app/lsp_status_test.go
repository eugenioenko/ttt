package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

func TestLanguageSelectionPreservesLSPResolution(t *testing.T) {
	s := config.DefaultSettings()
	s.LSP.Servers = map[string]config.LSPServerConfig{
		"python": {Command: []string{"missing-python-language-server"}},
		"shell":  {Command: []string{"missing-shell-language-server"}},
	}
	a := buildTestApp(t, s)
	a.LspManager = lsp.NewManager(&s.LSP)
	t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
	path := filepath.Join(t.TempDir(), "deploy")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env python3\nprint('hi')\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	for _, selection := range []string{"source.shell", highlight.PlainText, ""} {
		a.EditorGroup.SetActiveLanguage(selection)
		file, language := a.editorPathLang()
		server, _, ok := a.lspResolve(file, language)
		if !ok || server != "python" || language != "Python" {
			t.Fatalf("selection %q retargeted LSP: %q, %q", selection, server, language)
		}
	}
	var closedLanguage string
	a.EditorGroup.OnFileClose = func(_ string, language string) { closedLanguage = language }
	a.EditorGroup.SetActiveLanguage(highlight.PlainText)
	a.EditorGroup.CloseTab()
	if closedLanguage != "Python" {
		t.Fatalf("closing Plain Text tab lost its document lifecycle: %q", closedLanguage)
	}
}

func TestLspIcon(t *testing.T) {
	tests := []struct {
		name      string
		state     lsp.ServerState
		binaryOK  bool
		wantIcon  string
		wantStyle term.Style
	}{
		{"ready", lsp.ServerReady, true, "◉", 0},
		{"starting", lsp.ServerStarting, true, "◌", 0},
		{"failed", lsp.ServerFailed, true, "⚠", term.StyleDanger},
		{"not started, binary present", lsp.ServerStopped, true, "◌", 0},
		{"not started, binary missing", lsp.ServerStopped, false, "⚠", term.StyleDanger},
		// A crashed server reports failed regardless of the binary still being
		// on disk — presence on disk says nothing about the process.
		{"failed with binary present", lsp.ServerFailed, true, "⚠", term.StyleDanger},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon, style := lspIcon(tt.state, tt.binaryOK)
			if icon != tt.wantIcon {
				t.Errorf("icon = %q, want %q", icon, tt.wantIcon)
			}
			if style != tt.wantStyle {
				t.Errorf("style = %d, want %d", style, tt.wantStyle)
			}
		})
	}
}

// The indicator must not resize as the state changes, or the whole right-hand
// side of the status bar shifts. ⊕, ● and ○ are ambiguous width and would.
func TestLspIconsAreSingleWidth(t *testing.T) {
	for _, state := range []lsp.ServerState{lsp.ServerStopped, lsp.ServerStarting, lsp.ServerReady, lsp.ServerFailed} {
		for _, binaryOK := range []bool{true, false} {
			icon, _ := lspIcon(state, binaryOK)
			if w := textwidth.String(icon); w != 1 {
				t.Errorf("icon %q for state %v has width %d, want 1", icon, state, w)
			}
		}
	}
}

func TestLspStateLabel(t *testing.T) {
	tests := []struct {
		state    lsp.ServerState
		binaryOK bool
		want     string
	}{
		{lsp.ServerReady, true, "running"},
		{lsp.ServerStarting, true, "starting"},
		{lsp.ServerFailed, true, "failed"},
		{lsp.ServerFailed, false, "failed"},
		{lsp.ServerStopped, true, "not started"},
		{lsp.ServerStopped, false, "not installed"},
	}

	for _, tt := range tests {
		if got := lspStateLabel(tt.state, tt.binaryOK); got != tt.want {
			t.Errorf("lspStateLabel(%v, %v) = %q, want %q", tt.state, tt.binaryOK, got, tt.want)
		}
	}
}
