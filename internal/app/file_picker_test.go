package app

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

func TestFilePickerEntriesOrdersDirectoriesAndFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zeta", ".hidden-dir", "Alpha"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"zeta.txt", ".hidden.txt", "Alpha.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	gotEntries := filePickerEntries(dir)
	got := make([]string, len(gotEntries))
	for i, entry := range gotEntries {
		got[i] = entry.label
	}
	want := []string{"..", "Alpha", "zeta", ".hidden-dir", "Alpha.txt", "zeta.txt", ".hidden.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for _, entry := range gotEntries[1:4] {
		if !entry.dir {
			t.Errorf("entry %q should be a directory", entry.label)
		}
	}
	for _, entry := range gotEntries[4:] {
		if entry.dir {
			t.Errorf("entry %q should be a file", entry.label)
		}
	}
}

func TestFilePickerEntriesClassifiesSymlinks(t *testing.T) {
	dir := t.TempDir()
	targetDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		"to-dir":  targetDir,
		"to-file": filepath.Join(dir, "file"),
		"broken":  filepath.Join(dir, "missing"),
	}
	for name, dest := range links {
		if err := os.Symlink(dest, filepath.Join(dir, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	got := filePickerEntries(dir)
	byLabel := make(map[string]filePickerEntry, len(got))
	for _, entry := range got {
		byLabel[entry.label] = entry
	}

	if entry, ok := byLabel["to-dir"]; !ok || !entry.dir {
		t.Errorf("symlink to directory = %+v, want a directory entry", entry)
	}
	if entry, ok := byLabel["to-file"]; !ok || entry.dir {
		t.Errorf("symlink to file = %+v, want a file entry", entry)
	}
	if _, ok := byLabel["broken"]; ok {
		t.Error("broken symlink should not appear as a valid entry")
	}
}

func TestFilterFilePickerEntriesKeepsParent(t *testing.T) {
	entries := []filePickerEntry{
		{label: ".."},
		{label: "app.go"},
		{label: "README.md"},
	}
	got := filterFilePickerEntries(entries, "AP")
	if len(got) != 2 || got[0].label != ".." || got[1].label != "app.go" {
		t.Fatalf("filtered entries = %v, want [.. app.go]", got)
	}
}

// BenchmarkFilePickerEntries exercises the listing path. Ordinary entries are
// classified from DirEntry type bits without a per-entry os.Stat, so the only
// stat calls here come from resolving symlink targets.
func BenchmarkFilePickerEntries(b *testing.B) {
	dir := b.TempDir()
	for i := 0; i < 100; i++ {
		name := "file-" + strconv.Itoa(i) + ".txt"
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filePickerEntries(dir)
	}
}
