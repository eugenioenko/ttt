package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
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

func TestLspStatusMessage(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMsg   string
		truncated bool
	}{
		{
			name:    "ascii 60 columns unchanged",
			input:   strings.Repeat("a", 60),
			wantMsg: strings.Repeat("a", 60),
		},
		{
			name:      "ascii 61 columns truncated to 60 plus ellipsis",
			input:     strings.Repeat("a", 61),
			wantMsg:   strings.Repeat("a", 60) + "…",
			truncated: true,
		},
		{
			name:      "ascii 62 columns truncated to 60 plus ellipsis",
			input:     strings.Repeat("a", 62),
			wantMsg:   strings.Repeat("a", 60) + "…",
			truncated: true,
		},
		{
			name:      "issue repro wide char straddling cut dropped cleanly",
			input:     strings.Repeat("a", 59) + "界b",
			wantMsg:   strings.Repeat("a", 59) + "…",
			truncated: true,
		},
		{
			name:    "wide chars exactly 60 columns unchanged despite 90 bytes",
			input:   strings.Repeat("界", 30),
			wantMsg: strings.Repeat("界", 30),
		},
		{
			name:      "wide chars 61 columns truncated cleanly to 60",
			input:     strings.Repeat("界", 30) + "a",
			wantMsg:   strings.Repeat("界", 30) + "…",
			truncated: true,
		},
		{
			name:      "wide chars 62 columns truncated cleanly to 60",
			input:     strings.Repeat("界", 31),
			wantMsg:   strings.Repeat("界", 30) + "…",
			truncated: true,
		},
		{
			name:    "ascii plus wide char ending exactly at 60 columns",
			input:   strings.Repeat("a", 58) + "界",
			wantMsg: strings.Repeat("a", 58) + "界",
		},
		{
			name:      "ascii plus wide char ending at 60 with trailing rune",
			input:     strings.Repeat("a", 58) + "界a",
			wantMsg:   strings.Repeat("a", 58) + "界…",
			truncated: true,
		},
		{
			name:    "multibyte narrow char at column 60 unchanged",
			input:   strings.Repeat("a", 59) + "ñ",
			wantMsg: strings.Repeat("a", 59) + "ñ",
		},
		{
			name:      "multibyte narrow char at cut kept whole",
			input:     strings.Repeat("a", 59) + "ñb",
			wantMsg:   strings.Repeat("a", 59) + "ñ…",
			truncated: true,
		},
	}

	wrapper := i18n.Format("lsp.lsp", i18n.Args{"msg": ""})
	wrapperWidth := textwidth.String(wrapper)
	ellipsisWidth := textwidth.String("…")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lspStatusMessage(errors.New(tt.input))
			if !utf8.ValidString(got) {
				t.Fatalf("lspStatusMessage produced invalid UTF-8: %q", got)
			}
			want := i18n.Format("lsp.lsp", i18n.Args{"msg": tt.wantMsg})
			if got != want {
				t.Errorf("lspStatusMessage(%q) = %q, want %q", tt.input, got, want)
			}
			maxWidth := wrapperWidth + 60
			if tt.truncated {
				maxWidth += ellipsisWidth
			}
			if w := textwidth.String(got); w > maxWidth {
				t.Errorf("display width = %d, want <= %d (got %q)", w, maxWidth, got)
			}
		})
	}
}
