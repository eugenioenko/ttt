package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

func TestCommitDetailDefaultsRemainInheritedUntilViewOverride(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)

	detail.ApplyDefaultMode(DiffModeUnified)
	detail.ApplyDefaultWrapMode(DiffWrapOn)
	if detail.Mode() != DiffModeUnified || detail.WrapMode() != DiffWrapOn {
		t.Fatalf("inherited defaults = mode %v wrap %v, want unified/on", detail.Mode(), detail.WrapMode())
	}

	detail.SetMode(DiffModeSplit)
	detail.SetWrapMode(DiffWrapOff)
	detail.ApplyDefaultMode(DiffModeUnified)
	detail.ApplyDefaultWrapMode(DiffWrapOn)
	if detail.Mode() != DiffModeSplit || detail.WrapMode() != DiffWrapOff {
		t.Fatalf("defaults replaced View overrides: mode %v wrap %v", detail.Mode(), detail.WrapMode())
	}
}

func TestCurrentChangesUsesNativeHeaderAndPreservesCollapseOnRefresh(t *testing.T) {
	detail := NewCurrentChangesWidget("/repo", false)
	first := CommitDetailFile{Path: "first.go", Heading: "M  first.go · unstaged"}
	second := CommitDetailFile{Path: "second.go", Heading: "A  second.go · staged"}
	detail.SetDetail("2 files · +1 −1", []CommitDetailFile{first, second}, "")
	detail.toggleFile(1)
	detail.TopLine = 3

	detail.SetDetail("2 files · +2 −1", []CommitDetailFile{second, first}, "")
	if detail.Header != "Current changes" || !detail.CurrentChanges {
		t.Fatalf("current change-set identity = (%q, %v)", detail.Header, detail.CurrentChanges)
	}
	if !detail.collapsedFiles[0] || detail.collapsedFiles[1] {
		t.Fatalf("collapse state did not follow file identity: %v", detail.collapsedFiles)
	}
	if detail.TopLine != 3 {
		t.Fatalf("refresh reset scroll to %d", detail.TopLine)
	}
	if got := detail.rows[0].text; got != "Current changes" {
		t.Fatalf("document header = %q", got)
	}
}

func TestCurrentChangesCleanStateHasOneMessage(t *testing.T) {
	detail := NewCurrentChangesWidget("/repo", false)
	detail.SetDetail("Working tree clean", nil, "")
	if len(detail.rows) != 3 {
		t.Fatalf("clean current changes rendered redundant rows: %+v", detail.rows)
	}
	if detail.rows[1].text != "Working tree clean" {
		t.Fatalf("clean state message = %q", detail.rows[1].text)
	}
}

func TestCommitDetailModeAndWrapDefaultsAreIndependent(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetWrapMode(DiffWrapOn)

	detail.ApplyDefaultMode(DiffModeUnified)
	detail.ApplyDefaultWrapMode(DiffWrapOff)
	if detail.Mode() != DiffModeUnified {
		t.Fatalf("wrap override prevented inherited mode update: %v", detail.Mode())
	}
	if detail.WrapMode() != DiffWrapOn {
		t.Fatalf("wrap default replaced explicit wrapping: %v", detail.WrapMode())
	}
}

