package diff

import (
	"fmt"
	"testing"
)

func scatteredPair(n int) (oldLines, newLines []string) {
	for i := range n {
		oldLines = append(oldLines, fmt.Sprintf("line %d", i))
	}
	for i, l := range oldLines {
		switch {
		case i%97 == 0:
		case i%40 == 0:
			newLines = append(newLines, l+" changed")
		case i%151 == 0:
			newLines = append(newLines, l, "added a", "added b")
		default:
			newLines = append(newLines, l)
		}
	}
	return oldLines, newLines
}

func BenchmarkFullDiffLines(b *testing.B) {
	oldLines, newLines := scatteredPair(2000)
	b.ReportAllocs()
	for b.Loop() {
		FullDiffLines(oldLines, newLines)
	}
}

func TestFullDiffLinesRebuildsBothSides(t *testing.T) {
	words := []string{"a", "b", "c", "}", ""}
	seed := uint32(7)
	next := func(n int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % n
	}
	for round := range 300 {
		var oldLines, newLines []string
		for range next(30) {
			oldLines = append(oldLines, words[next(len(words))])
		}
		for range next(30) {
			newLines = append(newLines, words[next(len(words))])
		}
		var gotOld, gotNew []string
		lastOld, lastNew := 0, 0
		for _, dl := range FullDiffLines(oldLines, newLines) {
			if dl.Left.Kind != Blank {
				if dl.Left.Num != lastOld+1 {
					t.Fatalf("round %d: old numbers jump from %d to %d", round, lastOld, dl.Left.Num)
				}
				lastOld = dl.Left.Num
				gotOld = append(gotOld, dl.Left.Text)
			}
			if dl.Right.Kind != Blank {
				if dl.Right.Num != lastNew+1 {
					t.Fatalf("round %d: new numbers jump from %d to %d", round, lastNew, dl.Right.Num)
				}
				lastNew = dl.Right.Num
				gotNew = append(gotNew, dl.Right.Text)
			}
			if (dl.Left.Kind == Context) != (dl.Right.Kind == Context) || dl.Left.Kind == Context && dl.Left.Text != dl.Right.Text {
				t.Fatalf("round %d: context row pairs %q with %q", round, dl.Left.Text, dl.Right.Text)
			}
		}
		if fmt.Sprint(gotOld) != fmt.Sprint(oldLines) || fmt.Sprint(gotNew) != fmt.Sprint(newLines) {
			t.Fatalf("round %d: rebuilt %q / %q from %q / %q", round, gotOld, gotNew, oldLines, newLines)
		}
		if lcs := computeLCS(oldLines, newLines); countContext(FullDiffLines(oldLines, newLines)) != len(lcs) {
			t.Fatalf("round %d: context rows are not a longest common subsequence", round)
		}
	}
}

func countContext(lines []DiffLine) int {
	n := 0
	for _, dl := range lines {
		if dl.Left.Kind == Context {
			n++
		}
	}
	return n
}
