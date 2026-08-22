package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// FormatRelativeTime
// ---------------------------------------------------------------------------

func TestFormatRelativeTime(t *testing.T) {
	tests := []struct {
		name   string
		offset time.Duration
		want   string
	}{
		// just now
		{"zero seconds", 0, "just now"},
		{"30 seconds ago", 30 * time.Second, "just now"},
		{"59 seconds ago", 59 * time.Second, "just now"},

		// minutes
		{"1 minute ago", 1 * time.Minute, "1 minute ago"},
		{"2 minutes ago", 2 * time.Minute, "2 minutes ago"},
		{"30 minutes ago", 30 * time.Minute, "30 minutes ago"},
		{"59 minutes ago", 59 * time.Minute, "59 minutes ago"},

		// hours
		{"1 hour ago", 1 * time.Hour, "1 hour ago"},
		{"2 hours ago", 2 * time.Hour, "2 hours ago"},
		{"12 hours ago", 12 * time.Hour, "12 hours ago"},
		{"23 hours ago", 23 * time.Hour, "23 hours ago"},

		// days
		{"1 day ago", 24 * time.Hour, "1 day ago"},
		{"2 days ago", 2 * 24 * time.Hour, "2 days ago"},
		{"7 days ago", 7 * 24 * time.Hour, "7 days ago"},
		{"29 days ago", 29 * 24 * time.Hour, "29 days ago"},

		// months
		{"30 days = 1 month", 30 * 24 * time.Hour, "1 month ago"},
		{"45 days = 1 month", 45 * 24 * time.Hour, "1 month ago"},
		{"60 days = 2 months", 60 * 24 * time.Hour, "2 months ago"},
		{"150 days = 5 months", 150 * 24 * time.Hour, "5 months ago"},
		{"364 days = 12 months", 364 * 24 * time.Hour, "12 months ago"},

		// years
		{"365 days = 1 year", 365 * 24 * time.Hour, "1 year ago"},
		{"730 days = 2 years", 730 * 24 * time.Hour, "2 years ago"},
		{"1825 days = 5 years", 1825 * 24 * time.Hour, "5 years ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRelativeTime(time.Now().Add(-tt.offset))
			if got != tt.want {
				t.Errorf("FormatRelativeTime(now - %v) = %q, want %q", tt.offset, got, tt.want)
			}
		})
	}
}

func TestFormatRelativeTimeBoundary(t *testing.T) {
	// Boundary: exactly at minute threshold should not be "just now"
	got := FormatRelativeTime(time.Now().Add(-time.Minute))
	if got != "1 minute ago" {
		t.Errorf("FormatRelativeTime(now - 1m) = %q, want %q", got, "1 minute ago")
	}

	// Boundary: exactly at hour threshold
	got = FormatRelativeTime(time.Now().Add(-time.Hour))
	if got != "1 hour ago" {
		t.Errorf("FormatRelativeTime(now - 1h) = %q, want %q", got, "1 hour ago")
	}
}

// ---------------------------------------------------------------------------
// Helper: set up a temporary git repo
// ---------------------------------------------------------------------------

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test User"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v failed: %v\n%s", args, err, out)
		}
	}
	return dir
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// StatusFiles
// ---------------------------------------------------------------------------

func TestStatusFilesEmpty(t *testing.T) {
	dir := setupTestRepo(t)
	// Empty repo with nothing staged or modified
	writeFile(t, dir, "initial.txt", "hello\n")
	gitRun(t, dir, "add", "initial.txt")
	gitRun(t, dir, "commit", "-m", "init")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected no files, got %d: %+v", len(files), files)
	}
}

func TestStatusFilesContextHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := StatusFilesContext(ctx, setupTestRepo(t)); err == nil {
		t.Fatal("canceled status context returned success")
	}
}

