package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingSaveDirectory(t *testing.T) {
	root := t.TempDir()

	t.Run("existing directory", func(t *testing.T) {
		dir, missing := missingSaveDirectory(filepath.Join(root, "file.txt"))
		if missing {
			t.Fatalf("directory %q unexpectedly reported missing", dir)
		}
	})

	t.Run("nested missing directory", func(t *testing.T) {
		want := filepath.Join(root, "one", "two")
		dir, missing := missingSaveDirectory(filepath.Join(want, "file.txt"))
		if !missing {
			t.Fatalf("directory %q was not reported missing", dir)
		}
		if dir != want {
			t.Fatalf("directory = %q, want %q", dir, want)
		}
	})

	t.Run("parent is a file", func(t *testing.T) {
		parent := filepath.Join(root, "not-a-directory")
		if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		dir, missing := missingSaveDirectory(filepath.Join(parent, "file.txt"))
		if missing {
			t.Fatalf("non-directory parent %q was reported as missing", dir)
		}
	})
}
