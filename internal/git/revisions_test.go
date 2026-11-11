package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/scm"
)

func TestRevisionReaderKeepsRootRenameDeletionAndLiteralEndpoints(t *testing.T) {
	dir := setupTestRepo(t)
	ctx := context.Background()
	r := repository{root: dir}
	name := "[界]:literal.txt"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("root content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-qm", "root revision")
	ref := RevisionIdentity(dir)
	info, err := r.ReadRevision(ctx, ref)
	if err != nil || info.Message != "root revision" || info.AuthoredAt.IsZero() {
		t.Fatalf("revision: %+v %v", info, err)
	}
	content, err := r.ReadRevisionContent(ctx, ref, name, "")
	if err != nil || content.Before.Exists || !content.After.Exists || content.After.Content != "root content\n" {
		t.Fatalf("root content: %+v %v", content, err)
	}
	gitRun(t, dir, "mv", name, "renamed.txt")
	gitRun(t, dir, "commit", "-qm", "rename")
	ref = RevisionIdentity(dir)
	changes, err := r.RevisionChanges(ctx, ref)
	if err != nil || len(changes) != 1 || changes[0].Kind != scm.Renamed || changes[0].OldPath != name {
		t.Fatalf("rename identity: %+v %v", changes, err)
	}
	patch, err := r.ReadRevisionPatch(ctx, ref, changes[0])
	if err != nil || !strings.Contains(patch, "rename from") {
		t.Fatalf("rename patch: %q %v", patch, err)
	}
	content, err = r.ReadRevisionContent(ctx, ref, "renamed.txt", name)
	if err != nil || content.Before.Content != content.After.Content || !content.Before.Exists || !content.After.Exists {
		t.Fatalf("rename endpoints: %+v %v", content, err)
	}
	gitRun(t, dir, "rm", "renamed.txt")
	gitRun(t, dir, "commit", "-qm", "delete")
	content, err = r.ReadRevisionContent(ctx, RevisionIdentity(dir), "renamed.txt", "")
	if err != nil || !content.Before.Exists || content.After.Exists {
		t.Fatalf("deleted endpoint: %+v %v", content, err)
	}
}