func TestRevisionIdentityTracksHead(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "initial.txt", "hello\n")
	gitRun(t, dir, "add", "initial.txt")
	gitRun(t, dir, "commit", "-m", "init")
	initial := RevisionIdentity(dir)
	if initial == "" {
		t.Fatal("committed repository has an empty revision identity")
	}
	writeFile(t, dir, "next.txt", "next\n")
	gitRun(t, dir, "add", "next.txt")
	gitRun(t, dir, "commit", "-m", "next")
	if next := RevisionIdentity(dir); next == initial || next == "" {
		t.Fatalf("revision identity after commit = %q, initial = %q", next, initial)
	}
}

func TestRevisionIdentityAllowsUnbornRepository(t *testing.T) {
	dir := setupTestRepo(t)
	identity, err := RevisionIdentityContext(context.Background(), dir)
	if err != nil || identity != "" {
		t.Fatalf("unborn repository identity = (%q, %v), want empty success", identity, err)
	}
	if branch, err := BranchNameContext(context.Background(), dir); err != nil || branch == "" {
		t.Fatalf("unborn repository branch = (%q, %v), want symbolic branch", branch, err)
	}
}

func TestRevisionIdentityRequiresVerifiedUnbornHead(t *testing.T) {
	bin := t.TempDir()
	script := `#!/bin/sh
case "$*" in
  *"rev-parse --verify HEAD"*) exit 73 ;;
  *"symbolic-ref -q HEAD"*) printf 'refs/heads/main\n'; exit 0 ;;
  *"show-ref --verify --quiet refs/heads/main"*) exit 0 ;;
esac
exit 99
`
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	if identity, err := RevisionIdentityContext(context.Background(), "/repo"); err == nil {
		t.Fatalf("failed HEAD read returned empty success: identity=%q", identity)
	}
	if entries, err := LogWithErrorContext(context.Background(), "/repo", 10); err == nil {
		t.Fatalf("failed HEAD read returned successful empty log: entries=%v", entries)
	}
}

func TestStatusFilesModified(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "file.txt", "original\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Modify the file (unstaged)
	writeFile(t, dir, "file.txt", "modified\n")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "file.txt" && f.Status == "M" && !f.Staged {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unstaged modified file.txt, got %+v", files)
	}
}

func TestStatusFilesStagedModified(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "file.txt", "original\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Modify and stage
	writeFile(t, dir, "file.txt", "modified\n")
	gitRun(t, dir, "add", "file.txt")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "file.txt" && f.Status == "M" && f.Staged {
			found = true
		}
	}
	if !found {
		t.Errorf("expected staged modified file.txt, got %+v", files)
	}
}

func TestStatusFilesAdded(t *testing.T) {
	dir := setupTestRepo(t)
	// Need an initial commit so HEAD exists
	writeFile(t, dir, "initial.txt", "init\n")
	gitRun(t, dir, "add", "initial.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Add a new file and stage it
	writeFile(t, dir, "new.txt", "new content\n")
	gitRun(t, dir, "add", "new.txt")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "new.txt" && f.Status == "A" && f.Staged {
			found = true
		}
	}
	if !found {
		t.Errorf("expected staged added new.txt, got %+v", files)
	}
}

func TestStatusFilesDeleted(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "doomed.txt", "will be deleted\n")
	gitRun(t, dir, "add", "doomed.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Delete the file (unstaged delete)
	os.Remove(filepath.Join(dir, "doomed.txt"))

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "doomed.txt" && f.Status == "D" && !f.Staged {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unstaged deleted doomed.txt, got %+v", files)
	}
}

func TestStatusFilesStagedDelete(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "doomed.txt", "will be deleted\n")
	gitRun(t, dir, "add", "doomed.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Stage the deletion
	gitRun(t, dir, "rm", "doomed.txt")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "doomed.txt" && f.Status == "D" && f.Staged {
			found = true
		}
	}
	if !found {
		t.Errorf("expected staged deleted doomed.txt, got %+v", files)
	}
}

func TestStatusFilesUntracked(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "initial.txt", "init\n")
	gitRun(t, dir, "add", "initial.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Create an untracked file
	writeFile(t, dir, "untracked.txt", "untracked\n")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "untracked.txt" && f.Status == "?" && !f.Staged {
			found = true
		}
	}
	if !found {
		t.Errorf("expected untracked untracked.txt, got %+v", files)
	}
}

