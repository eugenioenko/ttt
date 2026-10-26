package ui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFileDetail(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"main.go", ""},
		{"src/main.go", "src"},
		{"a/b/c.txt", "a/b"},
	}
	for _, tt := range tests {
		if got := fileDetail(tt.input); got != tt.want {
			t.Errorf("fileDetail(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFuzzyFilterFilesEmptyQuery(t *testing.T) {
	files := []PaletteFile{
		{Rel: "a.txt", Abs: "/w/a.txt"},
		{Rel: "b.txt", Abs: "/w/b.txt"},
		{Rel: "c.txt", Abs: "/w/c.txt"},
	}

	items := fuzzyFilterFiles(files, "", 2)

	if len(items) != 2 {
		t.Fatalf("expected 2 items (capped), got %d", len(items))
	}
	if items[0].ID != "/w/a.txt" || items[1].ID != "/w/b.txt" {
		t.Fatalf("expected first two files in order, got %v", items)
	}
}

func TestFuzzyFilterFilesWithQuery(t *testing.T) {
	files := []PaletteFile{
		{Rel: "readme.md", Abs: "/w/readme.md"},
		{Rel: "src/main.go", Abs: "/w/src/main.go"},
		{Rel: "src/util.go", Abs: "/w/src/util.go"},
	}

	items := fuzzyFilterFiles(files, "main", 10)

	if len(items) == 0 {
		t.Fatal("expected at least one match for 'main'")
	}
	if items[0].ID != "/w/src/main.go" {
		t.Fatalf("expected main.go as top result, got %q", items[0].ID)
	}
}

func TestFuzzyFilterFilesIgnoresQueryWhitespace(t *testing.T) {
	for _, rel := range []string{"src/user_profile.go", "src/user-profile.go"} {
		abs := "/w/" + rel
		items := fuzzyFilterFiles([]PaletteFile{{Rel: rel, Abs: abs}}, "user profile", 10)
		if len(items) != 1 || items[0].ID != abs {
			t.Errorf("expected whitespace-separated query to match %q, got %v", abs, items)
		}
	}
}

func TestFuzzyFilterFilesMaxResults(t *testing.T) {
	var files []PaletteFile
	for i := 0; i < 200; i++ {
		name := filepath.Join("src", strings.Repeat("a", i%26+1)+".go")
		files = append(files, PaletteFile{Rel: name, Abs: "/w/" + name})
	}

	items := fuzzyFilterFiles(files, "a", 5)

	if len(items) > 5 {
		t.Fatalf("expected at most 5 results, got %d", len(items))
	}
}

func TestFuzzyFilterFilesNoMatch(t *testing.T) {
	files := []PaletteFile{
		{Rel: "hello.txt", Abs: "/w/hello.txt"},
	}

	items := fuzzyFilterFiles(files, "zzzzz", 10)

	if len(items) != 0 {
		t.Fatalf("expected no matches, got %d", len(items))
	}
}

func TestFuzzyFilterFilesDetail(t *testing.T) {
	files := []PaletteFile{
		{Rel: "src/components/button.tsx", Abs: "/w/src/components/button.tsx"},
	}

	items := fuzzyFilterFiles(files, "", 10)

	if len(items) != 1 {
		t.Fatal("expected 1 item")
	}
	if items[0].Label != "button.tsx" {
		t.Fatalf("expected label 'button.tsx', got %q", items[0].Label)
	}
	if items[0].Detail != "src/components" {
		t.Fatalf("expected detail 'src/components', got %q", items[0].Detail)
	}
}
