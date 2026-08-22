package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/gdamore/tcell/v3"
)

func currentChangesTestFile(path string, stage CommitDetailStage, oldLines, newLines []string) CommitDetailFile {
	file := CommitDetailFile{
		Status: "M", Path: path, Stage: stage, Diff: diff.Parse(diff.Generate(oldLines, newLines, path)),
	}
	return CommitDetailFileWithContent(file, oldLines, newLines)
}

func TestCurrentChangesUsesTypedHeadingAndClickableSharedFileRows(t *testing.T) {
	detail := NewCurrentChangesWidget("/repo", false)
	file := currentChangesTestFile("wide/界\nname.go", CommitDetailStageMixed,
		[]string{"old"}, []string{"new"})
	detail.SetCurrentChanges("1 file", []CommitDetailFile{file}, "")
	detail.SetRect(Rect{W: 80, H: 10})
	cells := makeGrid(80, 10)
	detail.Render(NewRenderSurface(cells, Rect{W: 80, H: 10}))
	text := detailTestText(cells)
	if got := displayCommitDetailPath(file.Path); got != `"wide/界\nname.go"` {
		t.Fatalf("raw path presentation = %q", got)
	}
	if !strings.Contains(text, `\nname.go" · mixed`) || !strings.Contains(text, "M  \"") {
		t.Fatalf("typed current-change heading was not safely presented:\n%s", text)
	}
	if len(detail.fileControls) != 1 {
		t.Fatalf("file controls = %d, want 1", len(detail.fileControls))
	}
	control := detail.fileControls[0]
	if result := detail.HandleEvent(tcell.NewEventMouse(control.rect.X+4, control.rect.Y, tcell.Button1, tcell.ModNone)); result != EventConsumed {
		t.Fatalf("heading click result = %v", result)
	}
	if !detail.collapsedFiles[0] {
		t.Fatal("current-change heading click did not collapse the file")
	}
}

func TestCurrentChangesRefreshPreservesLastGoodExpansionScrollAndContext(t *testing.T) {
	detail := NewCurrentChangesWidget("/repo", false)
	oldLines := []string{"zero", "old", "two", "three", "four", "five", "six", "seven"}
	newLines := []string{"zero", "new", "two", "three", "four", "five", "six", "seven"}
	file := currentChangesTestFile("file.txt", CommitDetailStageUnstaged, oldLines, newLines)
	detail.SetCurrentChanges("first", []CommitDetailFile{file}, "")
	detail.SetRect(Rect{W: 40, H: 4})
	detail.Render(NewRenderSurface(makeGrid(40, 4), Rect{W: 40, H: 4}))
	detail.collapsedFiles[0] = true
	detail.TopLine = 2

	refreshed := currentChangesTestFile("file.txt", CommitDetailStageUnstaged, oldLines,
		[]string{"zero", "latest", "two", "three", "four", "five", "six", "seven"})
	detail.SetCurrentChanges("second", []CommitDetailFile{refreshed}, "")
	if !detail.collapsedFiles[0] {
		t.Fatal("refresh lost file expansion identity")
	}
	if detail.TopLine == 0 {
		t.Fatal("refresh reset the current document scroll position")
	}
	if detail.Files[0].FullFileState != CommitDetailFullFileLoaded {
		t.Fatalf("refreshed Full File state = %v, want loaded", detail.Files[0].FullFileState)
	}

	detail.SetCurrentChanges("", nil, "temporary read failure")
	if detail.Message != "second" || len(detail.Files) != 1 || detail.Error != "" || detail.RefreshError == "" {
		t.Fatalf("failed refresh replaced last good state: message=%q files=%d error=%q refresh=%q",
			detail.Message, len(detail.Files), detail.Error, detail.RefreshError)
	}
	detail.SetCurrentChanges("recovered", []CommitDetailFile{refreshed}, "")
	if detail.RefreshError != "" || detail.Message != "recovered" {
		t.Fatalf("successful retry did not clear failure: message=%q refresh=%q", detail.Message, detail.RefreshError)
	}
}

func TestCurrentChangesBinaryAndEmptyFilesRemainVisibleInFullFileMode(t *testing.T) {
	detail := NewCurrentChangesWidget("/repo", false)
	detail.SetContextMode(DiffContextFullFile)
	detail.SetCurrentChanges("two files", []CommitDetailFile{
		{Status: "M", Path: "blob.bin", Stage: CommitDetailStageUnstaged, ContentKind: CommitDetailContentBinary, FullFileState: CommitDetailFullFileLoaded},
		{Status: "A", Path: "empty.txt", Stage: CommitDetailStageStaged, ContentKind: CommitDetailContentEmpty, FullFileState: CommitDetailFullFileLoaded},
	}, "")
	detail.SetRect(Rect{W: 80, H: 12})
	cells := makeGrid(80, 12)
	detail.Render(NewRenderSurface(cells, Rect{W: 80, H: 12}))
	text := detailTestText(cells)
	for _, want := range []string{"Binary file changed", "Empty file added", "M  blob.bin · unstaged", "A  empty.txt · staged"} {
		if !strings.Contains(text, want) {
			t.Fatalf("current changes omitted %q in Full File mode:\n%s", want, text)
		}
	}
}

func TestCurrentChangesSamePathBoundariesPreserveIndependentCollapseAndSelection(t *testing.T) {
	detail := NewCurrentChangesWidget("/repo", false)
	staged := currentChangesTestFile("file.txt", CommitDetailStageStaged, []string{"head"}, []string{"index"})
	staged.Boundary = CommitDetailBoundaryHeadToIndex
	unstaged := currentChangesTestFile("file.txt", CommitDetailStageUnstaged, []string{"index"}, []string{"worktree"})
	unstaged.Boundary = CommitDetailBoundaryIndexToWorktree
	detail.SetCurrentChanges("1 file", []CommitDetailFile{staged, unstaged}, "")
	detail.collapsedFiles[0] = true
	detail.rebuildRows()
	selectedRow := -1
	for i, row := range detail.rows {
		if row.kind == commitDetailDiffRow && row.fileIndex == 1 {
			selectedRow = i
			break
		}
	}
	if selectedRow < 0 {
		t.Fatal("unstaged boundary has no selectable diff row")
	}
	detail.hasSelection = true
	detail.selRight = true
	detail.selection.Anchor = diffSelPos{Line: selectedRow, Col: 0}
	detail.selection.Current = diffSelPos{Line: selectedRow, Col: 4}

	refreshedStaged := currentChangesTestFile("file.txt", CommitDetailStageStaged, []string{"head"}, []string{"index"})
	refreshedStaged.Boundary = CommitDetailBoundaryHeadToIndex
	refreshedUnstaged := currentChangesTestFile("file.txt", CommitDetailStageUnstaged, []string{"index"}, []string{"worktree latest"})
	refreshedUnstaged.Boundary = CommitDetailBoundaryIndexToWorktree
	detail.SetCurrentChanges("1 file", []CommitDetailFile{refreshedStaged, refreshedUnstaged}, "")

	if !detail.collapsedFiles[0] || detail.collapsedFiles[1] {
		t.Fatalf("independent collapse state = %v", detail.collapsedFiles)
	}
	if !detail.hasSelection {
		t.Fatal("unstaged selection was lost when the same-path staged boundary refreshed")
	}
	row := detail.rows[detail.selection.Anchor.Line]
	if row.fileIndex != 1 || detail.Files[row.fileIndex].Boundary != CommitDetailBoundaryIndexToWorktree {
		t.Fatalf("selection moved across boundary: row=%+v files=%+v", row, detail.Files)
	}
}
