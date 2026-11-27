package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/ui"
)

type revisionRepository struct {
	testSCMRepository
	contentReads int
	refs         []string
	err          error
	change       scm.Change
	onContent    func()
}

func (r *revisionRepository) ReadRevision(_ context.Context, ref string) (scm.Revision, error) {
	r.refs = append(r.refs, ref)
	return scm.Revision{Message: "provider revision"}, r.err
}
func (r *revisionRepository) RevisionChanges(_ context.Context, ref string) ([]scm.Change, error) {
	r.refs = append(r.refs, ref)
	return []scm.Change{r.change}, r.err
}
func (r *revisionRepository) ReadRevisionPatch(_ context.Context, ref string, _ scm.Change) (string, error) {
	r.refs = append(r.refs, ref)
	return "diff --git a/file b/file\n--- a/file\n+++ b/file\n@@ -1 +1 @@\n-before\n+after\n", r.err
}
func (r *revisionRepository) ReadRevisionContent(_ context.Context, ref, path, oldPath string) (scm.RevisionContent, error) {
	r.refs = append(r.refs, ref)
	r.contentReads++
	if r.onContent != nil {
		r.onContent()
	}
	return scm.RevisionContent{Before: scm.FileContent{Content: "before\n", Exists: true}, After: scm.FileContent{Content: "after\n", Exists: true}}, r.err
}

func TestProviderRevisionReadsKeepContentLazy(t *testing.T) {
	r := &revisionRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}, change: scm.Change{Path: "界.txt", OldPath: "old.txt", Kind: scm.Renamed, Group: "provider", Detail: "opaque"}}
	source := repositorySource{root: r.root, repository: r}
	files := readCommitFiles(context.Background(), 1, source, "revision:opaque^", "label", "node")
	if files.Err != nil || !reflect.DeepEqual(files.Files, []scm.Change{r.change}) {
		t.Fatalf("typed files: %+v", files)
	}
	result := readCommitDetail(context.Background(), 1, source, "revision:opaque^", "label")
	if result.Err != "" || result.Message != "provider revision" || len(result.Files) != 1 || result.Files[0].Status != "R" || result.Files[0].OldPath != "old.txt" || result.Files[0].Group != "provider" || r.contentReads != 0 {
		t.Fatalf("detail eagerly loaded content or lost metadata: %+v reads=%d", result, r.contentReads)
	}
	full := readCommitDetailContext(context.Background(), 1, "tab", source, "revision:opaque^", 0, result.Files[0])
	if full.Err != "" || !reflect.DeepEqual(full.OldLines, []string{"before"}) || !reflect.DeepEqual(full.NewLines, []string{"after"}) || r.contentReads != 1 {
		t.Fatalf("full context: %+v", full)
	}
	opened := readCommitDiff(context.Background(), source, "revision:opaque^", "label", r.change, true)
	if opened.Warn != "" || opened.Canceled || !opened.Extended || len(opened.NewLines) != 1 || opened.NewLines[0] != "after" {
		t.Fatalf("provider revision diff: %+v", opened)
	}
	for _, ref := range r.refs {
		if ref != "revision:opaque^" {
			t.Fatalf("provider ref changed: %q", ref)
		}
	}
}

func TestProviderRevisionUnsupportedErrorsAndCancellation(t *testing.T) {
	root := testAppRepository(t)
	source := repositorySource{root: root, repository: &testSCMRepository{root: root}}
	if got := readCommitFiles(context.Background(), 1, source, "opaque", "label", "node"); !errors.Is(got.Err, errors.ErrUnsupported) {
		t.Fatalf("unsupported reader fell back to Git: %+v", got)
	}
	r := &revisionRepository{testSCMRepository: testSCMRepository{root: root}, change: scm.Change{Path: "initial.txt", Kind: scm.Modified}}
	source.repository = r
	r.err = errors.New("provider failure")
	if got := readCommitDetail(context.Background(), 1, source, "opaque", "label"); got.Err == "" {
		t.Fatalf("metadata error hidden: %+v", got)
	}
	if got := readCommitDetailContext(context.Background(), 1, "tab", source, "opaque", 0, ui.CommitDetailFile{Path: "initial.txt"}); got.Err == "" {
		t.Fatalf("content error hidden: %+v", got)
	}
	r.err = nil
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.onContent = cancel
	if got := readCommitDiff(ctx, source, "opaque", "label", r.change, false); !got.Canceled {
		t.Fatalf("canceled content delivered: %+v", got)
	}
}

func TestHistoryFileParentKeepsOpaqueRevisionIdentity(t *testing.T) {
	ref := "revision:opaque^ with spaces%"
	parent := historyCommitNodeID(ref)
	gotParent, gotRef, ok := commitFileParent("cfile:" + parent + ":dir:literal.txt")
	if !ok || gotParent != parent || gotRef != ref {
		t.Fatalf("opaque revision parsed as Git hash: %q %q %v", gotParent, gotRef, ok)
	}
}
