package git

import (
	"os/exec"
	"strings"
	"testing"
)

func runOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// buildHistoryRepo produces a repo with, in order: a root commit, a commit on a
// side branch, a commit on the main line, a true merge of the two, and a rename.
// It returns the hashes keyed by subject.
func buildHistoryRepo(t *testing.T) (string, map[string]string) {
	t.Helper()
	dir := setupTestRepo(t)
	// A developer's global commit.gpgsign would block on a passphrase prompt.
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	gitRun(t, dir, "checkout", "-q", "-b", "main")

	writeFile(t, dir, "a.txt", "a\n")
	writeFile(t, dir, "b.txt", "b\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", "root")

	gitRun(t, dir, "checkout", "-q", "-b", "side")
	writeFile(t, dir, "c.txt", "c\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", "side")

	gitRun(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "a.txt", "a\na2\n")
	gitRun(t, dir, "commit", "-qam", "mainline")

	gitRun(t, dir, "merge", "-q", "--no-ff", "side", "-m", "merge")

	gitRun(t, dir, "mv", "a.txt", "renamed.txt")
	gitRun(t, dir, "commit", "-qm", "rename")

	hashes := map[string]string{}
	for _, subject := range []string{"root", "side", "mainline", "merge", "rename"} {
		out, err := runOutput(dir, "log", "--format=%h", "-1", "--grep", "^"+subject+"$", "--all")
		if err != nil {
			t.Fatalf("resolving %q: %v", subject, err)
		}
		hashes[subject] = out
	}
	return dir, hashes
}

func paths(files []FileStatus) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Status + " " + f.Path
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A merge commit is the case that fails without --diff-merges=first-parent:
// plain `git show --name-status` prints nothing at all for one. Drop the flag in
// CommitFiles and this test goes red while every other case here still passes.
func TestCommitFilesMerge(t *testing.T) {
	dir, hashes := buildHistoryRepo(t)
	files, err := CommitFiles(dir, hashes["merge"])
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"A c.txt"}; !equal(paths(files), want) {
		t.Errorf("merge commit: got %v, want %v", paths(files), want)
	}
}

func TestCommitFilesRootCommit(t *testing.T) {
	dir, hashes := buildHistoryRepo(t)
	files, err := CommitFiles(dir, hashes["root"])
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"A a.txt", "A b.txt"}; !equal(paths(files), want) {
		t.Errorf("root commit: got %v, want %v", paths(files), want)
	}
}

func TestCommitFilesModified(t *testing.T) {
	dir, hashes := buildHistoryRepo(t)
	files, err := CommitFiles(dir, hashes["mainline"])
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"M a.txt"}; !equal(paths(files), want) {
		t.Errorf("modified: got %v, want %v", paths(files), want)
	}
}

// --name-status reports a rename as "R100", with the old and new path as two
// further records. The similarity score must not reach FileStatus.Status, which
// the whole UI treats as a single letter.
func TestCommitFilesRename(t *testing.T) {
	dir, hashes := buildHistoryRepo(t)
	files, err := CommitFiles(dir, hashes["rename"])
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("rename: got %v, want one file", paths(files))
	}
	f := files[0]
	if f.Status != "R" {
		t.Errorf("status = %q, want %q", f.Status, "R")
	}
	if f.Path != "renamed.txt" {
		t.Errorf("path = %q, want %q", f.Path, "renamed.txt")
	}
	if f.OldPath != "a.txt" {
		t.Errorf("old path = %q, want %q", f.OldPath, "a.txt")
	}
}

func TestCommitFilesPathWithSpaces(t *testing.T) {
	dir := setupTestRepo(t)
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	writeFile(t, dir, "my file.txt", "hello\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", "spaces")

	out, err := runOutput(dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	files, err := CommitFiles(dir, out)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"A my file.txt"}; !equal(paths(files), want) {
		t.Errorf("got %v, want %v", paths(files), want)
	}
}

func TestCommitFilesUnknownHash(t *testing.T) {
	dir := setupTestRepo(t)
	if _, err := CommitFiles(dir, "deadbee"); err == nil {
		t.Error("expected an error for an unknown hash")
	}
}

func TestCommitFileDiffRename(t *testing.T) {
	dir, hashes := buildHistoryRepo(t)
	files, err := CommitFiles(dir, hashes["rename"])
	if err != nil {
		t.Fatal(err)
	}
	out, err := CommitFileDiff(dir, hashes["rename"], files[0])
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("rename diff is empty; both sides of the rename must be passed to git")
	}
}