func commitDetailGridText(cells [][]term.Cell) string {
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

func TestCommitDetailRendersMessageAndOrderedFileDiffsInOneScroll(t *testing.T) {
	first := diff.Parse("--- a/first.txt\n+++ b/first.txt\n@@ -1 +1 @@\n-old first\n+new first\n")
	second := diff.Parse("--- a/second.txt\n+++ b/second.txt\n@@ -1 +1 @@\n-old second\n+new second\n")
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject line\n\nBody paragraph", []CommitDetailFile{
		{Path: "first.txt", Diff: first},
		{Path: "second.txt", Diff: second},
	}, "")
	detail.SetRect(Rect{X: 0, Y: 0, W: 80, H: 7})

	initial := makeGrid(80, 7)
	detail.Render(NewRenderSurface(initial, Rect{X: 0, Y: 0, W: 80, H: 7}))
	initialText := commitDetailGridText(initial)
	for _, want := range []string{"Subject line", "Body paragraph", "first.txt", "old first", "new first"} {
		if !strings.Contains(initialText, want) {
			t.Errorf("initial viewport missing %q:\n%s", want, initialText)
		}
	}
	if strings.Contains(initialText, "second.txt") {
		t.Fatalf("second file should begin below the initial viewport:\n%s", initialText)
	}

	detail.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModNone))
	end := makeGrid(80, 7)
	detail.Render(NewRenderSurface(end, Rect{X: 0, Y: 0, W: 80, H: 7}))
	endText := commitDetailGridText(end)
	for _, want := range []string{"second.txt", "old second", "new second"} {
		if !strings.Contains(endText, want) {
			t.Errorf("end viewport missing %q:\n%s", want, endText)
		}
	}

	firstHeading, secondHeading := -1, -1
	for index, row := range detail.rows {
		if row.kind == commitDetailHeadingRow && row.text == "first.txt" {
			firstHeading = index
		}
		if row.kind == commitDetailHeadingRow && row.text == "second.txt" {
			secondHeading = index
		}
	}
	if firstHeading < 0 || secondHeading <= firstHeading {
		t.Fatalf("file headings are not in Git order: first=%d second=%d", firstHeading, secondHeading)
	}
}

func TestCommitDetailKeepsTouchedFilesWithoutLineHunks(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Rename only", []CommitDetailFile{{
		Path:    "new.txt",
		OldPath: "old.txt",
	}}, "")

	rows := make([]string, 0, len(detail.rows))
	for _, row := range detail.rows {
		rows = append(rows, row.text)
	}
	joined := strings.Join(rows, "\n")
	if !strings.Contains(joined, "old.txt → new.txt") || !strings.Contains(joined, "No line changes") {
		t.Fatalf("hunk-less file disappeared from detail rows:\n%s", joined)
	}
}

func TestCommitDetailDoesNotDrawWideRuneAcrossClipEdge(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	cells := makeGrid(3, 1)
	surface := NewRenderSurface(cells, Rect{X: 0, Y: 0, W: 3, H: 1})
	detail.drawText(surface, 0, 0, 3, "ab界", term.StyleDefault, term.StyleDefault, false, 0)

	if cells[0][2].Ch != ' ' {
		t.Fatalf("last clipped cell = %q, want a space instead of wide-rune bleed", cells[0][2].Ch)
	}
}

func TestCommitDetailCollapsedSeparatorUsesSharedStyle(t *testing.T) {
	fileDiff := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{Path: "test.go", Diff: fileDiff}}, "")

	separatorRow := -1
	for rowIndex, row := range detail.rows {
		if row.kind != commitDetailDiffRow {
			continue
		}
		if detail.Files[row.fileIndex].lines[row.lineIndex].Left.Kind == diff.Collapsed {
			separatorRow = rowIndex
			break
		}
	}
	if separatorRow < 0 {
		t.Fatal("commit detail has no collapsed separator row")
	}

	const width, height = 60, 8
	cells := makeGrid(width, height)
	detail.SetRect(Rect{W: width, H: height})
	detail.Render(NewRenderSurface(cells, Rect{W: width, H: height}))
	if got := cells[separatorRow][0].Style; got != term.StyleDiffCollapsed {
		t.Fatalf("collapsed gutter style = %v, want StyleDiffCollapsed", got)
	}
	if got := cells[separatorRow][detail.gutterW].Style; got != term.StyleDiffCollapsed {
		t.Fatalf("collapsed text style = %v, want StyleDiffCollapsed", got)
	}
}

