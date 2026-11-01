package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
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

func BenchmarkFilePickerEntries(b *testing.B) {
	dir := b.TempDir()
	for i := range 100 {
		name := filepath.Join(dir, fmt.Sprintf("file-%04d.txt", i))
		if err := os.WriteFile(name, nil, 0644); err != nil {
			b.Fatal(err)
		}
	}

	b.Run("Optimized", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			filePickerEntries(dir)
		}
	})

	b.Run("StatBaseline", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
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

func TestFilePickerSyncFromInputPreservesWhitespace(t *testing.T) {
	t.Chdir(t.TempDir())
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("example.txt", nil, 0644); err != nil {
		t.Fatal(err)
	}

	for _, text := range []string{" example.txt", "example.txt ", "example.txt\t", " "} {
		t.Run(text, func(t *testing.T) {
			fp := &filePicker{list: widgets.NewListWidgetFromConfig(widgets.ListConfig{})}
			fp.syncFromInput(text)

			if fp.dir != dir || fp.filter != text {
				t.Fatalf("directory, filter = %q, %q, want %q, %q", fp.dir, fp.filter, dir, text)
			}
			items := fp.list.Config.Items
			if len(items) != 1 || items[0].ID != filepath.Dir(dir) {
				t.Fatalf("filtered listing = %v, want only the parent directory", items)
			}
		})
	}
}

func TestFilePickerSyncFromInputEntersWhitespaceDirectory(t *testing.T) {
	for _, name := range []string{" folder", "folder "} {
		t.Run(name, func(t *testing.T) {
			if runtime.GOOS == "windows" && strings.HasSuffix(name, " ") {
				t.Skip("Windows ordinary paths cannot distinguish filenames with trailing spaces")
			}
			t.Chdir(t.TempDir())
			dir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range []string{"folder", name} {
				if err := os.Mkdir(entry, 0755); err != nil {
					t.Fatal(err)
				}
			}

			fp := &filePicker{list: widgets.NewListWidgetFromConfig(widgets.ListConfig{})}
			fp.syncFromInput(name)

			want := filepath.Join(dir, name)
			if fp.dir != want || fp.filter != "" {
				t.Fatalf("directory, filter = %q, %q, want %q, empty filter", fp.dir, fp.filter, want)
			}
			items := fp.list.Config.Items
			if len(items) != 1 || items[0].ID != dir {
				t.Fatalf("directory listing = %v, want only the parent directory", items)
			}
		})
	}
}
