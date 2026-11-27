//go:build linux && !android

package trash

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nativeMoveAndLocate(t *testing.T, path string) string {
	t.Helper()
	dataHome := filepath.Join(filepath.Dir(path), "trash-data")
	t.Setenv("XDG_DATA_HOME", dataHome)
	pattern := filepath.Join(dataHome, "Trash", "info", "*.trashinfo")
	before, _ := filepath.Glob(pattern)
	if err := Move(path); err != nil {
		t.Fatal(err)
	}
	infos, err := filepath.Glob(pattern)
	if err != nil || len(infos) != len(before)+1 {
		t.Fatalf("expected one new recovery record: %v, %v", infos, err)
	}
	added := ""
	for _, info := range infos {
		found := false
		for _, old := range before {
			found = found || info == old
		}
		if !found {
			added = info
		}
	}
	info, err := os.ReadFile(added)
	if err != nil || !strings.Contains(string(info), "Path="+(&url.URL{Path: path}).EscapedPath()+"\n") {
		t.Fatalf("invalid recovery metadata: %q, %v", info, err)
	}
	return filepath.Join(dataHome, "Trash", "files", strings.TrimSuffix(filepath.Base(added), ".trashinfo"))
}

func TestMoveWithoutGIOPreservesFile(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "keep.txt")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Move(path); err == nil {
		t.Fatal("expected unavailable Trash error")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("source changed after failure: %q, %v", data, err)
	}
}