func TestCommitDetailWrapsLongDiffRowsWithoutRepeatingGutters(t *testing.T) {
	fileDiff := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-left-prefix-LEFT-SUFFIX\n+right-prefix-RIGHT-SUFFIX\n")
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{Path: "test.go", Diff: fileDiff}}, "")
	detail.SetWrapped(true)

	const width, height = 30, 8
	cells := makeGrid(width, height)
	detail.SetRect(Rect{W: width, H: height})
	detail.Render(NewRenderSurface(cells, Rect{W: width, H: height}))
	text := commitDetailGridText(cells)
	if !strings.Contains(text, "LEFT-SUF") || !strings.Contains(text, "RIGHT-SUF") {
		t.Fatalf("wrapped commit detail should expose both long suffixes:\n%s", text)
	}
	if detail.hscrollbar.TotalCols != 0 || detail.rhscroll.TotalCols != 0 {
		t.Fatalf("wrapped detail retained horizontal scrollbars: %d / %d", detail.hscrollbar.TotalCols, detail.rhscroll.TotalCols)
	}

	continuation := -1
	for visualIndex, visual := range detail.visualRows {
		if visual.continuation && detail.rows[visual.row].kind == commitDetailDiffRow {
			continuation = visualIndex
			break
		}
	}
	if continuation < 0 || continuation >= height {
		t.Fatalf("expected a visible diff continuation, got row %d and map %+v", continuation, detail.visualRows)
	}
	if strings.Contains(commitDetailGridText(cells[continuation : continuation+1])[:detail.gutterW], "1") {
		t.Fatalf("continuation gutter repeated its line number: %q", commitDetailGridText(cells[continuation : continuation+1])[:detail.gutterW])
	}
}

func TestCommitDetailUnifiedModeUsesSharedProjection(t *testing.T) {
	fileDiff := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,2 +1,2 @@\n-old one\n-old two\n+new one\n+new two\n")
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{Path: "test.go", Diff: fileDiff}}, "")
	detail.SetMode(DiffModeUnified)

	if detail.Mode() != DiffModeUnified {
		t.Fatal("commit detail did not expose unified as its current mode")
	}
	got := make([]string, len(detail.Files[0].unified))
	for i, line := range detail.Files[0].unified {
		got[i] = line.side.Text
	}
	want := []string{"old one", "old two", "new one", "new two"}
	if len(got) < len(want) || strings.Join(got[:len(want)], "|") != strings.Join(want, "|") {
		t.Fatalf("commit detail unified order = %v, want %v", got, want)
	}

	const width, height = 60, 8
	cells := makeGrid(width, height)
	detail.SetRect(Rect{W: width, H: height})
	detail.Render(NewRenderSurface(cells, Rect{W: width, H: height}))
	text := commitDetailGridText(cells)
	oldOne, oldTwo := strings.Index(text, "old one"), strings.Index(text, "old two")
	newOne, newTwo := strings.Index(text, "new one"), strings.Index(text, "new two")
	if oldOne < 0 || oldTwo < oldOne || newOne < oldTwo || newTwo < newOne {
		t.Fatalf("commit detail did not render removals before additions:\n%s", text)
	}
}

func TestCommitDetailUnifiedWrapUsesFullContentWidth(t *testing.T) {
	fileDiff := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-left-prefix-LEFT-SUFFIX\n+right-prefix-RIGHT-SUFFIX\n")
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{Path: "test.go", Diff: fileDiff}}, "")
	detail.SetMode(DiffModeUnified)
	detail.SetWrapMode(DiffWrapOn)

	const width, height = 30, 8
	cells := makeGrid(width, height)
	detail.SetRect(Rect{W: width, H: height})
	detail.Render(NewRenderSurface(cells, Rect{W: width, H: height}))
	text := commitDetailGridText(cells)
	for _, suffix := range []string{"LEFT-SUFFIX", "RIGHT-SUFFIX"} {
		if !strings.Contains(text, suffix) {
			t.Fatalf("unified wrapped detail missing %q:\n%s", suffix, text)
		}
	}
	if detail.hscrollbar.TotalCols != 0 || detail.rhscroll.TotalCols != 0 {
		t.Fatalf("unified wrapped detail retained horizontal scrollbars: %d / %d", detail.hscrollbar.TotalCols, detail.rhscroll.TotalCols)
	}
}

