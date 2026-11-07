package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestFileVisibilityAcrossListingBackends(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if out, err := exec.Command(gitPath, "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for _, name := range []string{"plain.txt", "tracked.txt", "a b.txt", "日本語.txt", ".hidden.txt", ".config/tool.txt", "ignored.txt", "build/output.txt", ".hidden-ignored.txt", "build/.secret.txt", "nested/.git/config"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\nbuild/\n.hidden-ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(gitPath, "-C", dir, "add", "tracked.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	gitOnly := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(gitOnly, filepath.Base(gitPath))); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	backends := []struct {
		name string
		path string
		list func(string, string, FileVisibility) []File
	}{
		{"rg", os.Getenv("PATH"), listDirFiles},
		{"git", gitOnly, listDirFiles},
		{"walk", os.Getenv("PATH"), listFilesWalkDir},
	}
	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			t.Setenv("PATH", backend.path)
			if backend.name == "rg" {
				if _, err := exec.LookPath("rg"); err != nil {
					t.Skip("ripgrep not installed")
				}
			}
			for _, hidden := range []bool{false, true} {
				for _, ignored := range []bool{false, true} {
					t.Run(fmt.Sprintf("hidden=%v/ignored=%v", hidden, ignored), func(t *testing.T) {
						want := []string{"plain.txt", "tracked.txt", "a b.txt", "日本語.txt"}
						if hidden {
							want = append(want, ".hidden.txt", ".config/tool.txt", ".gitignore")
						}
						if ignored {
							want = append(want, "ignored.txt", "build/output.txt")
							if hidden {
								want = append(want, ".hidden-ignored.txt", "build/.secret.txt")
							}
						}
						var got []string
						for _, file := range backend.list(dir, "", FileVisibility{hidden, ignored}) {
							got = append(got, filepath.ToSlash(file.Rel))
							if !filepath.IsAbs(file.Abs) {
								t.Fatalf("file path is not absolute: %q", file.Abs)
							}
						}
						slices.Sort(got)
						slices.Sort(want)
						if !slices.Equal(got, want) {
							t.Fatalf("got %v, want %v", got, want)
						}
					})
				}
			}
		})
	}
}

func TestListFilesWorkspacePrefixes(t *testing.T) {
	dirs := []string{t.TempDir(), t.TempDir()}
	for _, dir := range dirs {
		if err := os.WriteFile(filepath.Join(dir, "file.txt"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, roots := range [][]string{dirs[:1], dirs} {
		files := ListFiles(roots, FileVisibility{})
		if len(files) != len(roots) {
			t.Fatalf("got %d files for %d roots", len(files), len(roots))
		}
		for i, file := range files {
			want := "file.txt"
			if len(roots) > 1 {
				want = filepath.Join(filepath.Base(roots[i]), want)
			}
			if file.Rel != want || file.Abs != filepath.Join(roots[i], "file.txt") {
				t.Fatalf("got %+v, want %s in %s", file, want, roots[i])
			}
		}
	}
}

func TestEmptyRipgrepResultDoesNotFallBack(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"file.txt": "", ".ignore": "*\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if files := ListFiles([]string{dir}, FileVisibility{}); len(files) != 0 {
		t.Fatalf("ignore rules bypassed: %v", files)
	}
}

func TestListFilesWalkDirDependencyExclusions(t *testing.T) {
	dir := t.TempDir()
	filesToCreate := []string{
		"src/app.js",
		"node_modules/pkg/index.js",
		"packages/sub/node_modules/dep/index.js",
		"__pycache__/cache.pyc",
		"pkg/__pycache__/compiled.pyc",
	}
	for _, rel := range filesToCreate {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Default visibility (ShowGitIgnored=false) must exclude node_modules and __pycache__ (including nested)
	defaultFiles := listFilesWalkDir(dir, "", FileVisibility{ShowGitIgnored: false})
	var defaultRels []string
	for _, f := range defaultFiles {
		defaultRels = append(defaultRels, filepath.ToSlash(f.Rel))
	}
	if !slices.Equal(defaultRels, []string{"src/app.js"}) {
		t.Fatalf("expected only [src/app.js] when ShowGitIgnored=false, got %v", defaultRels)
	}

	// ShowGitIgnored=true must include files under node_modules and __pycache__
	allFiles := listFilesWalkDir(dir, "", FileVisibility{ShowGitIgnored: true})
	var allRels []string
	for _, f := range allFiles {
		allRels = append(allRels, filepath.ToSlash(f.Rel))
	}
	slices.Sort(allRels)
	wantAll := append([]string(nil), filesToCreate...)
	slices.Sort(wantAll)
	if !slices.Equal(allRels, wantAll) {
		t.Fatalf("expected all files when ShowGitIgnored=true, got %v, want %v", allRels, wantAll)
	}
}
