package git

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func commitShapeGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Review", "GIT_AUTHOR_EMAIL=review@example.com",
		"GIT_COMMITTER_NAME=Review", "GIT_COMMITTER_EMAIL=review@example.com",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitShapeWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCommitShapesRootRenameMergeUnicodeAndBinary(t *testing.T) {
	dir := t.TempDir()
	commitShapeGit(t, dir, "init", "-q", "-b", "main")
	commitShapeWrite(t, filepath.Join(dir, "old name.txt"), []byte("old line\nshared line\n"))
	commitShapeWrite(t, filepath.Join(dir, "blob.bin"), bytes.Repeat([]byte{0, 1, 2, 3}, 32))
	commitShapeGit(t, dir, "add", "-A")
	cmd := exec.Command("git", "commit", "-qm", "root")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Review", "GIT_AUTHOR_EMAIL=review@example.com",
		"GIT_COMMITTER_NAME=Review", "GIT_COMMITTER_EMAIL=review@example.com",
		"GIT_AUTHOR_DATE=2026-08-22T03:14:15-04:00", "GIT_COMMITTER_DATE=2026-08-22T03:14:15-04:00",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("root commit: %v\n%s", err, out)
	}
	root := commitShapeGit(t, dir, "rev-parse", "HEAD")
	files, err := CommitFiles(dir, root)
	if err != nil || len(files) != 2 {
		t.Fatalf("root files: files=%+v err=%v", files, err)
	}
	when, err := CommitAuthoredAt(dir, root)
	_, offset := when.Zone()
	if err != nil || when.Second() != 15 || offset != -4*60*60 {
		t.Fatalf("authored time=%v err=%v", when, err)
	}
	for _, file := range files {
		text, err := CommitFileDiff(dir, root, file)
		if err != nil || !strings.Contains(text, file.Path) {
			t.Fatalf("root diff %q: err=%v\n%s", file.Path, err, text)
		}
	}

	commitShapeGit(t, dir, "mv", "old name.txt", "renamed:界.txt")
	commitShapeGit(t, dir, "commit", "-qm", "rename")
	renameRef := commitShapeGit(t, dir, "rev-parse", "HEAD")
	renames, err := CommitFiles(dir, renameRef)
	if err != nil || len(renames) != 1 || renames[0].Status != "R" || renames[0].OldPath != "old name.txt" || renames[0].Path != "renamed:界.txt" {
		t.Fatalf("rename files=%+v err=%v", renames, err)
	}
	if text, err := CommitFileDiff(dir, renameRef, renames[0]); err != nil || !strings.Contains(text, "rename from old name.txt") || !strings.Contains(text, "rename to ") {
		t.Fatalf("rename diff err=%v\n%s", err, text)
	}

	commitShapeGit(t, dir, "checkout", "-qb", "topic")
	commitShapeWrite(t, filepath.Join(dir, "topic.txt"), []byte("topic\n"))
	commitShapeGit(t, dir, "add", "topic.txt")
	commitShapeGit(t, dir, "commit", "-qm", "topic")
	commitShapeGit(t, dir, "checkout", "-q", "main")
	commitShapeWrite(t, filepath.Join(dir, "main.txt"), []byte("main\n"))
	commitShapeGit(t, dir, "add", "main.txt")
	commitShapeGit(t, dir, "commit", "-qm", "main")
	commitShapeGit(t, dir, "merge", "--no-ff", "-qm", "merge", "topic")
	mergeRef := commitShapeGit(t, dir, "rev-parse", "HEAD")
	mergeFiles, err := CommitFiles(dir, mergeRef)
	if err != nil || len(mergeFiles) != 1 || mergeFiles[0].Path != "topic.txt" {
		t.Fatalf("first-parent merge files=%+v err=%v", mergeFiles, err)
	}
}

func TestCommitLogUnbornAndNonRepo(t *testing.T) {
	unborn := t.TempDir()
	commitShapeGit(t, unborn, "init", "-q", "-b", "main")
	entries, err := LogWithError(unborn, 10)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unborn entries=%+v err=%v", entries, err)
	}
	if _, err := LogWithError(t.TempDir(), 10); err == nil {
		t.Fatal("non-repository log unexpectedly succeeded")
	}
	if got, err := time.Parse(time.RFC3339, "2026-08-22T03:14:15-04:00"); err != nil || got.Second() != 15 {
		t.Fatal("test timestamp invalid")
	}
}
