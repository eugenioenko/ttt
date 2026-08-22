package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func detailTestText(cells [][]term.Cell) string {
	var out strings.Builder
	for rowIndex, row := range cells {
		if rowIndex > 0 {
			out.WriteByte('\n')
		}
		for _, cell := range row {
			if cell.Ch == 0 {
				out.WriteByte(' ')
			} else {
				out.WriteRune(cell.Ch)
			}
		}
	}
	return out.String()
}

func TestCommitDetailInheritsEverySharedPresentationPreference(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
	detail.ApplyDefaultMode(DiffModeUnified)
	detail.ApplyDefaultContextMode(DiffContextFullFile)
	detail.ApplyDefaultWrapMode(DiffWrapOn)
	detail.SetDiffHighContrast(true)
	if detail.Mode() != DiffModeUnified || detail.ContextMode() != DiffContextFullFile || detail.WrapMode() != DiffWrapOn || !detail.DiffHighContrast() {
		t.Fatalf("inherited detail state = mode %v context %v wrap %v contrast %v", detail.Mode(), detail.ContextMode(), detail.WrapMode(), detail.DiffHighContrast())
	}
	detail.SetMode(DiffModeSplit)
	detail.SetContextMode(DiffContextChangesOnly)
	detail.SetWrapMode(DiffWrapOff)
	detail.ApplyDefaultMode(DiffModeUnified)
	detail.ApplyDefaultContextMode(DiffContextFullFile)
	detail.ApplyDefaultWrapMode(DiffWrapOn)
	if detail.Mode() != DiffModeSplit || detail.ContextMode() != DiffContextChangesOnly || detail.WrapMode() != DiffWrapOff {
		t.Fatal("shared defaults replaced explicit detail choices")
	}
}

func TestCommitDetailRendersMetadataAndAllFilesInOneDocument(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
	detail.Metadata = "Authored Aug 22, 2026 at 3:14 AM -0400"
	detail.SetDetail("Subject\n\nBody", []CommitDetailFile{
		{Path: "first.txt", Diff: diff.Parse("--- a/first.txt\n+++ b/first.txt\n@@ -1 +1 @@\n-old first\n+new first\n")},
		{Path: "second.txt", Diff: diff.Parse("--- a/second.txt\n+++ b/second.txt\n@@ -1 +1 @@\n-old second\n+new second\n")},
	}, "")
	detail.SetRect(Rect{W: 80, H: 20})
	cells := makeGrid(80, 20)
	detail.Render(NewRenderSurface(cells, Rect{W: 80, H: 20}))
	text := detailTestText(cells)
	for _, want := range []string{"Authored Aug 22, 2026 at 3:14 AM -0400", "Subject", "Body", "first.txt", "old first", "new first", "second.txt", "new second"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail document missing %q:\n%s", want, text)
		}
	}
}

func TestCommitDetailWholeFullwidthHeadingRowTogglesAtNarrowWidth(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{
		Path: "資料/界面.txt",
		Diff: diff.Parse("--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n"),
	}}, "")
	detail.SetRect(Rect{X: 7, Y: 4, W: 12, H: 8})
	detail.Render(NewRenderSurface(makeGrid(12, 8), Rect{X: 7, Y: 4, W: 12, H: 8}))
	if len(detail.fileControls) == 0 || detail.fileControls[0].rect.W != 12 {
		t.Fatalf("heading hit region = %+v, want whole 12-column row", detail.fileControls)
	}
	control := detail.fileControls[0]
	result := detail.HandleEvent(tcell.NewEventMouse(control.rect.X+control.rect.W-1, control.rect.Y, tcell.Button1, tcell.ModNone))
	if result != EventConsumed || !detail.collapsedFiles[0] {
		t.Fatalf("far-edge title click result=%v collapsed=%v", result, detail.collapsedFiles[0])
	}
}

