package jj

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/jjtest"
	"github.com/eugenioenko/ttt/internal/scm"
)

func discover(t *testing.T, dir string) repository {
	t.Helper()
	repo, err := (Provider{}).Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo.(repository)
}

func TestParseChangesMapsStatusesAndConflicts(t *testing.T) {
	out := []byte("F\x00modified\x00a.txt\x00a.txt\x00" +
		"F\x00renamed\x00old name.txt\x00new\tname.txt\x00" +
		"F\x00removed\x00gone.txt\x00gone.txt\x00" +
		"F\x00copied\x00src.txt\x00dst.txt\x00" +
		"F\x00added\x00c.txt\x00c.txt\x00" +
		"F\x00mystery\x00m.txt\x00m.txt\x00" +
		"C\x00c.txt\x00C\x00only-conflict.txt\x00")
	got, err := parseChanges(out)
	if err != nil {
		t.Fatal(err)
	}
	want := []scm.Change{
		{Path: "a.txt", Kind: scm.Modified, Group: scm.WorkingTree},
		{Path: "new\tname.txt", OldPath: "old name.txt", Kind: scm.Renamed, Group: scm.WorkingTree},
		{Path: "gone.txt", Kind: scm.Deleted, Group: scm.WorkingTree},
		{Path: "dst.txt", OldPath: "src.txt", Kind: scm.Copied, Group: scm.WorkingTree},
		{Path: "c.txt", Kind: scm.Conflicted, Group: scm.WorkingTree, Detail: "conflict"},
		{Path: "m.txt", Kind: scm.Unknown, Group: scm.WorkingTree, Detail: "mystery"},
		{Path: "only-conflict.txt", Kind: scm.Conflicted, Group: scm.WorkingTree, Detail: "conflict"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes:\n got %#v\nwant %#v", got, want)
	}
	if _, err := parseChanges([]byte("F\x00modified\x00a.txt\x00")); err == nil {
		t.Fatal("truncated record parsed")
	}
	if got, err := parseChanges(nil); err != nil || len(got) != 0 {
		t.Fatalf("empty output: %v %v", got, err)
	}
}

func TestFilesetQuotesLiteralPaths(t *testing.T) {
	got := fileset(`a "b".txt`, "", `c\d`, "*.go")
	want := `root-file:"a \"b\".txt" | root-file:"c\\d" | root-file:"*.go"`
	if got != want {
		t.Fatalf("fileset = %s, want %s", got, want)
	}
}

func TestValidCommitIDRejectsRevsets(t *testing.T) {
	for _, ref := range []string{"", "@", "main", "abc | all()", "ABC"} {
		if validCommitID(ref) == nil {
			t.Errorf("accepted %q", ref)
		}
	}
	if err := validCommitID("0123abcdef"); err != nil {
		t.Fatal(err)
	}
}

func stubLookPath(t *testing.T, err error) {
	t.Helper()
	original := lookPath
	lookPath = func(string) (string, error) { return "/usr/bin/jj", err }
	t.Cleanup(func() { lookPath = original })
}

func TestDiscoverFindsNearestWorkspace(t *testing.T) {
	stubLookPath(t, nil)
	root := realTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, ".jj"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := discover(t, nested).Root(); got != root {
		t.Fatalf("root = %q, want %q", got, root)
	}

	gitRepo := filepath.Join(root, "vendor")
	if err := os.MkdirAll(filepath.Join(gitRepo, ".git", "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (Provider{}).Discover(context.Background(), filepath.Join(gitRepo)); !errors.Is(err, scm.ErrNotRepository) {
		t.Fatalf("nested git repository claimed by jj: %v", err)
	}

	colocated := realTempDir(t)
	for _, name := range []string{".jj", ".git"} {
		if err := os.MkdirAll(filepath.Join(colocated, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := discover(t, colocated).Root(); got != colocated {
		t.Fatalf("colocated root = %q", got)
	}

	if _, err := (Provider{}).Discover(context.Background(), t.TempDir()); !errors.Is(err, scm.ErrNotRepository) {
		t.Fatalf("plain directory: %v", err)
	}

	alias := filepath.Join(realTempDir(t), "alias")
	if err := os.Symlink(nested, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got := discover(t, alias).Root(); got != root {
		t.Fatalf("symlinked directory root = %q, want %q", got, root)
	}
}

func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDiscoverReportsUnreadableAncestors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory modes do not block traversal on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	stubLookPath(t, nil)
	locked := filepath.Join(realTempDir(t), "locked")
	inner := filepath.Join(locked, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	if _, err := (Provider{}).Discover(context.Background(), inner); err == nil || errors.Is(err, scm.ErrNotRepository) {
		t.Fatalf("unreadable ancestor err = %v, want a filesystem error", err)
	}
}

func TestDiscoverWithoutBinaryDefersToOtherProviders(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".jj"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubLookPath(t, errors.New("missing"))
	if _, err := (Provider{}).Discover(context.Background(), root); !errors.Is(err, scm.ErrNotRepository) {
		t.Fatalf("err = %v", err)
	}
}

func TestRepositoryReportsWorkingCopyChanges(t *testing.T) {
	dir := jjtest.Init(t)
	jjtest.Write(t, dir, "keep.txt", "keep\n")
	jjtest.Write(t, dir, "edit.txt", "one\n")
	jjtest.Write(t, dir, "gone.txt", "bye\n")
	jjtest.Write(t, dir, "old.txt", "rename me\n")
	jjtest.Run(t, dir, "commit", "-m", "base")

	jjtest.Write(t, dir, "edit.txt", "one\ntwo\n")
	jjtest.Write(t, dir, "dir/new file.txt", "fresh\n")
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "old.txt"), filepath.Join(dir, "renamed.txt")); err != nil {
		t.Fatal(err)
	}

	repo := discover(t, dir)
	ctx := context.Background()
	changes, err := repo.Changes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]scm.Change{}
	for _, change := range changes {
		got[change.Path] = change
	}
	want := map[string]scm.Change{
		"edit.txt":         {Path: "edit.txt", Kind: scm.Modified, Group: scm.WorkingTree},
		"dir/new file.txt": {Path: "dir/new file.txt", Kind: scm.Added, Group: scm.WorkingTree},
		"gone.txt":         {Path: "gone.txt", Kind: scm.Deleted, Group: scm.WorkingTree},
		"renamed.txt":      {Path: "renamed.txt", OldPath: "old.txt", Kind: scm.Renamed, Group: scm.WorkingTree},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes:\n got %#v\nwant %#v", got, want)
	}

	identity, err := repo.Identity(ctx)
	if err != nil || identity.Root != dir || len(identity.Label) != 8 {
		t.Fatalf("identity = %#v, %v", identity, err)
	}
	jjtest.Run(t, dir, "bookmark", "create", "-r", "@", "feature")
	if identity, err = repo.Identity(ctx); err != nil || identity.Label != "feature" {
		t.Fatalf("bookmark identity = %#v, %v", identity, err)
	}

	before, err := repo.Revision(ctx)
	if err != nil || before == "" {
		t.Fatalf("revision = %q, %v", before, err)
	}
	jjtest.Write(t, dir, "edit.txt", "edited again\n")
	if _, err := repo.Changes(ctx); err != nil {
		t.Fatal(err)
	}
	if after, err := repo.Revision(ctx); err != nil || after != before {
		t.Fatalf("working-copy edit changed the baseline revision: %q -> %q, %v", before, after, err)
	}
	jjtest.Run(t, dir, "commit", "-m", "second")
	if after, err := repo.Revision(ctx); err != nil || after == "" || after == before {
		t.Fatalf("commit did not change the baseline revision: %q -> %q, %v", before, after, err)
	}
}

func TestReadBaselineAndChange(t *testing.T) {
	dir := jjtest.Init(t)
	jjtest.Write(t, dir, ".gitignore", "ignored.txt\n")
	jjtest.Write(t, dir, "edit.txt", "one\n")
	jjtest.Write(t, dir, "old.txt", "moved\n")
	jjtest.Run(t, dir, "commit", "-m", "base")
	jjtest.Write(t, dir, "edit.txt", "one\ntwo\n")
	jjtest.Write(t, dir, "new.txt", "new\n")
	jjtest.Write(t, dir, "ignored.txt", "secret\n")
	if err := os.Rename(filepath.Join(dir, "old.txt"), filepath.Join(dir, "moved.txt")); err != nil {
		t.Fatal(err)
	}
	repo := discover(t, dir)
	ctx := context.Background()

	base, err := repo.ReadBaseline(ctx, "edit.txt")
	if err != nil || !base.Exists || base.Content != "one\n" || base.Ignored || base.Conflicted {
		t.Fatalf("edit baseline = %#v, %v", base, err)
	}
	base, err = repo.ReadBaseline(ctx, "new.txt")
	if err != nil || base.Exists || base.Ignored {
		t.Fatalf("new baseline = %#v, %v", base, err)
	}
	base, err = repo.ReadBaseline(ctx, "ignored.txt")
	if err != nil || base.Exists || !base.Ignored {
		t.Fatalf("ignored baseline = %#v, %v", base, err)
	}

	diff, err := repo.ReadChange(ctx, scm.Change{Path: "edit.txt", Kind: scm.Modified, Group: scm.WorkingTree})
	if err != nil {
		t.Fatal(err)
	}
	if diff.Before != (scm.FileContent{Content: "one\n", Exists: true}) || diff.After != (scm.FileContent{Content: "one\ntwo\n", Exists: true}) {
		t.Fatalf("modified endpoints = %#v", diff)
	}
	if !strings.Contains(diff.Patch, "+two") || !strings.Contains(diff.Patch, "diff --git a/edit.txt b/edit.txt") {
		t.Fatalf("patch = %q", diff.Patch)
	}

	diff, err = repo.ReadChange(ctx, scm.Change{Path: "moved.txt", OldPath: "old.txt", Kind: scm.Renamed, Group: scm.WorkingTree})
	if err != nil || diff.Before.Content != "moved\n" || diff.After.Content != "moved\n" || !strings.Contains(diff.Patch, "rename from old.txt") {
		t.Fatalf("rename = %#v, %v", diff, err)
	}

	if _, err := repo.ReadChange(ctx, scm.Change{Path: "edit.txt", Group: scm.Staged}); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("staged change read: %v", err)
	}
}

func TestConflictedWorkingCopy(t *testing.T) {
	dir := jjtest.Init(t)
	jjtest.Write(t, dir, "f.txt", "base\n")
	jjtest.Run(t, dir, "describe", "-m", "base")
	jjtest.Run(t, dir, "new", "-m", "left")
	jjtest.Write(t, dir, "f.txt", "left\n")
	jjtest.Run(t, dir, "new", "-m", "right", `description(substring:"base")`)
	jjtest.Write(t, dir, "f.txt", "right\n")
	jjtest.Run(t, dir, "new", `description(substring:"left")`, `description(substring:"right")`)

	repo := discover(t, dir)
	ctx := context.Background()
	changes, err := repo.Changes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []scm.Change{{Path: "f.txt", Kind: scm.Conflicted, Group: scm.WorkingTree, Detail: "conflict"}}
	if !reflect.DeepEqual(changes, want) {
		t.Fatalf("changes = %#v", changes)
	}
	base, err := repo.ReadBaseline(ctx, "f.txt")
	if err != nil || !base.Conflicted || !base.Exists {
		t.Fatalf("baseline = %#v, %v", base, err)
	}
	diff, err := repo.ReadChange(ctx, changes[0])
	if err != nil || !diff.Conflicted || !strings.Contains(diff.After.Content, "<<<<<<<") {
		t.Fatalf("conflict read = %#v, %v", diff, err)
	}
}

func TestHistoryAndRevisions(t *testing.T) {
	dir := jjtest.Init(t)
	repo := discover(t, dir)
	ctx := context.Background()
	page, err := repo.ReadHistory(ctx, scm.HistoryRequest{Limit: 10})
	if err != nil || len(page.Entries) != 0 || page.HasMore {
		t.Fatalf("empty history = %#v, %v", page, err)
	}

	for i, name := range []string{"first", "second", "third"} {
		jjtest.Write(t, dir, "f.txt", strings.Repeat("x\n", i+1))
		jjtest.Run(t, dir, "commit", "-m", name+"\n\nbody "+name)
	}
	page, err = repo.ReadHistory(ctx, scm.HistoryRequest{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 2 || !page.HasMore || page.Entries[0].Message != "third" || page.Entries[1].Message != "second" || len(page.Entries[0].Label) != 8 {
		t.Fatalf("first page = %#v", page)
	}

	jjtest.Write(t, dir, "f.txt", "later\n")
	jjtest.Run(t, dir, "commit", "-m", "later")
	next, err := repo.ReadHistory(ctx, scm.HistoryRequest{Anchor: page.Anchor, Offset: 2, Limit: 2})
	if err != nil || next.Anchor != page.Anchor || len(next.Entries) != 1 || next.HasMore || next.Entries[0].Message != "first" {
		t.Fatalf("anchored page = %#v, %v", next, err)
	}

	ref := page.Entries[0].Ref
	revision, err := repo.ReadRevision(ctx, ref)
	if err != nil || revision.Message != "third\n\nbody third" || revision.AuthoredAtUnavailable || revision.AuthoredAt.IsZero() {
		t.Fatalf("revision = %#v, %v", revision, err)
	}
	changes, err := repo.RevisionChanges(ctx, ref)
	if err != nil || !reflect.DeepEqual(changes, []scm.Change{{Path: "f.txt", Kind: scm.Modified, Group: scm.WorkingTree}}) {
		t.Fatalf("revision changes = %#v, %v", changes, err)
	}
	patch, err := repo.ReadRevisionPatch(ctx, ref, changes[0])
	if err != nil || !strings.Contains(patch, "+x") {
		t.Fatalf("patch = %q, %v", patch, err)
	}
	content, err := repo.ReadRevisionContent(ctx, ref, "f.txt", "")
	if err != nil || content.Before.Content != "x\nx\n" || content.After.Content != "x\nx\nx\n" {
		t.Fatalf("content = %#v, %v", content, err)
	}
	first := next.Entries[0].Ref
	content, err = repo.ReadRevisionContent(ctx, first, "f.txt", "")
	if err != nil || content.Before.Exists || content.After.Content != "x\n" {
		t.Fatalf("root-parent content = %#v, %v", content, err)
	}
	if _, err := repo.ReadRevisionContent(ctx, first, "missing.txt", ""); err == nil {
		t.Fatal("missing path read succeeded")
	}
	if _, err := repo.ReadRevision(ctx, "all()"); err == nil {
		t.Fatal("revset ref accepted")
	}
}

func TestReadsWorkWithoutConfiguredIdentity(t *testing.T) {
	dir := jjtest.Init(t)
	jjtest.Write(t, dir, "f.txt", "x\n")
	repo := discover(t, dir)
	if _, err := repo.Changes(context.Background()); err != nil {
		t.Fatalf("changes without user.name/user.email: %v", err)
	}
}

func TestParsedOutputIgnoresUserPresentationConfig(t *testing.T) {
	dir := jjtest.Init(t)
	name := "a long file name with many spaces so that log word wrapping would split it.txt"
	jjtest.Write(t, dir, name, "content\n")
	jjtest.Run(t, dir, "commit", "-m", "base")
	jjtest.Write(t, dir, name, "changed\n")
	config := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(config, []byte("[ui]\nlog-word-wrap = true\n[templates]\nfile_show = '\"META \" ++ path ++ \"\\n\"'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JJ_CONFIG", config)
	t.Setenv("COLUMNS", "30")

	repo := discover(t, dir)
	ctx := context.Background()
	changes, err := repo.Changes(ctx)
	if err != nil || len(changes) != 1 || changes[0].Path != name {
		t.Fatalf("changes = %#v, %v", changes, err)
	}
	base, err := repo.ReadBaseline(ctx, name)
	if err != nil || base.Content != "content\n" {
		t.Fatalf("baseline = %#v, %v", base, err)
	}
}

func TestMergeReadsAlignWithFirstParent(t *testing.T) {
	dir := jjtest.Init(t)
	jjtest.Write(t, dir, "f.txt", "one\ntwo\nthree\n")
	jjtest.Write(t, dir, "g.txt", "g\n")
	jjtest.Run(t, dir, "describe", "-m", "base")
	jjtest.Run(t, dir, "new", "-m", "left")
	jjtest.Write(t, dir, "f.txt", "one\ntwo\nthree\nleft\n")
	jjtest.Run(t, dir, "new", "-m", "right", `description(substring:"base")`)
	jjtest.Write(t, dir, "g.txt", "g\nright\n")
	jjtest.Run(t, dir, "new", `description(substring:"left")`, `description(substring:"right")`)
	jjtest.Write(t, dir, "f.txt", "ONE\ntwo\nthree\nleft\n")

	repo := discover(t, dir)
	ctx := context.Background()
	diff, err := repo.ReadChange(ctx, scm.Change{Path: "f.txt", Kind: scm.Modified, Group: scm.WorkingTree})
	if err != nil {
		t.Fatal(err)
	}
	if diff.Before.Content != "one\ntwo\nthree\nleft\n" || !strings.Contains(diff.Patch, "-one\n+ONE") || strings.Contains(diff.Patch, "+left") {
		t.Fatalf("merge working-copy read: before=%q patch=%q", diff.Before.Content, diff.Patch)
	}
	diff, err = repo.ReadChange(ctx, scm.Change{Path: "g.txt", Kind: scm.Modified, Group: scm.WorkingTree})
	if err != nil || diff.Before.Content != "g\n" || !strings.Contains(diff.Patch, "+right") {
		t.Fatalf("second-parent file against first parent: before=%q patch=%q err=%v", diff.Before.Content, diff.Patch, err)
	}
}

func TestBaselineMarksConflictedParent(t *testing.T) {
	dir := jjtest.Init(t)
	jjtest.Write(t, dir, "f.txt", "base\n")
	jjtest.Run(t, dir, "describe", "-m", "base")
	jjtest.Run(t, dir, "new", "-m", "left")
	jjtest.Write(t, dir, "f.txt", "left\n")
	jjtest.Run(t, dir, "new", "-m", "right", `description(substring:"base")`)
	jjtest.Write(t, dir, "f.txt", "right\n")
	jjtest.Run(t, dir, "new", "-m", "merge", `description(substring:"left")`, `description(substring:"right")`)
	jjtest.Run(t, dir, "new")
	jjtest.Write(t, dir, "f.txt", "resolved\n")

	repo := discover(t, dir)
	changes, err := repo.Changes(context.Background())
	if err != nil || len(changes) != 1 || changes[0].Kind != scm.Modified {
		t.Fatalf("working copy should resolve the parent's conflict: %#v, %v", changes, err)
	}
	base, err := repo.ReadBaseline(context.Background(), "f.txt")
	if err != nil || !base.Conflicted {
		t.Fatalf("baseline from a conflicted parent = %#v, %v", base, err)
	}
}

func TestReadChangeRejectsNonRegularFiles(t *testing.T) {
	dir := t.TempDir()
	if err := mkfifo(filepath.Join(dir, "pipe")); err != nil {
		t.Skipf("fifo unavailable: %v", err)
	}
	if _, err := (repository{root: dir}).workingFile("pipe"); err == nil {
		t.Fatal("read a FIFO as a working-copy file")
	}
}
