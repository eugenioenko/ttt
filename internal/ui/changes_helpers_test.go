package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestStatusStyle(t *testing.T) {
	cases := map[string]term.Style{
		"M": term.StyleWarning,
		"A": term.StyleSuccess,
		"?": term.StyleSuccess,
		"R": term.StyleSuccess,
		"C": term.StyleSuccess,
		"D": term.StyleDanger,
		"U": term.StyleGitConflict,
		"X": term.StyleDefault,
	}
	for status, want := range cases {
		if got := StatusStyle(status); got != want {
			t.Errorf("StatusStyle(%q) = %v, want %v", status, got, want)
		}
	}
}
