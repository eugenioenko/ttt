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
