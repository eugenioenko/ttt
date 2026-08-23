package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

func TestCommitDetailFailedFullFileContextCanRetryWithoutModeRoundTrip(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abcdef0", false)
	detail.SetDetail("subject", []CommitDetailFile{{
		Path: "file.txt",
		Diff: diff.Parse("--- a/file.txt\n+++ b/file.txt\n@@ -2 +2 @@\n-old\n+new\n"),
	}}, "")
	fetches := 0
	detail.OnFetchContext = func(int, CommitDetailFile) { fetches++ }

	detail.SetContextMode(DiffContextFullFile)
	if fetches != 1 {
		t.Fatalf("first full-file request count = %d", fetches)
	}
	detail.ApplyFileContext(0, CommitDetailContextKey(detail.Files[0]), nil, nil, "Could not load full file for file.txt")
	if detail.Files[0].FullFileState != CommitDetailFullFileFailed {
		t.Fatalf("failed request state = %v", detail.Files[0].FullFileState)
	}

	detail.SetContextMode(DiffContextFullFile)
	if fetches != 2 {
		t.Fatalf("failed full-file request is not retryable: fetches=%d mode=%v", fetches, detail.ContextMode())
	}
}

func TestCommitDetailLegitimateEmptyFullFileIsLoaded(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abcdef0", false)
	detail.SetDetail("subject", []CommitDetailFile{{Status: "A", Path: "empty.txt"}}, "")
	detail.SetContextMode(DiffContextFullFile)
	detail.ApplyFileContext(0, CommitDetailContextKey(detail.Files[0]), nil, []string{})
	if detail.Files[0].FullFileState != CommitDetailFullFileLoaded || detail.Files[0].FullFileErr != "" {
		t.Fatalf("empty file state=%v err=%q", detail.Files[0].FullFileState, detail.Files[0].FullFileErr)
	}
}

func TestCommitDetailFailedFullFileIsVisibleAlongsideCompactDiff(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abcdef0", false)
	detail.OnFetchContext = func(int, CommitDetailFile) {}
	detail.SetDetail("subject", []CommitDetailFile{{
		Path: "file.txt",
		Diff: diff.Parse("--- a/file.txt\n+++ b/file.txt\n@@ -2 +2 @@\n-old\n+new\n"),
	}}, "")
	detail.SetContextMode(DiffContextFullFile)
	detail.ApplyFileContext(0, CommitDetailContextKey(detail.Files[0]), nil, nil, "Could not load full file for file.txt")
	detail.SetRect(Rect{W: 80, H: 12})
	cells := makeGrid(80, 12)
	detail.Render(NewRenderSurface(cells, Rect{W: 80, H: 12}))
	text := detailTestText(cells)
	for _, want := range []string{"Could not load full file for file.txt", "old", "new"} {
		if !strings.Contains(text, want) {
			t.Fatalf("failed Full File view missing %q:\n%s", want, text)
		}
	}
}

func TestCommitDetailContextReadDoesNotRetargetWhileLoading(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abcdef0", false)
	detail.SetDetail("subject", []CommitDetailFile{{
		Path: "file.txt",
		Diff: diff.Parse("--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old one\n+new one\n@@ -10 +10 @@\n-old ten\n+new ten\n@@ -20 +20 @@\n-old twenty\n+new twenty\n"),
	}}, "")
	requests := 0
	detail.OnFetchContext = func(int, CommitDetailFile) { requests++ }
	detail.requestFileContext(0, 0)
	detail.requestFileContext(0, 1)
	if requests != 1 || detail.Files[0].pendingGap != 0 {
		t.Fatalf("in-flight gap request count=%d pending=%d", requests, detail.Files[0].pendingGap)
	}
}

func TestCommitDetailCollapsedRowsAreNotSelectableOrCopied(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abcdef0", false)
	detail.SetDetail("subject", []CommitDetailFile{{
		Path: "file.txt",
		Diff: diff.Parse("--- a/file.txt\n+++ b/file.txt\n@@ -1,1 +1,1 @@\n-old one\n+new one\n@@ -10,1 +10,1 @@\n-old ten\n+new ten\n"),
	}}, "")
	gapRow := -1
	for rowIndex, row := range detail.rows {
		if row.kind == commitDetailDiffRow && detail.Files[0].lines[row.lineIndex].Left.Kind == diff.Collapsed {
			gapRow = rowIndex
			break
		}
	}
	if gapRow < 0 {
		t.Fatal("missing collapsed presentation row")
	}
	if text, selectable := detail.rowText(gapRow, false); selectable || !strings.Contains(text, "8 lines") {
		t.Fatalf("collapsed row text = %q, selectable=%v", text, selectable)
	}
	detail.hasSelection = true
	detail.selection.Anchor = diffSelPos{Line: gapRow - 1, Col: 0}
	detail.selection.Current = diffSelPos{Line: gapRow + 1, Col: 100}
	if copied := detail.selectionText(); strings.Contains(copied, "lines ⋯") {
		t.Fatalf("copy included collapsed presentation text: %q", copied)
	}
}

func TestCommitDetailBinaryAndEmptyContextAreSuccessfulLoadedStates(t *testing.T) {
	for _, test := range []struct {
		name string
		kind CommitDetailContentKind
	}{
		{name: "binary", kind: CommitDetailContentBinary},
		{name: "empty", kind: CommitDetailContentEmpty},
	} {
		t.Run(test.name, func(t *testing.T) {
			detail := NewCommitDetailWidget("/repo", "full", "abcdef0", false)
			detail.SetDetail("subject", []CommitDetailFile{{Status: "A", Path: test.name}}, "")
			if !detail.ApplyFileContextContent(0, CommitDetailContextKey(detail.Files[0]), nil, nil, test.kind, "") {
				t.Fatal("context result rejected")
			}
			if detail.Files[0].FullFileState != CommitDetailFullFileLoaded || detail.Files[0].ContentKind != test.kind {
				t.Fatalf("state=%v kind=%v", detail.Files[0].FullFileState, detail.Files[0].ContentKind)
			}
		})
	}
}