func TestStatusFilesRenamed(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "old_name.txt", "some content that is long enough to be detected as a rename rather than add+delete\n")
	gitRun(t, dir, "add", "old_name.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Rename the file via git mv (staged rename)
	gitRun(t, dir, "mv", "old_name.txt", "new_name.txt")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "new_name.txt" && f.OldPath == "old_name.txt" && f.Status == "R" && f.Staged {
			found = true
		}
	}
	if !found {
		t.Errorf("expected staged renamed old_name.txt -> new_name.txt, got %+v", files)
	}
}

func TestStatusFilesWithSpaces(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "initial.txt", "init\n")
	gitRun(t, dir, "add", "initial.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Create a file with spaces in the name.
	writeFile(t, dir, "file with spaces.txt", "content\n")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "file with spaces.txt" && f.Status == "?" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected raw untracked file with spaces, got %+v", files)
	}
}

func TestStatusFilesReturnsRawPathIdentity(t *testing.T) {
	dir := setupTestRepo(t)
	tracked := map[string]string{
		"ordinary.txt":        "ordinary\n",
		"界-wide.txt":          "wide\n",
		"delete-staged.txt":   "delete staged\n",
		"delete-unstaged.txt": "delete unstaged\n",
		"rename-old.txt":      "rename\n",
	}
	for path, content := range tracked {
		writeFile(t, dir, path, content)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "raw paths")

	writeFile(t, dir, "ordinary.txt", "staged\n")
	gitRun(t, dir, "add", "--", "ordinary.txt")
	writeFile(t, dir, "ordinary.txt", "staged and unstaged\n")
	writeFile(t, dir, "界-wide.txt", "changed\n")
	if err := os.Remove(filepath.Join(dir, "delete-staged.txt")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "--", "delete-staged.txt")
	if err := os.Remove(filepath.Join(dir, "delete-unstaged.txt")); err != nil {
		t.Fatal(err)
	}
	renamePath := "rename -> new\n界.txt"
	gitRun(t, dir, "mv", "rename-old.txt", renamePath)

	untracked := []string{
		"quote\"name.txt",
		"line\nbreak.txt",
		`back\slash.txt`,
		"colon:name.txt",
		"-leading.txt",
		"新規/深い/同名.go",
	}
	for _, path := range untracked {
		fullPath := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(path), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus := func(path, oldPath, status string, staged bool) {
		t.Helper()
		for _, file := range files {
			if file.Path == path && file.OldPath == oldPath && file.Status == status && file.Staged == staged {
				return
			}
		}
		t.Errorf("missing raw status path=%q old=%q status=%q staged=%v in %+v", path, oldPath, status, staged, files)
	}
	assertStatus("ordinary.txt", "", "M", true)
	assertStatus("ordinary.txt", "", "M", false)
	assertStatus("界-wide.txt", "", "M", false)
	assertStatus("delete-staged.txt", "", "D", true)
	assertStatus("delete-unstaged.txt", "", "D", false)
	assertStatus(renamePath, "rename-old.txt", "R", true)
	for _, path := range untracked {
		assertStatus(path, "", "?", false)
	}
}

func TestParseStatusPorcelainZUsesDestinationThenSourceForRenameAndCopy(t *testing.T) {
	files := parseStatusPorcelainZ([]byte("R  renamed\n界.txt\x00old:name.txt\x00 C copied\\name.txt\x00source\"name.txt\x00"))
	if len(files) != 2 {
		t.Fatalf("rename/copy statuses = %+v", files)
	}
	if got := files[0]; got.Status != "R" || !got.Staged || got.Path != "renamed\n界.txt" || got.OldPath != "old:name.txt" {
		t.Fatalf("rename order = %+v", got)
	}
	if got := files[1]; got.Status != "C" || got.Staged || got.Path != `copied\name.txt` || got.OldPath != `source"name.txt` {
		t.Fatalf("copy order = %+v", got)
	}
}

func TestParseStatusPorcelainZRejectsEmptyPaths(t *testing.T) {
	valid := "?? valid\n界.txt\x00"
	tests := []struct {
		name string
		tail string
	}{
		{name: "ordinary destination", tail: " M \x00"},
		{name: "rename destination", tail: "R  \x00old.txt\x00"},
		{name: "rename source", tail: "R  new.txt\x00\x00"},
		{name: "staged copy destination", tail: "C  \x00old.txt\x00"},
		{name: "staged copy source", tail: "C  new.txt\x00\x00"},
		{name: "unstaged copy destination", tail: " C \x00old.txt\x00"},
		{name: "unstaged copy source", tail: " C new.txt\x00\x00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := parseStatusPorcelainZ([]byte(valid + tt.tail))
			if len(files) != 1 || files[0].Path != "valid\n界.txt" || files[0].Status != "?" || files[0].Staged {
				t.Fatalf("statuses = %+v, want only valid prefix", files)
			}
		})
	}
}