func TestCommitDetailWrappedSelectionCopiesOriginalText(t *testing.T) {
	fileDiff := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-abcdefghijklmno\n+ABCDEFGHIJKLMNO\n")
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{Path: "test.go", Diff: fileDiff}}, "")
	detail.SetWrapped(true)

	const width, height = 24, 10
	detail.SetRect(Rect{W: width, H: height})
	detail.Render(NewRenderSurface(makeGrid(width, height), Rect{W: width, H: height}))

	var segments []int
	for visualIndex, visual := range detail.visualRows {
		row := detail.rows[visual.row]
		if row.kind == commitDetailDiffRow && row.lineIndex == 0 {
			segments = append(segments, visualIndex)
		}
	}
	if len(segments) < 3 {
		t.Fatalf("expected three wrapped segments, got %v", detail.visualRows)
	}
	detail.HandleEvent(tcell.NewEventMouse(detail.layoutLeftStart+2, segments[0], tcell.Button1, tcell.ModNone))
	detail.HandleEvent(tcell.NewEventMouse(detail.layoutLeftStart+1, segments[2], tcell.Button1, tcell.ModNone))
	detail.HandleEvent(tcell.NewEventMouse(detail.layoutLeftStart+1, segments[2], tcell.ButtonNone, tcell.ModNone))

	selected := makeGrid(width, height)
	detail.Render(NewRenderSurface(selected, Rect{W: width, H: height}))
	foundSelection := false
	for _, row := range selected {
		for _, cell := range row {
			if cell.BgStyle == term.StyleSelection {
				foundSelection = true
			}
		}
	}
	if !foundSelection {
		t.Fatal("shared diff decorator did not paint commit-detail selection")
	}
	if got := detail.CopySelection(); got != "cdefghijklmno" {
		t.Fatalf("wrapped copy = %q, want original unwrapped text", got)
	}
}

func TestCommitDetailCollapseRebuildsExtentAndClearsSelection(t *testing.T) {
	first := diff.Parse("--- a/first.txt\n+++ b/first.txt\n@@ -1 +1 @@\n-old first\n+new first\n")
	second := diff.Parse("--- a/second.txt\n+++ b/second.txt\n@@ -1 +1 @@\n-old second\n+new second\n")
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{Path: "first.txt", Diff: first}, {Path: "second.txt", Diff: second}}, "")
	expandedRows := len(detail.rows)
	detail.hasSelection = true
	detail.selection.Anchor = diffSelPos{Line: 1, Col: 0}
	detail.selection.Current = diffSelPos{Line: 1, Col: 1}
	detail.TopLine = expandedRows

	detail.CollapseAllFiles()
	if !detail.allFilesCollapsed() {
		t.Fatal("collapse-all left an expanded file")
	}
	if detail.hasSelection {
		t.Fatal("collapse retained a selection whose logical rows were rebuilt")
	}
	for _, row := range detail.rows {
		if row.kind == commitDetailDiffRow || row.kind == commitDetailNoticeRow {
			t.Fatalf("collapsed detail retained content row %+v", row)
		}
	}
	if len(detail.rows) >= expandedRows {
		t.Fatalf("collapsed rows = %d, want fewer than expanded %d", len(detail.rows), expandedRows)
	}

	const width, height = 50, 6
	detail.SetRect(Rect{W: width, H: height})
	detail.Render(NewRenderSurface(makeGrid(width, height), Rect{W: width, H: height}))
	if detail.TopLine > max(0, detail.totalVisualRows-detail.viewH) {
		t.Fatalf("collapse left TopLine %d beyond extent %d", detail.TopLine, detail.totalVisualRows-detail.viewH)
	}
	if detail.scrollbar.TotalItems != 0 && detail.scrollbar.TotalItems != len(detail.rows) {
		t.Fatalf("collapsed scrollbar extent = %d, want %d", detail.scrollbar.TotalItems, len(detail.rows))
	}

	detail.ExpandAllFiles()
	if detail.allFilesCollapsed() || len(detail.rows) != expandedRows {
		t.Fatalf("expand-all restored %d rows, want %d", len(detail.rows), expandedRows)
	}
}

