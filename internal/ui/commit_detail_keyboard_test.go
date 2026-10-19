package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestCommitDetailKeyboardDisclosures(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		for _, wrap := range []DiffWrapMode{DiffWrapOff, DiffWrapOn} {
			t.Run(fmt.Sprintf("%d-%d", mode, wrap), func(t *testing.T) {
				d := twoHunkDetail()
				d.SetMode(mode)
				d.SetWrapMode(wrap)
				d.SetFocused(true)
				render := func() [][]term.Cell { return renderDetail(d, 60, 8) }
				press := func(key tcell.Key, text string) {
					t.Helper()
					if result := d.HandleEvent(tcell.NewEventKey(key, text, 0)); result != EventConsumed {
						t.Fatalf("key %v was not consumed", key)
					}
					render()
				}
				render()
				press(tcell.KeyTab, "")
				grid := render()
				marked := false
				for _, row := range grid {
					for _, cell := range row {
						marked = marked || cell.Ch == '>' && cell.Style == term.StyleButtonFocused
					}
				}
				if !marked {
					t.Fatal("keyboard selection has no visible marker")
				}
				press(tcell.KeyEnter, "")
				if !d.allFilesCollapsed() {
					t.Fatal("Enter did not collapse all files")
				}
				press(tcell.KeyRune, " ")
				press(tcell.KeyTab, "")
				press(tcell.KeyEnter, "")
				if !d.collapsedFiles[0] {
					t.Fatal("file heading did not collapse")
				}
				press(tcell.KeyEnter, "")
				press(tcell.KeyTab, "")
				if d.selectedDisclosure.gap < 0 {
					t.Fatal("Tab skipped the context gap")
				}
				requested := false
				d.OnFetchContext = func(index int, file CommitDetailFile) { requested = index == 0 }
				press(tcell.KeyEnter, "")
				if !requested {
					t.Fatal("Enter did not request gap context")
				}
				oldLines, newLines := make([]string, 10), make([]string, 10)
				for i := range oldLines {
					oldLines[i], newLines[i] = "unchanged", "unchanged"
				}
				oldLines[0], newLines[0] = "old one", "new one"
				oldLines[9], newLines[9] = "old ten", "new ten"
				d.ApplyFileContext(0, CommitDetailContextKey(d.Files[0]), oldLines, newLines)
				render()
				if !d.keyboardDisclosure || d.selectedDisclosure != d.disclosure(0, -1) {
					t.Fatal("focus did not fall back to the file heading after the gap disappeared")
				}
				press(tcell.KeyBacktab, "")
				if d.selectedDisclosure.file != "" {
					t.Fatal("Shift+Tab did not reach Collapse All")
				}
			})
		}
	}
}

func TestCommitDetailKeyboardWrapAndRefreshIdentity(t *testing.T) {
	d := twoHunkDetail()
	d.CurrentChanges = true
	d.Files[0].Path = strings.Repeat("long-folder/", 12) + "file.go"
	d.SetCurrentChanges("message", d.Files, "")
	d.SetWrapMode(DiffWrapOn)
	d.SetFocused(true)
	renderDetail(d, 45, 10)
	for range 2 {
		d.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", 0))
		renderDetail(d, 45, 10)
	}
	selected := d.selectedDisclosure
	other := CommitDetailFile{Path: "another.txt"}
	d.SetCurrentChanges("refreshed message\nmore metadata", append([]CommitDetailFile{other}, d.Files...), "")
	renderDetail(d, 60, 10)
	if d.selectedDisclosure != selected {
		t.Fatal("refresh changed the selected file")
	}
	headings := 0
	for _, c := range d.disclosureRows() {
		if c.id == selected {
			headings++
		}
	}
	if headings != 1 {
		t.Fatalf("wrapped heading produced %d controls, want one", headings)
	}
	d.SetFocused(false)
	grid := renderDetail(d, 60, 10)
	for _, row := range grid {
		for _, c := range row {
			if c.Ch == '>' && c.Style == term.StyleButtonFocused {
				t.Fatal("blurred detail retained focus marker")
			}
		}
	}
}

func TestCommitDetailKeyboardPreservesCopyAndStickyHeading(t *testing.T) {
	d := twoHunkDetail()
	d.SetFocused(true)
	grid := renderDetail(d, 60, 15)
	x, y, ok := detailFind(grid, "new one")
	if !ok {
		t.Fatal("missing diff text")
	}
	d.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
	d.HandleEvent(tcell.NewEventMouse(x+7, y, tcell.Button1, 0))
	d.HandleEvent(tcell.NewEventMouse(x+7, y, tcell.ButtonNone, 0))
	for range 2 {
		d.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", 0))
		renderDetail(d, 60, 15)
	}
	if got := d.CopySelection(); got != "new one" {
		t.Fatalf("Tab changed copied text to %q", got)
	}
	for i, visual := range d.visualRows {
		if d.rows[visual.row].kind == commitDetailDiffRow {
			d.TopLine = i
			break
		}
	}
	grid = renderDetail(d, 60, 2)
	if d.stickyControl.fileIndex != 0 || grid[0][0].Ch != '>' || grid[0][0].Style != term.StyleButtonFocused {
		t.Fatalf("sticky=%+v selected=%+v keyboard=%v top=%d grid:\n%s", d.stickyControl, d.selectedDisclosure, d.keyboardDisclosure, d.TopLine, detailTestText(grid))
	}
}
