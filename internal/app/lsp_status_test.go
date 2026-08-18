package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

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
