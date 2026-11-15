package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestThemeCrossMatchesLegacyBorderStyle(t *testing.T) {
	for _, style := range []string{"sharp", "rounded", "double", "bold", "ascii", "none"} {
		t.Run(style, func(t *testing.T) {
			preset := term.BorderSetByName(style)
			chars := config.BorderChars{Horizontal: string(preset.Horizontal), Vertical: string(preset.Vertical)}
			if got := BuildBorderSet(chars).Cross; got != preset.Cross {
				t.Fatalf("cross = %q, want %q", got, preset.Cross)
			}
			chars.Cross = "+"
			if got := BuildBorderSet(chars).Cross; got != '+' {
				t.Fatalf("explicit cross = %q, want +", got)
			}
		})
	}
}
