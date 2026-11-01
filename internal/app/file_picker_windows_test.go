//go:build windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFilePickerEntriesListsWindowsJunctionDirectories verifies that a
// directory junction is listed as a directory entry. In Go 1.25 DirEntry
// reports a junction as ModeIrregular (not ModeSymlink), so it must be
// resolved with an explicit stat rather than dropped.
func TestFilePickerEntriesListsWindowsJunctionDirectories(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()

	junction := filepath.Join(dir, "junction")
	out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput()
	if err != nil {
		t.Skipf("junction creation unavailable: %v (%s)", err, string(out))
	}
	info, err := os.Lstat(junction)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeIrregular == 0 {
		t.Fatalf("expected a junction (ModeIrregular), got mode %v", info.Mode())
	}

	got := filePickerEntries(dir)
	for _, entry := range got {
		if entry.label == "junction" {
			if !entry.dir {
				t.Fatalf("junction entry = %+v, want a directory entry", entry)
			}
			return
		}
	}
	t.Fatal("junction not listed in file picker entries")
}