func TestCommitDetailUsesSharedContextProjectionAndHighContrastPainter(t *testing.T) {
	fileDiff := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -2 +2 @@\n-old\n+new\n")
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
	requested := -1
	detail.OnFetchContext = func(fileIndex int, _ CommitDetailFile) { requested = fileIndex }
	detail.ApplyDefaultContextMode(DiffContextFullFile)
	detail.SetDetail("Subject", []CommitDetailFile{{Path: "test.go", Diff: fileDiff}}, "")
	if requested != 0 {
		t.Fatalf("full context requested file %d, want 0", requested)
	}
	if !detail.ApplyFileContext(0, CommitDetailContextKey(detail.Files[0]), []string{"before", "old", "after"}, []string{"before", "new", "after"}) {
		t.Fatal("context result was rejected")
	}
	if len(detail.Files[0].lines) != 3 {
		t.Fatalf("shared full-file projection has %d lines, want 3", len(detail.Files[0].lines))
	}
	detail.SetDiffHighContrast(true)
	detail.SetRect(Rect{W: 60, H: 10})
	cells := makeGrid(60, 10)
	detail.Render(NewRenderSurface(cells, Rect{W: 60, H: 10}))
	foundSharedForeground := false
	for _, row := range cells {
		for _, cell := range row {
			if cell.Style == term.StyleGutterAdded && cell.BgStyle == term.StyleDiffAdded {
				foundSharedForeground = true
			}
		}
	}
	if !foundSharedForeground {
		t.Fatal("detail did not route high-contrast diff text through the shared painter")
	}
}

func TestFullFileLCSProjectionIsSharedByHistoricalAndCurrentChanges(t *testing.T) {
	oldLines := []string{"", "a", "c", "same"}
	newLines := []string{"same", "a", "b", "b", "c", "c"}
	fileDiff := diff.Parse(diff.Generate(oldLines, newLines, "file.txt"))
	want := diff.FullDiffLines(oldLines, newLines)

	tests := []struct {
		name   string
		detail *CommitDetailWidget
		apply  func(*CommitDetailWidget, CommitDetailFile)
	}{
		{
			name:   "historical commit detail",
			detail: NewCommitDetailWidget("/repo", "full", "abcdef0", false),
			apply: func(detail *CommitDetailWidget, file CommitDetailFile) {
				detail.SetDetail("subject", []CommitDetailFile{file}, "")
			},
		},
		{
			name:   "current changes",
			detail: NewCurrentChangesWidget("/repo", false),
			apply: func(detail *CommitDetailWidget, file CommitDetailFile) {
				detail.SetCurrentChanges("one file", []CommitDetailFile{file}, "")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := CommitDetailFileWithContent(CommitDetailFile{Status: "M", Path: "file.txt", Diff: fileDiff}, oldLines, newLines)
			test.apply(test.detail, file)
			test.detail.SetContextMode(DiffContextFullFile)
			if !reflect.DeepEqual(test.detail.Files[0].lines, want) {
				t.Fatal("Full File surface did not preserve the established LCS projection")
			}
		})
	}
}

