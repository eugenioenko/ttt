package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
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

func TestFilePickerEntriesFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	targetDir := t.TempDir()

	targetFile := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	links := []struct {
		name   string
		target string
	}{
		{name: "linked-dir", target: targetDir},
		{name: "linked-file", target: targetFile},
		{name: "broken-link", target: filepath.Join(dir, "missing.txt")},
	}

	for _, link := range links {
		if err := os.Symlink(link.target, filepath.Join(dir, link.name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	entries := filePickerEntries(dir)
	got := make(map[string]bool)
	dirs := make(map[string]bool)
	for _, entry := range entries {
		got[entry.label] = true
		dirs[entry.label] = entry.dir
	}

	if !got["linked-dir"] || !dirs["linked-dir"] {
		t.Error("symlink to directory should appear as a directory")
	}
	if !got["linked-file"] || dirs["linked-file"] {
		t.Error("symlink to regular file should appear as a file")
	}
	if got["broken-link"] {
		t.Error("broken symlink should not appear")
	}
}

func BenchmarkFilePickerEntries(b *testing.B) {
	dir := b.TempDir()

	for i := 0; i < 1000; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file-%04d.txt", i))
		if err := os.WriteFile(name, nil, 0644); err != nil {
			b.Fatal(err)
		}
	}

	b.Run("Optimized", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			filePickerEntries(dir)
		}
	})

	b.Run("StatBaseline", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkFilePickerEntriesStatBaseline(dir)
		}
	})
}

func benchmarkFilePickerEntriesStatBaseline(dir string) []filePickerEntry {
	var out []filePickerEntry

	if parent := filepath.Dir(dir); parent != dir {
		out = append(out, filePickerEntry{label: "..", path: parent, dir: true})
	}

	items, err := os.ReadDir(dir)
	if err != nil {
		return out
	}

	for _, item := range items {
		path := filepath.Join(dir, item.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if info.IsDir() {
			out = append(out, filePickerEntry{
				label: item.Name(),
				path:  path,
				dir:   true,
			})
		} else if info.Mode().IsRegular() {
			out = append(out, filePickerEntry{
				label: item.Name(),
				path:  path,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].label == out[j].label {
			return false
		}
		if out[i].label == ".." {
			return true
		}
		if out[j].label == ".." {
			return false
		}
		if out[i].dir != out[j].dir {
			return out[i].dir
		}
		hiddenI := strings.HasPrefix(out[i].label, ".")
		hiddenJ := strings.HasPrefix(out[j].label, ".")
		if hiddenI != hiddenJ {
			return !hiddenI
		}
		return strings.ToLower(out[i].label) < strings.ToLower(out[j].label)
	})

	return out
}
