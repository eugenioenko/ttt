package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/appearance"
)

func TestResolveAutoThemeHonorsSides(t *testing.T) {
	if theme, name, ok := ResolveAutoTheme(appearance.Light, "default-light", "default-dark"); !ok || name != "default-light" || theme.Syntax.Keyword.Fg == "" {
		t.Errorf("light = (%q, %v), want (default-light, true) with real colors", name, ok)
	}
	if _, name, ok := ResolveAutoTheme(appearance.Dark, "", "solarized-dark"); !ok || name != "solarized-dark" {
		t.Errorf("dark = (%q, %v), want (solarized-dark, true)", name, ok)
	}
}

func TestResolveAutoThemeFallsBackToBuiltin(t *testing.T) {
	_, name, ok := ResolveAutoTheme(appearance.Light, "no-such-theme", "default-dark")
	if !ok || name != "default-light" {
		t.Errorf("broken light side = (%q, %v), want (default-light, true)", name, ok)
	}
	// Unknown preserves the dark default rather than guessing.
	if _, name, ok := ResolveAutoTheme(appearance.Unknown, "no-such-theme", ""); !ok || name != "default-dark" {
		t.Errorf("unknown = (%q, %v), want (default-dark, true)", name, ok)
	}
}
