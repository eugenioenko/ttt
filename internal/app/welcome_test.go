package app

import (
	"path/filepath"
	"testing"
)

func TestLayoutWelcomeShrinksTitleBeforeSpacing(t *testing.T) {
	tests := []struct {
		w, h      int
		titleRows int
		gap       int
	}{
		{120, 40, 6, 1},
		{80, 17, 3, 1},
		{10, 19, 1, 1},
		{40, 8, 1, 0},
		{40, 4, 0, 0},
	}
	for _, tt := range tests {
		l := layoutWelcome(tt.w, tt.h, len(welcomeCommands), 0, 0)
		if len(l.title) != tt.titleRows || l.gap != tt.gap {
			t.Errorf("layoutWelcome(%d, %d): title rows %d gap %d, want %d and %d",
				tt.w, tt.h, len(l.title), l.gap, tt.titleRows, tt.gap)
		}
		if l.top < 0 || (l.height <= tt.h && l.top+l.height > tt.h) {
			t.Errorf("layoutWelcome(%d, %d): top %d height %d overflows", tt.w, tt.h, l.top, l.height)
		}
	}
}

func TestLayoutWelcomeReservesSectionRows(t *testing.T) {
	plain := layoutWelcome(120, 60, 8, 0, 0)
	sectioned := layoutWelcome(120, 60, 8, 0, 1)
	if sectioned.height != plain.height+welcomeSectionRows {
		t.Errorf("height with a section = %d, want %d", sectioned.height, plain.height+welcomeSectionRows)
	}
}

func TestTildePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := map[string]string{
		filepath.Join(home, "code", "ttt"): "~/code/ttt",
		home:                               home,
		filepath.Dir(home):                 filepath.Dir(home),
		home + "-other":                    home + "-other",
	}
	for in, want := range tests {
		if got := tildePath(in); got != want {
			t.Errorf("tildePath(%q) = %q, want %q", in, got, want)
		}
	}
}