func TestStatusFilesMultiple(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "tracked.txt", "tracked\n")
	writeFile(t, dir, "will_modify.txt", "original\n")
	writeFile(t, dir, "will_delete.txt", "doomed\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "init")

	// Modify one
	writeFile(t, dir, "will_modify.txt", "changed\n")
	// Delete one
	os.Remove(filepath.Join(dir, "will_delete.txt"))
	// Add untracked
	writeFile(t, dir, "brand_new.txt", "new\n")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	statuses := make(map[string]string)
	for _, f := range files {
		statuses[f.Path] = f.Status
	}

	if statuses["will_modify.txt"] != "M" {
		t.Errorf("expected will_modify.txt to be M, got %q", statuses["will_modify.txt"])
	}
	if statuses["will_delete.txt"] != "D" {
		t.Errorf("expected will_delete.txt to be D, got %q", statuses["will_delete.txt"])
	}
	if statuses["brand_new.txt"] != "?" {
		t.Errorf("expected brand_new.txt to be ?, got %q", statuses["brand_new.txt"])
	}
}

func TestStatusFilesStagedAndUnstagedSameFile(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "file.txt", "v1\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Modify and stage
	writeFile(t, dir, "file.txt", "v2\n")
	gitRun(t, dir, "add", "file.txt")
	// Modify again without staging
	writeFile(t, dir, "file.txt", "v3\n")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	var staged, unstaged bool
	for _, f := range files {
		if f.Path == "file.txt" && f.Status == "M" && f.Staged {
			staged = true
		}
		if f.Path == "file.txt" && f.Status == "M" && !f.Staged {
			unstaged = true
		}
	}
	if !staged {
		t.Error("expected staged modified entry for file.txt")
	}
	if !unstaged {
		t.Error("expected unstaged modified entry for file.txt")
	}
}

func TestStatusFilesInSubdirectory(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "initial.txt", "init\n")
	gitRun(t, dir, "add", "initial.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Create a file in a subdirectory
	writeFile(t, dir, "sub/dir/file.txt", "nested\n")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "sub/dir/file.txt" && f.Status == "?" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected untracked sub/dir/file.txt, got %+v", files)
	}
}

// ---------------------------------------------------------------------------
// BlameLine
// ---------------------------------------------------------------------------

func TestBlameLineBasic(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "file.txt", "line one\nline two\nline three\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "add file")

	info := BlameLine(dir, "file.txt", 1)
	if info == nil {
		t.Fatal("BlameLine returned nil, expected info")
	}
	if info.Author != "Test User" {
		t.Errorf("Author = %q, want %q", info.Author, "Test User")
	}
	if info.Time.IsZero() {
		t.Error("Time is zero, expected a valid timestamp")
	}
}