func TestCommitDetailSelectionCrossesCollapsedFileBoundary(t *testing.T) {
	files := []CommitDetailFile{
		{Path: "first.txt", Diff: diff.Parse("--- a/first.txt\n+++ b/first.txt\n@@ -1 +1 @@\n-old first\n+new first\n")},
		{Path: "second.txt", Diff: diff.Parse("--- a/second.txt\n+++ b/second.txt\n@@ -1 +1 @@\n-old second\n+new second\n")},
		{Path: "third.txt", Diff: diff.Parse("--- a/third.txt\n+++ b/third.txt\n@@ -1 +1 @@\n-old third\n+new third\n")},
	}
	for _, tc := range []struct {
		name    string
		mode    DiffMode
		wrapped bool
		want    string
	}{
		{name: "split", mode: DiffModeSplit, want: "old first\n\n\nsecond.txt\n\nthird.txt\nold third"},
		{name: "unified", mode: DiffModeUnified, want: "old first\nnew first\n\n\nsecond.txt\n\nthird.txt\nold third\nnew third"},
		{name: "wrapped", mode: DiffModeSplit, wrapped: true, want: "old first\n\n\nsecond.txt\n\nthird.txt\nold third"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
			detail.SetDetail("Subject", files, "")
			detail.SetMode(tc.mode)
			detail.SetWrapped(tc.wrapped)
			detail.toggleFile(1)

			startRow, endRow := -1, -1
			for rowIndex, row := range detail.rows {
				if row.kind != commitDetailDiffRow {
					continue
				}
				text, selectable := detail.rowText(rowIndex, false)
				if !selectable || text == "" {
					continue
				}
				if row.fileIndex == 0 && startRow < 0 {
					startRow = rowIndex
				}
				if row.fileIndex == 2 {
					endRow = rowIndex
				}
			}
			if startRow < 0 || endRow < 0 {
				t.Fatalf("missing boundary diff rows: start=%d end=%d rows=%+v", startRow, endRow, detail.rows)
			}
			endText, ok := detail.rowText(endRow, false)
			if !ok {
				t.Fatalf("end row %d has no selectable text", endRow)
			}
			detail.hasSelection = true
			detail.selection.Anchor = diffSelPos{Line: startRow, Col: 0}
			detail.selection.Current = diffSelPos{Line: endRow, Col: len([]rune(endText))}

			got := detail.CopySelection()
			if got != tc.want {
				t.Fatalf("copy across collapsed boundary = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, "old second") || strings.Contains(got, "new second") {
				t.Fatalf("copy included collapsed file contents: %q", got)
			}
		})
	}
}

func TestCommitDetailHeadingControlCollapsesOnlyItsFile(t *testing.T) {
	fileDiff := diff.Parse("--- a/test.txt\n+++ b/test.txt\n@@ -1 +1 @@\n-old\n+new\n")
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{Path: "first.txt", Diff: fileDiff}, {Path: "second.txt", Diff: fileDiff}}, "")
	const width, height = 60, 12
	detail.SetRect(Rect{W: width, H: height})
	detail.Render(NewRenderSurface(makeGrid(width, height), Rect{W: width, H: height}))
	if len(detail.fileControls) < 2 {
		t.Fatalf("rendered controls = %v, want one per heading", detail.fileControls)
	}
	control := detail.fileControls[0]
	if control.rect.W != detail.layoutViewW {
		t.Fatalf("heading hit target width = %d, want full row %d", control.rect.W, detail.layoutViewW)
	}
	titleX := control.rect.X + 6
	detail.HandleEvent(tcell.NewEventMouse(titleX, control.rect.Y, tcell.Button1, tcell.ModNone))
	if !detail.collapsedFiles[0] || detail.collapsedFiles[1] {
		t.Fatalf("per-file collapse state = %v, want [true false]", detail.collapsedFiles)
	}
	detail.HandleEvent(tcell.NewEventMouse(titleX+3, control.rect.Y, tcell.Button1, tcell.ModNone))
	if !detail.collapsedFiles[0] {
		t.Fatal("held movement across a heading toggled disclosure a second time")
	}
	detail.HandleEvent(tcell.NewEventMouse(titleX+3, control.rect.Y, tcell.ButtonNone, tcell.ModNone))
}

