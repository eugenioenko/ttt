package icons

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

func TestGetResolvesByMode(t *testing.T) {
	if got := Get(config.IconsNerdFont, Branch); got != "" {
		t.Errorf("nerd branch = %q", got)
	}
	for _, mode := range []string{config.IconsNone, ""} {
		if got := Get(mode, Commit); got != "●" {
			t.Errorf("mode %q commit = %q, want plain", mode, got)
		}
	}
}

func TestEveryEntryHasBothForms(t *testing.T) {
	for name, g := range table {
		if g.Plain == "" || g.Nerd == "" {
			t.Errorf("%s: missing a form: %+v", name, g)
		}
	}
}

func TestNerdGlyphsAreSingleRune(t *testing.T) {
	privateUseWidth := textwidth.Rune('\ue000')
	for name, g := range table {
		r := []rune(g.Nerd)
		if len(r) != 1 {
			t.Errorf("%s: nerd glyph %q must be exactly one rune", name, g.Nerd)
			continue
		}
		if got := textwidth.Rune(r[0]); got != privateUseWidth {
			t.Errorf("%s: nerd glyph %q measures %d, want the private-use width %d", name, g.Nerd, got, privateUseWidth)
		}
	}
}
