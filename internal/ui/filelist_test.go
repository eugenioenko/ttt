package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestListDirFilesCollectsFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "src", "lib.go"), []byte(""), 0o644)

	files := listDirFiles(dir, "")

	rels := make([]string, len(files))
	for i, f := range files {
		rels[i] = f.Rel
	}
	sort.Strings(rels)

	found := map[string]bool{"main.go": false, filepath.Join("src", "lib.go"): false}
	for _, r := range rels {
		if _, ok := found[r]; ok {
			found[r] = true
		}
	}
	for name, ok := range found {
		if !ok {
			t.Fatalf("expected %q in results, got %v", name, rels)
		}
	}
	for _, f := range files {
		if !filepath.IsAbs(f.Abs) {
			t.Fatalf("expected absolute path, got %q", f.Abs)
		}
	}
}

func TestListFilesWalkDirSkipsDirs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.js"), []byte(""), 0o644)
	for _, skip := range []string{".git", "node_modules", ".cache", "__pycache__"} {
		sub := filepath.Join(dir, skip)
		os.MkdirAll(sub, 0o755)
		os.WriteFile(filepath.Join(sub, "junk.txt"), []byte(""), 0o644)
	}

	files := listFilesWalkDir(dir, "")

	if len(files) != 1 || files[0].Rel != "app.js" {
		rels := make([]string, len(files))
		for i, f := range files {
			rels[i] = f.Rel
		}
		t.Fatalf("expected only [app.js], got %v", rels)
	}
}

func TestListFilesWalkDirPrefix(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0o644)

	files := listFilesWalkDir(dir, "proj/")

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Rel != "proj/a.txt" {
		t.Fatalf("expected rel %q, got %q", "proj/a.txt", files[0].Rel)
	}
}

func TestListWorkspaceFilesMultiRoot(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "a.txt"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir2, "b.txt"), []byte(""), 0o644)

	files := listWorkspaceFiles([]string{dir1, dir2})

	if len(files) < 2 {
		t.Fatalf("expected at least 2 files, got %d", len(files))
	}
	for _, f := range files {
		base1 := filepath.Base(dir1)
		base2 := filepath.Base(dir2)
		if !strings.HasPrefix(f.Rel, base1) && !strings.HasPrefix(f.Rel, base2) {
			t.Fatalf("multi-root file %q missing workspace prefix", f.Rel)
		}
	}
}

func TestListWorkspaceFilesSingleRoot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.txt"), []byte(""), 0o644)

	files := listWorkspaceFiles([]string{dir})

	if len(files) != 1 || files[0].Rel != "x.txt" {
		t.Fatalf("single root should have no prefix, got %v", files)
	}
}

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
	files := []paletteFile{
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
	files := []paletteFile{
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

func TestFuzzyFilterFilesMaxResults(t *testing.T) {
	var files []paletteFile
	for i := 0; i < 200; i++ {
		name := filepath.Join("src", strings.Repeat("a", i%26+1)+".go")
		files = append(files, paletteFile{Rel: name, Abs: "/w/" + name})
	}

	items := fuzzyFilterFiles(files, "a", 5)

	if len(items) > 5 {
		t.Fatalf("expected at most 5 results, got %d", len(items))
	}
}

func TestFuzzyFilterFilesNoMatch(t *testing.T) {
	files := []paletteFile{
		{Rel: "hello.txt", Abs: "/w/hello.txt"},
	}

	items := fuzzyFilterFiles(files, "zzzzz", 10)

	if len(items) != 0 {
		t.Fatalf("expected no matches, got %d", len(items))
	}
}

func TestFuzzyFilterFilesDetail(t *testing.T) {
	files := []paletteFile{
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