func TestCommitDetailTopControlCollapsesAndExpandsAllFiles(t *testing.T) {
	fileDiff := diff.Parse("--- a/test.txt\n+++ b/test.txt\n@@ -1 +1 @@\n-old\n+new\n")
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{Path: "first.txt", Diff: fileDiff}, {Path: "second.txt", Diff: fileDiff}}, "")
	const width, height = 60, 12
	detail.SetRect(Rect{W: width, H: height})
	detail.Render(NewRenderSurface(makeGrid(width, height), Rect{W: width, H: height}))
	if detail.topControl.W == 0 {
		t.Fatal("commit header did not expose collapse-all control")
	}
	detail.HandleEvent(tcell.NewEventMouse(detail.topControl.X, detail.topControl.Y, tcell.Button1, tcell.ModNone))
	detail.HandleEvent(tcell.NewEventMouse(detail.topControl.X, detail.topControl.Y, tcell.ButtonNone, tcell.ModNone))
	if !detail.allFilesCollapsed() {
		t.Fatalf("top control did not collapse all files: %v", detail.collapsedFiles)
	}

	detail.Render(NewRenderSurface(makeGrid(width, height), Rect{W: width, H: height}))
	detail.HandleEvent(tcell.NewEventMouse(detail.topControl.X, detail.topControl.Y, tcell.Button1, tcell.ModNone))
	detail.HandleEvent(tcell.NewEventMouse(detail.topControl.X, detail.topControl.Y, tcell.ButtonNone, tcell.ModNone))
	if detail.allFilesCollapsed() {
		t.Fatalf("top control did not expand all files: %v", detail.collapsedFiles)
	}
}

func TestCommitDetailMessageUsesDedicatedStyle(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
	detail.SetDetail("Subject\nBody", nil, "")
	const width, height = 30, 5
	cells := makeGrid(width, height)
	detail.SetRect(Rect{W: width, H: height})
	detail.Render(NewRenderSurface(cells, Rect{W: width, H: height}))
	for _, row := range []int{0, 1, 2} {
		if got := cells[row][0].Style; got != term.StyleCommitMessage {
			t.Fatalf("message row %d style = %v, want StyleCommitMessage", row, got)
		}
	}
}

