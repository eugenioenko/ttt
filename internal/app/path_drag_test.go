package app

import (
	"runtime"
	"testing"
)

func TestShellQuote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix quoting")
	}
	tests := map[string]string{
		"/home/u/src/main.go":       "/home/u/src/main.go",
		"/home/u/my file.txt":       "'/home/u/my file.txt'",
		"/home/u/it's.txt":          `'/home/u/it'\''s.txt'`,
		"/home/u/$HOME/a":           "'/home/u/$HOME/a'",
		"/home/u/v1.2_final-draft+": "/home/u/v1.2_final-draft+",
	}
	for in, want := range tests {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}
