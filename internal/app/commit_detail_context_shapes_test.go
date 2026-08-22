package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestReadCommitDetailContextUsesTypedPathsAcrossCommitShapes(t *testing.T) {
	dir := t.TempDir()
	testAppGit(t, dir, "init", "-q", "-b", "main")
	testAppGit(t, dir, "config", "user.email", "test@test.com")
	testAppGit(t, dir, "config", "user.name", "Test User")
	testAppGit(t, dir, "config", "commit.gpgsign", "false")

	pathological := ":raw\npath.txt"
	writeContextFixture(t, dir, "normal.txt", []byte("normal root\n"))
	writeContextFixture(t, dir, pathological, []byte("pathological root\n"))
	writeContextFixture(t, dir, "empty.txt", nil)
	writeContextFixture(t, dir, "binary.bin", []byte{'a', 0, 'b'})
	if err := os.Symlink("normal.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "add", "-A")
	testAppGit(t, dir, "commit", "-qm", "root shapes")
	root := git.RevisionIdentity(dir)

	rootCases := []struct {
		file     ui.CommitDetailFile
		kind     ui.CommitDetailContentKind
		newLines []string
	}{
		{file: ui.CommitDetailFile{Status: "A", Path: "normal.txt"}, newLines: []string{"normal root"}},
		{file: ui.CommitDetailFile{Status: "A", Path: pathological}, newLines: []string{"pathological root"}},
		{file: ui.CommitDetailFile{Status: "A", Path: "empty.txt"}, kind: ui.CommitDetailContentEmpty},
		{file: ui.CommitDetailFile{Status: "A", Path: "binary.bin"}, kind: ui.CommitDetailContentBinary},
		{file: ui.CommitDetailFile{Status: "A", Path: "link.txt"}, newLines: []string{"normal.txt"}},
	}
	for _, test := range rootCases {
		result := readCommitDetailContext(context.Background(), 1, "tab", dir, root, 0, test.file)
		if result.Err != "" || result.Canceled || result.ContentKind != test.kind {
			t.Fatalf("root context for %q = %+v", test.file.Path, result)
		}
		if !equalStrings(result.NewLines, test.newLines) || result.OldLines != nil {
			t.Fatalf("root lines for %q old=%q new=%q", test.file.Path, result.OldLines, result.NewLines)
		}
	}

	if err := os.Rename(filepath.Join(dir, "normal.txt"), filepath.Join(dir, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	writeContextFixture(t, dir, "copied.txt", []byte("pathological root\n"))
	if err := os.Remove(filepath.Join(dir, "empty.txt")); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "add", "-A")
	testAppGit(t, dir, "commit", "-qm", "rename copy delete")
	ref := git.RevisionIdentity(dir)

	shapeCases := []struct {
		file               ui.CommitDetailFile
		kind               ui.CommitDetailContentKind
		oldLines, newLines []string
	}{
		{file: ui.CommitDetailFile{Status: "R", OldPath: "normal.txt", Path: "renamed.txt"}, oldLines: []string{"normal root"}, newLines: []string{"normal root"}},
		{file: ui.CommitDetailFile{Status: "C", OldPath: pathological, Path: "copied.txt"}, oldLines: []string{"pathological root"}, newLines: []string{"pathological root"}},
		{file: ui.CommitDetailFile{Status: "D", Path: "empty.txt"}, kind: ui.CommitDetailContentEmpty},
	}
	for _, test := range shapeCases {
		result := readCommitDetailContext(context.Background(), 2, "tab", dir, ref, 0, test.file)
		if result.Err != "" || result.Canceled || result.ContentKind != test.kind {
			t.Fatalf("context for %q = %+v", test.file.Path, result)
		}
		if !equalStrings(result.OldLines, test.oldLines) || !equalStrings(result.NewLines, test.newLines) {
			t.Fatalf("lines for %q old=%q new=%q", test.file.Path, result.OldLines, result.NewLines)
		}
	}
}

func writeContextFixture(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
