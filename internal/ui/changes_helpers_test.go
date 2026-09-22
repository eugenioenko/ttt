package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestGitDecorationRankOrdersStagedBelowLive(t *testing.T) {
	// Lowest to highest: unknown, then each category's staged variant just
	// below its live one, categories ordered new < deleted < modified < conflict.
	order := []term.Style{
		term.StyleDefault,
		term.StyleSuccessStaged,
		term.StyleSuccess,
		term.StyleDangerStaged,
		term.StyleDanger,
		term.StyleWarningStaged,
		term.StyleWarning,
		term.StyleGitConflictStaged,
		term.StyleGitConflict,
	}
	for i := 1; i < len(order); i++ {
		if GitDecorationRank(order[i]) <= GitDecorationRank(order[i-1]) {
			t.Errorf("expected rank(%v) > rank(%v)", order[i], order[i-1])
		}
	}
}

func TestGitDecorationStyle(t *testing.T) {
	cases := []struct {
		status string
		staged bool
		want   term.Style
	}{
		{"M", false, term.StyleWarning},
		{"M", true, term.StyleWarningStaged},
		{"?", false, term.StyleSuccess},
		{"A", true, term.StyleSuccessStaged},
		{"D", false, term.StyleDanger},
		{"D", true, term.StyleDangerStaged},
		{"U", false, term.StyleGitConflict},
		{"U", true, term.StyleGitConflictStaged},
		{"X", true, term.StyleDefault},
	}
	for _, c := range cases {
		if got := GitDecorationStyle(c.status, c.staged); got != c.want {
			t.Errorf("GitDecorationStyle(%q, %v) = %v, want %v", c.status, c.staged, got, c.want)
		}
	}
}

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