func TestCommitDetailStickyHeadingIsSelectionInertButControlRemainsClickable(t *testing.T) {
	var patch strings.Builder
	patch.WriteString("--- a/file.go\n+++ b/file.go\n@@ -1,8 +1,8 @@\n")
	for i := 1; i <= 8; i++ {
		patch.WriteString("-old line ")
		patch.WriteString(strconv.Itoa(i))
		patch.WriteByte('\n')
	}
	for i := 1; i <= 8; i++ {
		patch.WriteString("+new line ")
		patch.WriteString(strconv.Itoa(i))
		patch.WriteByte('\n')
	}
	fileDiff := diff.Parse(patch.String())
	const longPath = "very/long/component/path/whose/filename-survives.go"

	for _, tc := range []struct {
		name    string
		mode    DiffMode
		wrapped bool
	}{{"split", DiffModeSplit, false}, {"unified", DiffModeUnified, false}, {"wrapped", DiffModeSplit, true}} {
		t.Run(tc.name, func(t *testing.T) {
			detail := NewCommitDetailWidget("/repo", "full-hash", "abc1234", false)
			detail.SetDetail("Subject", []CommitDetailFile{{Path: longPath, Diff: fileDiff}}, "")
			detail.SetMode(tc.mode)
			detail.SetWrapped(tc.wrapped)
			const width, height = 32, 5
			detail.SetRect(Rect{W: width, H: height})
			detail.Render(NewRenderSurface(makeGrid(width, height), Rect{W: width, H: height}))

			firstDiffVisual := -1
			if detail.IsWrapped() {
				for visualIndex, visual := range detail.visualRows {
					if detail.rows[visual.row].kind == commitDetailDiffRow {
						firstDiffVisual = visualIndex
						break
					}
				}
			} else {
				for rowIndex, row := range detail.rows {
					if row.kind == commitDetailDiffRow {
						firstDiffVisual = rowIndex
						break
					}
				}
			}
			if firstDiffVisual < 0 {
				t.Fatal("detail has no diff row")
			}
			detail.TopLine = firstDiffVisual
			cells := makeGrid(width, height)
			detail.Render(NewRenderSurface(cells, Rect{W: width, H: height}))
			firstRow := commitDetailGridText(cells[:1])
			if !strings.HasPrefix(firstRow, " ▼ …") || !strings.Contains(firstRow, "filename-survives.go") {
				t.Fatalf("sticky heading did not preserve the path tail:\n%s", firstRow)
			}
			selectionX := detail.layoutLeftStart + 1
			if pos, _, ok := detail.screenToSelection(selectionX, 0); ok {
				t.Fatalf("sticky row exposed covered diff position %+v", pos)
			}
			if result := detail.HandleEvent(tcell.NewEventMouse(selectionX, 0, tcell.Button1, tcell.ModNone)); result != EventConsumed {
				t.Fatalf("sticky title press result = %v, want consumed", result)
			}
			if !detail.collapsedFiles[0] {
				t.Fatal("sticky title press did not collapse its file")
			}
			detail.HandleEvent(tcell.NewEventMouse(selectionX, 0, tcell.ButtonNone, tcell.ModNone))
			detail.HandleEvent(tcell.NewEventMouse(selectionX, 0, tcell.Button1, tcell.ModNone))
			if detail.collapsedFiles[0] {
				t.Fatal("second sticky title press did not expand its file")
			}
			detail.HandleEvent(tcell.NewEventMouse(selectionX, 0, tcell.ButtonNone, tcell.ModNone))
			detail.TopLine = firstDiffVisual
			cells = makeGrid(width, height)
			detail.Render(NewRenderSurface(cells, Rect{W: width, H: height}))
			if detail.hasSelection || detail.selecting {
				t.Fatal("sticky title press started a selection")
			}

			start, _, ok := detail.screenToSelection(selectionX, 1)
			if !ok {
				t.Fatal("row below sticky heading is not selectable")
			}
			detail.HandleEvent(tcell.NewEventMouse(selectionX, 1, tcell.Button1, tcell.ModNone))
			detail.HandleEvent(tcell.NewEventMouse(selectionX, 0, tcell.Button1, tcell.ModNone))
			if detail.selection.Current != start {
				t.Fatalf("drag across sticky row moved selection to covered content: got %+v want %+v", detail.selection.Current, start)
			}
			detail.HandleEvent(tcell.NewEventMouse(selectionX, 0, tcell.ButtonNone, tcell.ModNone))

			control := detail.stickyControl
			if result := detail.HandleEvent(tcell.NewEventMouse(control.rect.X, control.rect.Y, tcell.Button1, tcell.ModNone)); result != EventConsumed {
				t.Fatalf("sticky collapse result = %v, want consumed", result)
			}
			if !detail.collapsedFiles[control.fileIndex] {
				t.Fatal("sticky collapse control did not collapse its file")
			}
		})
	}
}

func TestTruncateCommitDetailPathUsesDisplayWidthAndKeepsTail(t *testing.T) {
	got := truncateCommitDetailPath("prefix/界/filename.go", 12)
	if textwidth.String(got) > 12 {
		t.Fatalf("truncated path width = %d, want <= 12: %q", textwidth.String(got), got)
	}
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "filename.go") {
		t.Fatalf("left-truncated path = %q, want ellipsis and filename tail", got)
	}
}