func TestBlameLineDifferentLines(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "file.txt", "first line\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "first commit")

	// Append a second line in a new commit
	writeFile(t, dir, "file.txt", "first line\nsecond line\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "second commit")

	// Line 1 should reference "first commit"
	info1 := BlameLine(dir, "file.txt", 1)
	if info1 == nil {
		t.Fatal("BlameLine(line 1) returned nil")
	}
	// Line 2 should reference "second commit"
	info2 := BlameLine(dir, "file.txt", 2)
	if info2 == nil {
		t.Fatal("BlameLine(line 2) returned nil")
	}
}

func TestBlameLineUncommittedReturnsNil(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "file.txt", "committed\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Add an uncommitted line
	writeFile(t, dir, "file.txt", "committed\nuncommitted\n")

	info := BlameLine(dir, "file.txt", 2)
	if info != nil {
		t.Errorf("expected nil for uncommitted line, got %+v", info)
	}
}

func TestBlameLineNonexistentFile(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "file.txt", "x\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "init")

	info := BlameLine(dir, "nonexistent.txt", 1)
	if info != nil {
		t.Errorf("expected nil for nonexistent file, got %+v", info)
	}
}

func TestBlameLineInvalidLineNumber(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "file.txt", "only one line\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "init")

	// Line 999 doesn't exist
	info := BlameLine(dir, "file.txt", 999)
	if info != nil {
		t.Errorf("expected nil for out-of-range line, got %+v", info)
	}
}

func TestBlameLineTimeIsRecent(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "file.txt", "hello\n")
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "now")

	info := BlameLine(dir, "file.txt", 1)
	if info == nil {
		t.Fatal("BlameLine returned nil")
	}
	// The commit was just created, so the time should be within the last minute
	if time.Since(info.Time) > time.Minute {
		t.Errorf("blame time %v is too old, expected recent", info.Time)
	}
}

// ---------------------------------------------------------------------------
// FileStatus struct
// ---------------------------------------------------------------------------

func TestFileStatusFields(t *testing.T) {
	fs := FileStatus{
		Status:  "M",
		Path:    "src/main.go",
		OldPath: "",
		Staged:  true,
	}
	if fs.Status != "M" {
		t.Errorf("Status = %q, want %q", fs.Status, "M")
	}
	if fs.Path != "src/main.go" {
		t.Errorf("Path = %q, want %q", fs.Path, "src/main.go")
	}
	if fs.Staged != true {
		t.Error("Staged = false, want true")
	}
}

func TestFileStatusRename(t *testing.T) {
	fs := FileStatus{
		Status:  "R",
		Path:    "new.go",
		OldPath: "old.go",
		Staged:  true,
	}
	if fs.OldPath != "old.go" {
		t.Errorf("OldPath = %q, want %q", fs.OldPath, "old.go")
	}
}

// ---------------------------------------------------------------------------
// BlameInfo struct
// ---------------------------------------------------------------------------

func TestBlameInfoFields(t *testing.T) {
	now := time.Now()
	bi := BlameInfo{
		Author: "Jane Doe",
		Time:   now,
	}
	if bi.Author != "Jane Doe" {
		t.Errorf("Author = %q, want %q", bi.Author, "Jane Doe")
	}
	if !bi.Time.Equal(now) {
		t.Error("Time mismatch")
	}
}

// ---------------------------------------------------------------------------
// IsRepo / RepoRoot (need real git repos)
// ---------------------------------------------------------------------------

func TestIsRepo(t *testing.T) {
	dir := setupTestRepo(t)
	if !IsRepo(dir) {
		t.Error("IsRepo returned false for a git repo")
	}
}

func TestIsRepoFalse(t *testing.T) {
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Error("IsRepo returned true for a non-git directory")
	}
}

func TestRepoRoot(t *testing.T) {
	dir := setupTestRepo(t)
	root := RepoRoot(dir)
	// RepoRoot should return the repo directory (might be a symlink-resolved path)
	if root == "" {
		t.Fatal("RepoRoot returned empty string for a git repo")
	}
	// Resolve symlinks for comparison
	expectedDir, _ := filepath.EvalSymlinks(dir)
	gotDir, _ := filepath.EvalSymlinks(root)
	if gotDir != expectedDir {
		t.Errorf("RepoRoot = %q, want %q", gotDir, expectedDir)
	}
}

func TestRepoRootNotARepo(t *testing.T) {
	dir := t.TempDir()
	root := RepoRoot(dir)
	if root != "" {
		t.Errorf("RepoRoot returned %q for a non-git directory, want empty", root)
	}
}

