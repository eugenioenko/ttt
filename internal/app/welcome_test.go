package app

import (
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/eugenioenko/ttt/internal/icons"
)

func TestWelcomeNavigationHintUsesConfiguredIcons(t *testing.T) {
	t.Cleanup(func() { icons.Configure("", nil) })
	if warnings := icons.Configure("", map[string]string{"arrow-up": "u", "arrow-down": "d"}); len(warnings) != 0 {
		t.Fatal(warnings)
	}
	if got := welcomeNavigationHint(false); got != "ud select   enter open" {
		t.Fatalf("navigation hint = %q", got)
	}
	if got := welcomeNavigationHint(true); got != "ud select   enter open   c copy path" {
		t.Fatalf("copy hint = %q", got)
	}
}

func TestPushRecent(t *testing.T) {
	got := pushRecent([]string{"/a", "/b", "/c"}, []string{"/c", "/d"}, maxRecentFolders)
	if want := []string{"/c", "/d", "/a", "/b"}; !slices.Equal(got, want) {
		t.Fatalf("pushRecent = %v, want %v", got, want)
	}
	var many []string
	for i := range maxRecentFolders + 5 {
		many = append(many, strconv.Itoa(i))
	}
	if got := pushRecent(many, []string{"new"}, maxRecentFolders); len(got) != maxRecentFolders || got[0] != "new" {
		t.Fatalf("pushRecent over the cap = %v", got)
	}
}

func TestCloneSource(t *testing.T) {
	tests := []struct {
		input, repo, name string
		haveGH, viaGH     bool
	}{
		{"owner/repo", "owner/repo", "repo", true, true},
		{"owner/repo", "https://github.com/owner/repo", "repo", false, false},
		{" https://github.com/owner/repo.git ", "https://github.com/owner/repo.git", "repo", true, false},
		{"git@github.com:owner/repo.git", "git@github.com:owner/repo.git", "repo", true, false},
		{"https://gitlab.com/group/sub/proj/", "https://gitlab.com/group/sub/proj/", "proj", true, false},
		{"-o/repo", "-o/repo", "repo", true, false},
	}
	for _, tt := range tests {
		repo, name, viaGH := cloneSource(tt.input, tt.haveGH)
		if repo != tt.repo || name != tt.name || viaGH != tt.viaGH {
			t.Errorf("cloneSource(%q, %v) = %q, %q, %v; want %q, %q, %v",
				tt.input, tt.haveGH, repo, name, viaGH, tt.repo, tt.name, tt.viaGH)
		}
	}
}

func TestLayoutWelcomeShrinksTitleBeforeSpacing(t *testing.T) {
	tests := []struct {
		w, h      int
		titleRows int
		gap       int
	}{
		{120, 40, 6, 1},
		{80, 19, 3, 1},
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
