//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitFileDiffMergeWorksWhenGitShowFirstParentPathspecIsEmpty(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("look up git: %v", err)
	}

	dir := t.TempDir()
	commitShapeGit(t, dir, "init", "-q", "-b", "main")
	commitShapeWrite(t, filepath.Join(dir, "a.txt"), []byte("1\n2\n3\n4\n5\n"))
	commitShapeWrite(t, filepath.Join(dir, "b.txt"), []byte("1\n2\n3\n4\n5\n"))
	commitShapeGit(t, dir, "add", "-A")
	commitShapeGit(t, dir, "commit", "-qm", "init")

	commitShapeGit(t, dir, "checkout", "-qb", "feat")
	commitShapeWrite(t, filepath.Join(dir, "a.txt"), []byte("1\n2\n3\n4\n5\nf\n"))
	commitShapeGit(t, dir, "commit", "-qam", "feat")

	commitShapeGit(t, dir, "checkout", "-q", "main")
	commitShapeWrite(t, filepath.Join(dir, "b.txt"), []byte("1\n2\n3\n4\n5\nm\n"))
	commitShapeGit(t, dir, "commit", "-qam", "mainc")
	commitShapeGit(t, dir, "merge", "-q", "--no-edit", "feat")
	mergeRef := commitShapeGit(t, dir, "rev-parse", "HEAD")

	// Simulate git 2.34, where `git show --diff-merges=first-parent <merge> -- <path>`
	// prints nothing when a pathspec is present.
	binDir := t.TempDir()
	wrapper := `#!/bin/sh
has_show=0
has_first_parent=0
has_pathspec=0
for arg in "$@"; do
  case "$arg" in
    show) has_show=1 ;;
    --diff-merges=first-parent) has_first_parent=1 ;;
    --) has_pathspec=1 ;;
  esac
done
if [ "$has_show" -eq 1 ] && [ "$has_first_parent" -eq 1 ] && [ "$has_pathspec" -eq 1 ]; then
  exit 0
fi
exec "$REAL_GIT" "$@"
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REAL_GIT", realGit)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	files, err := CommitFiles(dir, mergeRef)
	if err != nil || len(files) != 1 || files[0].Path != "a.txt" {
		t.Fatalf("merge CommitFiles = %+v, err = %v", files, err)
	}
	diffText, err := CommitFileDiff(dir, mergeRef, files[0])
	if err != nil {
		t.Fatalf("merge CommitFileDiff err = %v", err)
	}
	if !strings.Contains(diffText, "\n+f\n") {
		t.Fatalf("merge CommitFileDiff missing added line:\n%s", diffText)
	}
}
