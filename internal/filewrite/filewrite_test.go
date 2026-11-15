package filewrite

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStagedReplacement(t *testing.T) {
	for _, failure := range []string{"", "write", "sync", "close", "replace", "discard"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "document")
			if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			cause := errors.New("injected failure")
			var file *os.File
			staged, err := Stage(target, 0o600, func(f *os.File) error {
				file = f
				if filepath.Dir(f.Name()) != dir {
					t.Fatal("temporary file is not beside destination")
				}
				if _, err := f.WriteString("new"); err != nil {
					return err
				}
				if failure == "write" {
					return cause
				}
				if failure == "sync" {
					return f.Close()
				}
				return nil
			})
			if err == nil {
				defer staged.Discard()
				if got, err := os.ReadFile(target); err != nil || string(got) != "old" {
					t.Fatalf("staging changed destination: %q, %v", got, err)
				}
				if failure == "close" {
					if err := staged.File.Close(); err != nil {
						t.Fatal(err)
					}
				}
				if failure == "discard" {
					staged.Discard()
				} else {
					err = staged.Commit(func(from, to string) error {
						if _, err := staged.File.Stat(); !errors.Is(err, os.ErrClosed) {
							t.Fatalf("file still open at replacement: %v", err)
						}
						if failure == "replace" {
							return cause
						}
						return os.Rename(from, to)
					})
				}
			}
			wantError := failure != "" && failure != "discard"
			if (err != nil) != wantError {
				t.Fatalf("error = %v, want error: %v", err, wantError)
			}
			if (failure == "write" || failure == "replace") && !errors.Is(err, cause) {
				t.Fatalf("lost original error: %v", err)
			}
			want := "old"
			if failure == "" {
				want = "new"
			}
			if got, err := os.ReadFile(target); err != nil || string(got) != want {
				t.Fatalf("destination = %q, %v; want %q", got, err, want)
			}
			if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("temporary descriptor leaked: %v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 || entries[0].Name() != "document" {
				t.Fatalf("unexpected files after save: %v, %v", entries, err)
			}
		})
	}
}
