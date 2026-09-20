package icons

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
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
