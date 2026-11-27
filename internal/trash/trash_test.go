package trash

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveRejectsInvalidPaths(t *testing.T) {
	for _, path := range []string{"", "a\x00b"} {
		if err := Move(path); !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("Move(%q) = %v, want invalid path", path, err)
		}
	}
	path := filepath.Join(t.TempDir(), "missing")
	if err := Move(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Move(missing) = %v, want not found", err)
	}
}

func TestCanceledMovePreservesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keep.txt")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := MoveContext(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled move = %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("cancellation changed source: %q, %v", data, err)
	}
}
