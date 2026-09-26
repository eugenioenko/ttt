package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestBuildStyleMapFillsTokenSlots(t *testing.T) {
	bold := "bold"
	theme := config.DefaultTheme()
	theme.TokenColors = []config.TokenColor{
		{Scope: config.TokenScopes{"keyword"}, Settings: config.TokenSettings{Foreground: "#123456", FontStyle: &bold}},
	}
	m := BuildStyleMap(theme)
	tokens := NewTokenTheme(theme)
	for i, style := range tokens.Styles {
		if style.Fg != "#123456" || !style.Bold || style.Italic {
			continue
		}
		got := m[term.TokenStyle(i)]
		if got.GetForeground() != tcell.GetColor("#123456") || !got.HasBold() {
			t.Fatalf("token slot %d = %v, want #123456 bold", i, got)
		}
		return
	}
	t.Fatal("no token slot for #123456 bold")
}
