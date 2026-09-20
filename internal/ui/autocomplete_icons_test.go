package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestCompletionSymbolFollowsIconMode(t *testing.T) {
	for kind := CompletionFunction; kind <= CompletionModule; kind++ {
		if got := kind.Symbol(config.IconsNone); got != '■' {
			t.Errorf("kind %d plain symbol = %q, want ■", kind, got)
		}
		if got := kind.Symbol(config.IconsNerdFont); got == '■' || got == 0 {
			t.Errorf("kind %d nerd symbol = %q, want a Nerd Font glyph", kind, got)
		}
	}
	if CompletionFunction.Symbol(config.IconsNerdFont) != CompletionMethod.Symbol(config.IconsNerdFont) {
		t.Error("function and method should share a glyph, matching the outline")
	}
	if CompletionType.Symbol(config.IconsNerdFont) == CompletionField.Symbol(config.IconsNerdFont) {
		t.Error("type and field should differ")
	}
}
