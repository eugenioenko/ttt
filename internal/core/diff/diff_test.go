package diff

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type cancelAfterChecksContext struct {
	context.Context
	calls    int
	cancelAt int
}

func (c *cancelAfterChecksContext) Err() error {
	c.calls++
	if c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

const sampleDiff = `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 package main

-import "fmt"
+import "log"

 func main() {
@@ -10,3 +10,4 @@
 	x := 1
-	y := 2
+	y := 3
+	z := 4
`

func TestParseHunkCount(t *testing.T) {
	fd := Parse(sampleDiff)
	if len(fd.Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(fd.Hunks))
	}
}

func TestParseContextLines(t *testing.T) {
	fd := Parse(sampleDiff)
	h := fd.Hunks[0]
	if h.Lines[0].Left.Kind != Context {
		t.Errorf("first line should be context, got %d", h.Lines[0].Left.Kind)
	}
	if h.Lines[0].Left.Text != "package main" {
		t.Errorf("expected 'package main', got %q", h.Lines[0].Left.Text)
	}
}

func TestParseDeletedAdded(t *testing.T) {
	fd := Parse(sampleDiff)
	h := fd.Hunks[0]
	// Line at index 2 should be the change: -import "fmt" / +import "log"
	found := false
	for _, dl := range h.Lines {
		if dl.Left.Kind == Deleted && dl.Right.Kind == Added {
			if dl.Left.Text == `import "fmt"` && dl.Right.Text == `import "log"` {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected paired delete/add for import line")
	}
}

func TestParseUnmatchedAdd(t *testing.T) {
	fd := Parse(sampleDiff)
	h := fd.Hunks[1]
	// Second hunk has 1 delete and 2 adds, so last row should have blank left
	found := false
	for _, dl := range h.Lines {
		if dl.Left.Kind == Blank && dl.Right.Kind == Added {
			if dl.Right.Text == "\tz := 4" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected unmatched add line for z := 4")
	}
}

func TestAllLines(t *testing.T) {
	fd := Parse(sampleDiff)
	all := fd.AllLines()
	if len(all) == 0 {
		t.Fatal("AllLines returned empty")
	}
	// Should have lines from both hunks plus a separator
	hunk1Lines := len(fd.Hunks[0].Lines)
	hunk2Lines := len(fd.Hunks[1].Lines)
	expected := hunk1Lines + 1 + hunk2Lines // +1 for separator
	if len(all) != expected {
		t.Errorf("expected %d all lines, got %d", expected, len(all))
	}
}

func TestParseEmpty(t *testing.T) {
	fd := Parse("")
	if len(fd.Hunks) != 0 {
		t.Errorf("expected 0 hunks for empty input, got %d", len(fd.Hunks))
	}
}

func TestGenerate(t *testing.T) {
	old := []string{"a", "b", "c", "d"}
	new := []string{"a", "x", "c", "d"}
	result := Generate(old, new, "test.txt")
	if result == "" {
		t.Fatal("expected non-empty diff")
	}

	fd := Parse(result)
	if len(fd.Hunks) == 0 {
		t.Fatal("expected at least 1 hunk")
	}

	foundDel := false
	foundAdd := false
	for _, h := range fd.Hunks {
		for _, dl := range h.Lines {
			if dl.Left.Kind == Deleted && dl.Left.Text == "b" {
				foundDel = true
			}
			if dl.Right.Kind == Added && dl.Right.Text == "x" {
				foundAdd = true
			}
		}
	}
	if !foundDel {
		t.Error("expected deleted line 'b'")
	}
	if !foundAdd {
		t.Error("expected added line 'x'")
	}
}

func TestGenerateIdentical(t *testing.T) {
	lines := []string{"a", "b", "c"}
	result := Generate(lines, lines, "test.txt")
	if result != "" {
		t.Errorf("expected empty diff for identical files, got: %s", result)
	}
}

func TestGenerateAddition(t *testing.T) {
	old := []string{"a", "c"}
	new := []string{"a", "b", "c"}
	result := Generate(old, new, "test.txt")
	fd := Parse(result)

	foundAdd := false
	for _, h := range fd.Hunks {
		for _, dl := range h.Lines {
			if dl.Right.Kind == Added && dl.Right.Text == "b" {
				foundAdd = true
			}
		}
	}
	if !foundAdd {
		t.Error("expected added line 'b'")
	}
}

func TestGenerateContextPreservesGenerateOutput(t *testing.T) {
	oldLines := []string{"zero", "old", "two", "three"}
	newLines := []string{"zero", "new", "two", "three", "four"}
	got, err := GenerateContext(context.Background(), oldLines, newLines, "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := Generate(oldLines, newLines, "test.txt"); got != want {
		t.Fatalf("GenerateContext output differs from compatibility API:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestComputeLCSContextChecksCancellationDuringMatrixAllocation(t *testing.T) {
	ctx := &cancelAfterChecksContext{Context: context.Background(), cancelAt: 3}
	_, err := computeLCSContext(ctx, make([]string, 32), make([]string, 32))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("allocation cancellation error = %v", err)
	}
}

func TestComputeLCSContextChecksCancellationDuringMatrixComputation(t *testing.T) {
	const rows = 32
	ctx := &cancelAfterChecksContext{Context: context.Background(), cancelAt: 1 + rows + 1 + 2}
	_, err := computeLCSContext(ctx, make([]string, rows), make([]string, 4096))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("computation cancellation error = %v", err)
	}
}

func TestComputeLCSContextChecksCancellationDuringReconstruction(t *testing.T) {
	lines := make([]string, cancellationCheckInterval)
	for i := range lines {
		lines[i] = string(rune(i + 1))
	}
	matrixChecks := len(lines) * len(lines) / cancellationCheckInterval
	ctx := &cancelAfterChecksContext{
		Context:  context.Background(),
		cancelAt: 1 + len(lines) + 1 + matrixChecks + 1,
	}
	_, err := computeLCSContext(ctx, lines, lines)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("reconstruction cancellation error = %v after %d checks", err, ctx.calls)
	}
}

func TestFullDiffLines(t *testing.T) {
	old := []string{"a", "b", "c", "d"}
	new := []string{"a", "x", "c", "d"}
	lines := FullDiffLines(old, new)

	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if lines[0].Left.Kind != Context || lines[0].Left.Text != "a" {
		t.Errorf("line 0: expected context 'a', got %v", lines[0].Left)
	}
	if lines[1].Left.Kind != Deleted || lines[1].Left.Text != "b" {
		t.Errorf("line 1 left: expected deleted 'b', got %v", lines[1].Left)
	}
	if lines[1].Right.Kind != Added || lines[1].Right.Text != "x" {
		t.Errorf("line 1 right: expected added 'x', got %v", lines[1].Right)
	}
	if lines[2].Left.Kind != Context || lines[2].Left.Text != "c" {
		t.Errorf("line 2: expected context 'c', got %v", lines[2].Left)
	}
	if lines[3].Left.Kind != Context || lines[3].Left.Text != "d" {
		t.Errorf("line 3: expected context 'd', got %v", lines[3].Left)
	}
}

func TestFullDiffLinesIdentical(t *testing.T) {
	lines := FullDiffLines([]string{"a", "b"}, []string{"a", "b"})
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for i, dl := range lines {
		if dl.Left.Kind != Context || dl.Right.Kind != Context {
			t.Errorf("line %d: expected both sides context", i)
		}
	}
}

func TestFullDiffLinesAddition(t *testing.T) {
	old := []string{"a", "c"}
	new := []string{"a", "b", "c"}
	lines := FullDiffLines(old, new)

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[1].Left.Kind != Blank {
		t.Errorf("line 1 left: expected blank, got %v", lines[1].Left.Kind)
	}
	if lines[1].Right.Kind != Added || lines[1].Right.Text != "b" {
		t.Errorf("line 1 right: expected added 'b', got %v", lines[1].Right)
	}
}

func TestFullDiffLinesDeletion(t *testing.T) {
	old := []string{"a", "b", "c"}
	new := []string{"a", "c"}
	lines := FullDiffLines(old, new)

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[1].Left.Kind != Deleted || lines[1].Left.Text != "b" {
		t.Errorf("line 1 left: expected deleted 'b', got %v", lines[1].Left)
	}
	if lines[1].Right.Kind != Blank {
		t.Errorf("line 1 right: expected blank, got %v", lines[1].Right.Kind)
	}
}

func TestFullDiffLinesFromHunksMatchesLCSProjection(t *testing.T) {
	separatedOld := make([]string, 30)
	separatedNew := make([]string, 30)
	for i := range separatedOld {
		separatedOld[i] = fmt.Sprintf("line %d", i)
		separatedNew[i] = separatedOld[i]
	}
	separatedNew[1] = "changed near top"
	separatedNew[28] = "changed near bottom"

	tests := []struct {
		name     string
		oldLines []string
		newLines []string
	}{
		{name: "identical", oldLines: []string{"same", "tail"}, newLines: []string{"same", "tail"}},
		{name: "replacement", oldLines: []string{"a", "old", "z"}, newLines: []string{"a", "new", "z"}},
		{name: "addition", oldLines: []string{"a", "z"}, newLines: []string{"a", "added", "z"}},
		{name: "deletion", oldLines: []string{"a", "deleted", "z"}, newLines: []string{"a", "z"}},
		{name: "added file", newLines: []string{"first", "second"}},
		{name: "deleted file", oldLines: []string{"first", "second"}},
		{name: "repeated lines", oldLines: []string{"same", "old", "same", "tail"}, newLines: []string{"same", "new", "same", "tail"}},
		{name: "separated hunks", oldLines: separatedOld, newLines: separatedNew},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fileDiff := Parse(Generate(test.oldLines, test.newLines, "file.txt"))
			got, ok := FullDiffLinesFromHunks(fileDiff, test.oldLines, test.newLines)
			if !ok {
				t.Fatal("validated hunk expansion rejected matching snapshots")
			}
			want := FullDiffLines(test.oldLines, test.newLines)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("linear projection differs from LCS projection:\ngot  %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestFullDiffLinesFromHunksRejectsMismatchedSnapshots(t *testing.T) {
	fileDiff := Parse(Generate([]string{"before", "old", "after"}, []string{"before", "new", "after"}, "file.txt"))
	if _, ok := FullDiffLinesFromHunks(fileDiff, []string{"before", "different", "after"}, []string{"before", "new", "after"}); ok {
		t.Fatal("hunk expansion accepted snapshots that do not match parsed rows")
	}
	if _, ok := FullDiffLinesFromHunks(FileDiff{}, []string{"old"}, []string{"new"}); ok {
		t.Fatal("hunk-less differing snapshots were treated as a valid projection")
	}
}

var benchmarkFullDiffLines []DiffLine

func BenchmarkFullDiffLinesFromHunksUnrelated3500(b *testing.B) {
	const count = 3500
	oldLines := make([]string, count)
	newLines := make([]string, count)
	hunk := Hunk{Lines: make([]DiffLine, count)}
	for i := range count {
		oldLines[i] = fmt.Sprintf("old unrelated line %d", i)
		newLines[i] = fmt.Sprintf("new unrelated line %d", i)
		hunk.Lines[i] = DiffLine{
			Left:  SideLine{Num: i + 1, Text: oldLines[i], Kind: Deleted},
			Right: SideLine{Num: i + 1, Text: newLines[i], Kind: Added},
		}
	}
	fileDiff := FileDiff{Hunks: []Hunk{hunk}}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		lines, ok := FullDiffLinesFromHunks(fileDiff, oldLines, newLines)
		if !ok {
			b.Fatal("matching hunk projection was rejected")
		}
		benchmarkFullDiffLines = lines
	}
}
