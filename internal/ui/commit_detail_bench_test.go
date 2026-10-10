package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

func manyFileDetail(files int) *CommitDetailWidget {
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", true)
	list := make([]CommitDetailFile, files)
	for i := range list {
		var sb strings.Builder
		fmt.Fprintf(&sb, "--- a/f%d.go\n+++ b/f%d.go\n@@ -1,6 +1,6 @@\n", i, i)
		for k := 0; k < 3; k++ {
			fmt.Fprintf(&sb, " context %d\n", k)
		}
		fmt.Fprintf(&sb, "-old line %d\n+new line %d\n", i, i)
		for k := 0; k < 2; k++ {
			fmt.Fprintf(&sb, " tail %d\n", k)
		}
		list[i] = CommitDetailFile{Path: fmt.Sprintf("f%d.go", i), Diff: diff.Parse(sb.String())}
	}
	detail.SetDetail("Subject", list, "")
	return detail
}

func BenchmarkCommitDetailManyFilesOpen(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		detail := manyFileDetail(500)
		renderDetail(detail, 120, 40)
	}
}

func TestCommitDetailBuildsOnlyVisibleDiffs(t *testing.T) {
	detail := manyFileDetail(200)
	renderDetail(detail, 120, 30)
	first, last := detail.Files[0].view, detail.Files[len(detail.Files)-1].view
	if first.right.Highlighter == nil || len(first.right.Buf.Lines) < 2 {
		t.Fatal("the visible first file was not built")
	}
	if last.right.Highlighter != nil || last.left.Highlighter != nil || last.unified.Highlighter != nil {
		t.Fatal("an off-screen file loaded a highlighter")
	}
	if len(last.right.Buf.Lines) > 1 || len(last.unified.Buf.Lines) > 1 {
		t.Fatal("an off-screen file built its panes")
	}
}

func TestDeferredDiffRowCountMatchesBuiltPanes(t *testing.T) {
	diffs := []string{
		"--- a/x\n+++ b/x\n@@ -1,1 +1,1 @@\n-old one\n+new one\n@@ -10,1 +10,1 @@\n-old ten\n+new ten\n",
		"--- a/x\n+++ b/x\n@@ -1,2 +1,4 @@\n keep\n+a\n+b\n-gone\n+c\n",
		"--- a/x\n+++ b/x\n@@ -3,3 +3,1 @@\n ctx\n-x\n-y\n",
	}
	for _, text := range diffs {
		for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
			lazy := NewDiffEditorWidget("x", diff.FileDiff{}, nil, nil, false)
			lazy.setEmbedded()
			lazy.mode = mode
			lazy.setSource(diff.Parse(text), nil, nil, false, false, nil)
			eager := NewDiffEditorWidget("x", diff.Parse(text), nil, nil, false)
			eager.mode = mode
			eager.ensurePanes()
			want := eager.unified.layout().total()
			if mode == DiffModeSplit {
				eager.right.Viewport.Width, eager.left.Viewport.Width = 40, 40
				eager.alignSplit()
				want = max(eager.left.layout().total(), eager.right.layout().total())
			}
			if got := lazy.embedRows(81); got != want {
				t.Fatalf("mode %v: deferred count %d, built panes %d\n%s", mode, got, want, text)
			}
		}
	}
}
