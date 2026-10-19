package icons

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"golang.org/x/text/width"
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
		if g.Nerd == g.Plain {
			continue
		}
		if got := textwidth.Rune(r[0]); got != privateUseWidth {
			t.Errorf("%s: nerd glyph %q measures %d, want the private-use width %d", name, g.Nerd, got, privateUseWidth)
		}
	}
}

// ambiguousDefaults may render two columns in East Asian locales; they are
// kept because no narrow glyph reads as well.
var ambiguousDefaults = map[Name]bool{
	Commit: true, Lock: true, Dirty: true,
	Class: true, Interface: true, Module: true, Constant: true, Variable: true,
	String: true, Keyword: true, Snippet: true, Symbol: true,
}

func TestPlainGlyphsAreSingleWidth(t *testing.T) {
	for name, g := range table {
		r := []rune(g.Plain)
		if len(r) != 1 || textwidth.Rune(r[0]) != 1 {
			t.Errorf("%s: plain glyph %q must be one single-width rune", name, g.Plain)
			continue
		}
		if width.LookupRune(r[0]).Kind() == width.EastAsianAmbiguous && !ambiguousDefaults[name] {
			t.Errorf("%s: plain glyph %q is East Asian ambiguous width", name, g.Plain)
		}
	}
}

func TestOverrideWins(t *testing.T) {
	defer Configure("", nil)
	warnings := Configure(config.IconsNerdFont, map[string]string{
		"chevron-right": "»",
		"close":         "ab",
		"chevron-down":  "日",
		"nope":          "x",
	})
	if len(warnings) != 3 {
		t.Errorf("warnings = %q, want 3", warnings)
	}
	if got := Rune(ChevronRight); got != '»' {
		t.Errorf("chevron-right = %q, want the override", got)
	}
	if got := Get(config.IconsNone, ChevronRight); got != "»" {
		t.Errorf("plain chevron-right = %q, want the override", got)
	}
	if got := Rune(Close); got != '' {
		t.Errorf("close = %q, want the nerd default", got)
	}
	if got := Rune(ChevronDown); got != '' {
		t.Errorf("chevron-down = %q, want the nerd default", got)
	}
}
