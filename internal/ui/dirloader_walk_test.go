package ui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestWalkDirEntriesMatchesLoadDirEntriesForEveryDirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	for path, content := range map[string]string{
		".gitignore":                  "deps/\n*.log\n",
		"src/main.go":                 "",
		"src/debug.log":               "",
		"src/pkg/util.go":             "",
		"deps/lib/index.js":           "",
		"mixed/keep.txt":              "",
		"mixed/logs/a.log":            "",
		"nested/.gitignore":           "build/\n",
		"nested/build/out.bin":        "",
		"nested/src/debug.log":        "",
		"deps/vendored/.gitignore":    "tmp/\n",
		"deps/vendored/tmp/cache.txt": "",
		"deps/vendored/keep.txt":      "",
		".hidden/file.txt":            "",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, repo := range []string{root, filepath.Join(root, "nested"), filepath.Join(root, "deps", "vendored")} {
		if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
	}

	for _, settings := range []config.ExplorerSettings{
		{},
		{ShowHidden: true},
		{ShowGitIgnored: true},
		{ShowHidden: true, ShowGitIgnored: true},
	} {
		walked, err := WalkDirEntries(context.Background(), []string{root}, settings)
		if err != nil {
			t.Fatal(err)
		}
		for dir, entries := range walked {
			if want := LoadDirEntries(dir, settings); !reflect.DeepEqual(entries, want) {
				t.Errorf("%+v: %s\n got %+v\nwant %+v", settings, dir, entries, want)
			}
		}
		_, deps := walked[filepath.Join(root, "deps", "lib")]
		if deps != settings.ShowGitIgnored {
			t.Errorf("%+v: walked ignored deps/lib = %v", settings, deps)
		}
		if _, ok := walked[filepath.Join(root, ".git")]; ok {
			t.Errorf("%+v: walked .git metadata", settings)
		}
	}
}

func TestWalkDirEntriesStopsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := WalkDirEntries(ctx, []string{t.TempDir()}, config.ExplorerSettings{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
