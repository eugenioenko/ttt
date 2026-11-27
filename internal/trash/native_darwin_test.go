package trash

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func nativeMoveAndLocate(t *testing.T, path string) string {
	t.Helper()
	trashed, err := moveMac(context.Background(), path)
	if err != nil || trashed == "" {
		t.Fatalf("native Trash returned %q, %v", trashed, err)
	}
	restored := filepath.Join(t.TempDir(), filepath.Base(path))
	t.Cleanup(func() {
		if err := os.Rename(trashed, restored); err != nil {
			t.Errorf("restore test item %q: %v", trashed, err)
		}
	})
	return trashed
}