// ---------------------------------------------------------------------------
// RepoRoot from subdirectory
// ---------------------------------------------------------------------------

func TestRepoRootFromSubdir(t *testing.T) {
	dir := setupTestRepo(t)
	subdir := filepath.Join(dir, "sub", "dir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	root := RepoRoot(subdir)
	expectedDir, _ := filepath.EvalSymlinks(dir)
	gotDir, _ := filepath.EvalSymlinks(root)
	if gotDir != expectedDir {
		t.Errorf("RepoRoot(%q) = %q, want %q", subdir, gotDir, expectedDir)
	}
}

func TestRemoteToHTTPS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"git@github.com:user/repo.git", "https://github.com/user/repo"},
		{"git@github.com:user/repo", "https://github.com/user/repo"},
		{"https://github.com/user/repo.git", "https://github.com/user/repo"},
		{"https://github.com/user/repo", "https://github.com/user/repo"},
	}
	for _, tt := range tests {
		got := remoteToHTTPS(tt.input)
		if got != tt.want {
			t.Errorf("remoteToHTTPS(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Batched stage / unstage / discard
// ---------------------------------------------------------------------------

func stagedPaths(t *testing.T, dir string) map[string]bool {
	t.Helper()
	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, f := range files {
		if f.Staged {
			out[f.Path] = true
		}
	}
	return out
}

func TestStageMultiplePaths(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "a.txt", "a")
	writeFile(t, dir, "b.txt", "b")
	writeFile(t, dir, "c.txt", "c")

	if err := Stage(dir, "a.txt", "b.txt", "c.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	staged := stagedPaths(t, dir)
	for _, p := range []string{"a.txt", "b.txt", "c.txt"} {
		if !staged[p] {
			t.Errorf("%s was not staged", p)
		}
	}
}

func TestUnstageMultiplePaths(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "a.txt", "a")
	writeFile(t, dir, "b.txt", "b")
	gitRun(t, dir, "-C", dir, "add", ".")

	if err := Unstage(dir, "a.txt", "b.txt"); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	if staged := stagedPaths(t, dir); len(staged) != 0 {
		t.Errorf("expected nothing staged, got %v", staged)
	}
}

func TestDiscardMultiplePaths(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "a.txt", "original")
	writeFile(t, dir, "b.txt", "original")
	gitRun(t, dir, "-C", dir, "add", ".")
	gitRun(t, dir, "-C", dir, "commit", "-m", "init")

	writeFile(t, dir, "a.txt", "changed")
	writeFile(t, dir, "b.txt", "changed")

	if err := Discard(dir, "a.txt", "b.txt"); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Errorf("%s = %q, want the committed content back", name, data)
		}
	}
}

func TestDiscardUntrackedMultiplePaths(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "a.txt", "a")
	writeFile(t, dir, "b.txt", "b")

	if err := DiscardUntracked(dir, "a.txt", "b.txt"); err != nil {
		t.Fatalf("DiscardUntracked: %v", err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s still exists", name)
		}
	}
}

// No paths must be a no-op, not a bare "git add --" that stages the whole tree.
func TestBatchOpsWithNoPathsDoNothing(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "a.txt", "a")

	if err := Stage(dir); err != nil {
		t.Fatalf("Stage with no paths: %v", err)
	}
	if staged := stagedPaths(t, dir); len(staged) != 0 {
		t.Errorf("Stage with no paths staged %v", staged)
	}
	if err := Unstage(dir); err != nil {
		t.Errorf("Unstage with no paths: %v", err)
	}
	if err := Discard(dir); err != nil {
		t.Errorf("Discard with no paths: %v", err)
	}
	if err := DiscardUntracked(dir); err != nil {
		t.Errorf("DiscardUntracked with no paths: %v", err)
	}
}

func TestStageReportsError(t *testing.T) {
	dir := setupTestRepo(t)
	if err := Stage(dir, "does-not-exist.txt"); err == nil {
		t.Error("expected an error staging a missing path")
	}
}