func TestCommitDetailCollapsedContextUsesSharedExpansionControl(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{
		Path: "test.go",
		Diff: diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n-old one\n+new one\n@@ -10,1 +10,1 @@\n-old ten\n+new ten\n"),
	}}, "")
	requests := 0
	detail.OnFetchContext = func(fileIndex int, _ CommitDetailFile) {
		if fileIndex != 0 {
			t.Fatalf("context request file = %d, want 0", fileIndex)
		}
		requests++
	}
	detail.SetRect(Rect{X: 4, Y: 3, W: 60, H: 15})
	detail.Render(NewRenderSurface(makeGrid(60, 15), Rect{X: 4, Y: 3, W: 60, H: 15}))

	gapRow, gap := -1, -1
	for rowIndex, row := range detail.rows {
		if row.kind != commitDetailDiffRow {
			continue
		}
		if candidate, ok := detail.Files[0].gapByLine[row.lineIndex]; ok {
			gapRow, gap = rowIndex, candidate
			break
		}
	}
	if gapRow < 0 {
		t.Fatalf("commit detail has no collapsed context row: hunks=%d lines=%+v gaps=%v", len(detail.Files[0].Diff.Hunks), detail.Files[0].lines, detail.Files[0].gapByLine)
	}
	press := tcell.NewEventMouse(20, detail.GetRect().Y+gapRow, tcell.Button1, tcell.ModNone)
	if result := detail.HandleEvent(press); result != EventConsumed {
		t.Fatalf("collapsed context click result = %v", result)
	}
	detail.HandleEvent(press)
	if requests != 1 {
		t.Fatalf("held click started %d context reads, want 1", requests)
	}
	detail.HandleEvent(tcell.NewEventMouse(20, detail.GetRect().Y+gapRow, tcell.ButtonNone, tcell.ModNone))

	oldLines := []string{"old one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "old ten"}
	newLines := []string{"new one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "new ten"}
	if !detail.ApplyFileContext(0, CommitDetailContextKey(detail.Files[0]), oldLines, newLines) {
		t.Fatal("collapsed context result was rejected")
	}
	if !detail.Files[0].expandedGaps[gap] || len(detail.Files[0].gapByLine) != 0 {
		t.Fatalf("shared context gap remained collapsed: expanded=%v gaps=%v", detail.Files[0].expandedGaps, detail.Files[0].gapByLine)
	}
}

func TestCommitDetailWrappedCollapsedContextUsesFullRowHitTarget(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		t.Run(fmt.Sprintf("mode-%d", mode), func(t *testing.T) {
			fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n-old one\n+new one\n@@ -20,1 +20,1 @@\n-old twenty\n+new twenty\n")
			oldLines := make([]string, 20)
			newLines := make([]string, 20)
			for index := range oldLines {
				oldLines[index] = fmt.Sprintf("old line %d", index+1)
				newLines[index] = fmt.Sprintf("new line %d", index+1)
			}
			file := CommitDetailFileWithContent(CommitDetailFile{Path: "test.go", Diff: fd}, oldLines, newLines)
			detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
			detail.SetDetail("Subject", []CommitDetailFile{file}, "")
			detail.SetMode(mode)
			detail.SetWrapMode(DiffWrapOn)
			const width, height = 20, 16
			rect := Rect{X: 3, Y: 2, W: width, H: height}
			detail.SetRect(rect)
			detail.Render(NewRenderSurface(makeGrid(width, height), rect))
			gapRow := -1
			for rowIndex, row := range detail.rows {
				if row.kind == commitDetailDiffRow {
					lineIndex := row.lineIndex
					if mode == DiffModeUnified {
						lineIndex = detail.Files[0].unified[lineIndex].sourceLine
					}
					if _, ok := detail.Files[0].gapByLine[lineIndex]; ok {
						gapRow = rowIndex
						break
					}
				}
			}
			clickY := -1
			for visualIndex, visual := range detail.visualRows {
				if visual.row == gapRow && visualIndex >= detail.TopLine && visualIndex < detail.TopLine+detail.viewH {
					clickY = rect.Y + visualIndex - detail.TopLine
				}
			}
			if clickY < 0 {
				t.Fatalf("wrapped detail separator mapping missing: row=%d visual=%+v", gapRow, detail.visualRows)
			}
			if result := detail.HandleEvent(tcell.NewEventMouse(rect.X+rect.W-2, clickY, tcell.Button1, tcell.ModNone)); result != EventConsumed {
				t.Fatalf("wrapped aggregate gap click result = %v", result)
			}
			if len(detail.Files[0].gapByLine) != 0 || detail.ContextMode() != DiffContextChangesOnly {
				t.Fatalf("wrapped aggregate gap not locally expanded: gaps=%v mode=%v", detail.Files[0].gapByLine, detail.ContextMode())
			}
		})
	}
}