func TestCommitFileDiffMerge(t *testing.T) {
	dir, hashes := buildHistoryRepo(t)
	out, err := CommitFileDiff(dir, hashes["merge"], FileStatus{Status: "A", Path: "c.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("merge diff is empty; --diff-merges=first-parent is missing")
	}
}

func TestCommitMessageIncludesSubjectAndBody(t *testing.T) {
	dir := setupTestRepo(t)
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	writeFile(t, dir, "message.txt", "content\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", "Detailed subject", "-m", "First body line.\n\nSecond paragraph.")

	message, err := CommitMessage(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	want := "Detailed subject\n\nFirst body line.\n\nSecond paragraph."
	if message != want {
		t.Fatalf("CommitMessage() = %q, want %q", message, want)
	}
}

// -- only stops option parsing; without --literal-pathspecs these legal file
// names either match unrelated files or act as pathspec magic and disappear.
// CommitFiles itself has no input pathspec, while ShowFile uses Git's
// <ref>:<path> object syntax; exercising both here keeps those distinct forms
// exact as the diff path is hardened.
func TestCommitPathsWithPathspecMagicAreLiteral(t *testing.T) {
	dir := setupTestRepo(t)
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	contents := map[string]string{
		"*.txt":           "literal star\n",
		":(exclude)*.txt": "literal magic\n",
		"other.txt":       "unrelated\n",
	}
	for path, content := range contents {
		writeFile(t, dir, path, content)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", "pathspec names")

	ref, err := runOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	files, err := CommitFiles(dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]FileStatus, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}

	for _, path := range []string{"*.txt", ":(exclude)*.txt"} {
		t.Run(path, func(t *testing.T) {
			status, ok := byPath[path]
			if !ok {
				t.Fatalf("CommitFiles lost literal path %q: %+v", path, files)
			}
			out, err := CommitFileDiff(dir, ref, status)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(out, "diff --git ") != 1 || !strings.Contains(out, "+"+strings.TrimSpace(contents[path])) {
				t.Fatalf("single-file diff did not select literal path %q:\n%s", path, out)
			}
			for other, content := range contents {
				if other != path && strings.Contains(out, "+"+strings.TrimSpace(content)) {
					t.Fatalf("diff for %q included unrelated path %q:\n%s", path, other, out)
				}
			}

			blob, err := ShowFile(dir, path, ref)
			if err != nil {
				t.Fatalf("ShowFile(%q): %v", path, err)
			}
			if blob != contents[path] {
				t.Fatalf("ShowFile(%q) = %q, want %q", path, blob, contents[path])
			}
		})
	}
}

// The parser is exercised directly because the copy record it must handle is
// not reachable through CommitFiles: -M asks for renames only, and the flags
// that do produce a C record either need the source modified in the same commit
// (-C) or compare against the whole tree (--find-copies-harder).
func TestParseNameStatusZ(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single", "M\x00a.txt\x00", []string{"M a.txt"}},
		{"several", "M\x00a.txt\x00A\x00b.txt\x00D\x00c.txt\x00", []string{"M a.txt", "A b.txt", "D c.txt"}},
		{"typechange", "T\x00link\x00", []string{"T link"}},
		{"rename last", "M\x00a.txt\x00R100\x00old\x00new\x00", []string{"M a.txt", "R new"}},
		// The record after a rename or copy is where an off-by-one shows up:
		// consume two fields instead of three and every later record shifts.
		{"rename first", "R100\x00old\x00new\x00M\x00a.txt\x00", []string{"R new", "M a.txt"}},
		{"copy first", "C085\x00source.txt\x00copy.txt\x00M\x00source.txt\x00", []string{"C copy.txt", "M source.txt"}},
		{"path with spaces", "A\x00my file.txt\x00", []string{"A my file.txt"}},
		{"path with a newline", "A\x00two\nlines.txt\x00", []string{"A two\nlines.txt"}},
		{"leading dash", "A\x00--dash.txt\x00", []string{"A --dash.txt"}},
		// A stream cut mid-record must drop the partial one, not invent a path.
		{"truncated pair", "M\x00a.txt\x00A\x00", []string{"M a.txt"}},
		{"truncated rename", "M\x00a.txt\x00R100\x00old\x00", []string{"M a.txt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := paths(parseNameStatusZ([]byte(tt.in)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// The similarity score must not survive into Status: the UI treats it as one
// letter and would render "R100" as the badge.
func TestParseNameStatusZDropsSimilarityScore(t *testing.T) {
	files := parseNameStatusZ([]byte("R100\x00old\x00new\x00"))
	if len(files) != 1 {
		t.Fatalf("got %d records, want 1", len(files))
	}
	if files[0].Status != "R" {
		t.Errorf("status = %q, want %q", files[0].Status, "R")
	}
	if files[0].OldPath != "old" {
		t.Errorf("old path = %q, want %q", files[0].OldPath, "old")
	}
}
