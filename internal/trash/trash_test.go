package trash

import (
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
