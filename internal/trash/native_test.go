//go:build (linux && !android) || darwin || windows

package trash

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNativeMove(t *testing.T) {
	if os.Getenv("TTT_TEST_TRASH") != "1" {
		t.Skip("set TTT_TEST_TRASH=1 to exercise the system Trash")
	}
	t.Run("name collision", func(t *testing.T) {
		dir, err := os.MkdirTemp(t.TempDir(), "ttt-trash-")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, filepath.Base(dir)+".txt")
		var destinations []string
		for _, text := range []string{"first", "second"} {
			if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			destinations = append(destinations, nativeMoveAndLocate(t, path))
		}
		for i, text := range []string{"first", "second"} {
			if data, err := os.ReadFile(destinations[i]); err != nil || string(data) != text {
				t.Fatalf("collision lost %s version: %q, %v", text, data, err)
			}
		}
	})
	names := []string{"spaces ' 日本語.txt", "folder"}
	if runtime.GOOS != "windows" {
		names = append(names, "quotes \" backslash \\ newline\n.txt", "symlink", "broken-link")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			dir, err := os.MkdirTemp(t.TempDir(), "ttt-trash-")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, filepath.Base(dir)+"-"+name)
			contentPath := path
			if name == "folder" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				contentPath = filepath.Join(path, "child.txt")
			}
			if err := os.WriteFile(contentPath, []byte("recover me"), 0o600); err != nil {
				t.Fatal(err)
			}
			var target string
			if name == "symlink" || name == "broken-link" {
				target = path
				if name == "broken-link" {
					target += "-missing"
				}
				path += "-link"
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			trashed := nativeMoveAndLocate(t, path)
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("source remains after trashing: %v", err)
			}
			if target != "" {
				if got, err := os.Readlink(trashed); err != nil || got != target {
					t.Fatalf("trashed link = %q, %v", got, err)
				}
			} else {
				if name == "folder" {
					trashed = filepath.Join(trashed, "child.txt")
				}
				if data, err := os.ReadFile(trashed); err != nil || string(data) != "recover me" {
					t.Fatalf("trashed content = %q, %v", data, err)
				}
			}
			if target != "" && name != "broken-link" {
				if data, err := os.ReadFile(target); err != nil || string(data) != "recover me" {
					t.Fatalf("symlink target changed: %q, %v", data, err)
				}
			}
		})
	}
}
