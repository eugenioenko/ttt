package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/core/logfile"
	"github.com/gdamore/tcell/v3"
)

func testLogViewer(t *testing.T, text string, w, h int) *LogViewerWidget {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := logfile.Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(file.Close)
	deadline := time.Now().Add(3 * time.Second)
	for file.Indexing() {
		if time.Now().After(deadline) {
			t.Fatal("index timeout")
		}
		time.Sleep(time.Millisecond)
	}
	v := NewLogViewer(file)
	v.SetRect(Rect{W: w, H: h})
	v.SetFocused(true)
	return v
}
func TestLogViewerFullwidthClippingAndMouse(t *testing.T) {
	v := testLogViewer(t, "界ab界cd\nabcde界\n", 11, 4)
	grid := makeGrid(11, 4)
	v.Render(NewRenderSurface(grid, Rect{W: 11, H: 4}))
	if text := gridText(grid); grid[0][v.gutter].Ch != '界' || grid[0][v.gutter+2].Ch != 'a' || grid[0][v.gutter+3].Ch != 'b' {
		t.Fatalf("text rendering lost characters:\n%s", text)
	}
	if grid[1][9].Ch == '界' {
		t.Fatal("fullwidth character crossed the text clip boundary")
	}
	v.HandleEvent(tcell.NewEventMouse(v.gutter+2, 0, tcell.Button1, 0))
	v.HandleEvent(tcell.NewEventMouse(v.gutter+3, 0, tcell.ButtonNone, 0))
	if v.Cursor.Col != 1 {
		t.Fatalf("click after a wide rune = column %d, want 1", v.Cursor.Col)
	}
}
func TestLogViewerSelectionSurvivesFileEdgesAndUnknownLineNotice(t *testing.T) {
	v := testLogViewer(t, strings.Repeat("line\n", 30000)+"end\n", 60, 10)
	v.Render(NewRenderSurface(makeGrid(60, 10), Rect{W: 60, H: 10}))
	v.MoveToFileEdge(false, false)
	v.MoveToFileEdge(true, true)
	start, end := v.Selection()
	if start.Offset != 0 || end.Offset < 100000 {
		t.Fatal("selection was limited to a loaded window")
	}
	var notice string
	v.OnNotice = func(s string) { notice = s }
	before := v.Cursor
	v.GoToLine(100000)
	if notice == "" || v.Cursor != before {
		t.Fatal("unindexed Go to Line silently clamped")
	}
}
func TestFileWordWrapSizeLimit(t *testing.T) {
	dir := t.TempDir()
	small, large := filepath.Join(dir, "small.txt"), filepath.Join(dir, "large.txt")
	os.WriteFile(small, []byte("1234567890"), 0o644)
	os.WriteFile(large, []byte("12345678901"), 0o644)
	g := NewEditorGroupWidget(nil, 4, true, "compact")
	g.WordWrap = true
	g.WordWrapMaxBytes = 10
	g.OpenFile(small)
	g.CommitActiveTab()
	if !g.Editor.WordWrap {
		t.Fatal("file at limit did not wrap")
	}
	g.OpenFile(large)
	g.CommitActiveTab()
	if g.Editor.WordWrap {
		t.Fatal("file over limit wrapped")
	}
	g.OpenFile(small)
	if !g.Editor.WordWrap {
		t.Fatal("small file did not restore wrap")
	}
	g.WordWrapMaxBytes = 0
	g.OpenFile(large)
	if !g.Editor.WordWrap {
		t.Fatal("zero limit did not allow wrap")
	}
	g.WordWrap = false
	g.OpenFile(small)
	if g.Editor.WordWrap {
		t.Fatal("disabled wrap was enabled")
	}
}

type unindexedLogSource struct{ LogSource }

func (s unindexedLogSource) Progress() logfile.Progress {
	p := s.LogSource.Progress()
	p.Lines = 1
	p.Scanned = 0
	p.Indexing = true
	return p
}
func (s unindexedLogSource) LineNumber(int64) (int, bool) { return 0, false }
func TestLogViewerCtrlEndBeforeLineNumbersAreKnown(t *testing.T) {
	v := testLogViewer(t, strings.Repeat("entry\n", 30000)+"actual tail\n", 60, 10)
	source := v.Source
	v.Source = unindexedLogSource{source}
	grid := makeGrid(60, 10)
	v.Render(NewRenderSurface(grid, Rect{W: 60, H: 10}))
	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModCtrl))
	v.Render(NewRenderSurface(grid, Rect{W: 60, H: 10}))
	if !strings.Contains(gridText(grid), "actual tail") || !v.Following {
		t.Fatal("Ctrl+End stopped at the indexed line count")
	}
	if _, known := v.Source.LineNumber(v.Cursor.Offset); known {
		t.Fatal("test unexpectedly has line numbers")
	}
	offset := v.Cursor.Offset
	v.Source = source
	v.Render(NewRenderSurface(grid, Rect{W: 60, H: 10}))
	if v.Cursor.Offset != offset || !strings.Contains(gridText(grid), "30001") {
		t.Fatal("finishing the index moved the view or failed to fill line numbers")
	}
}

func TestLogViewerAppendPreservesHorizontalScroll(t *testing.T) {
	v := testLogViewer(t, strings.Repeat("long entry ", 20)+"\n", 40, 5)
	v.Render(NewRenderSurface(makeGrid(40, 5), Rect{W: 40, H: 5}))
	v.MoveToFileEdge(true, false)
	v.HandleEvent(tcell.NewEventMouse(10, 1, tcell.WheelRight, 0))
	left := v.left
	if left == 0 {
		t.Fatal("fixture did not scroll horizontally")
	}
	file := v.Source.(*logfile.File)
	f, err := os.OpenFile(file.Path(), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(strings.Repeat("next entry ", 20) + "\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Refresh(); err != nil {
		t.Fatal(err)
	}
	v.RefreshProgress(false)
	if !v.Following || v.left != left {
		t.Fatalf("append changed horizontal scroll from %d to %d", left, v.left)
	}
	v.RefreshProgress(true)
	if v.left != 0 {
		t.Fatal("reset did not clear horizontal scroll")
	}
}
func TestLogViewerClickCopyAndDrag(t *testing.T) {
	v := testLogViewer(t, "first\nsecond\n", 40, 5)
	v.Render(NewRenderSurface(makeGrid(40, 5), Rect{W: 40, H: 5}))
	v.HandleEvent(tcell.NewEventMouse(v.gutter+1, 0, tcell.Button1, 0))
	v.HandleEvent(tcell.NewEventMouse(v.gutter+1, 0, tcell.ButtonNone, 0))
	start, end := v.Selection()
	text, _, err := v.Source.(*logfile.File).Copy(start, end)
	if err != nil || v.Selecting || text != "first\n" {
		t.Fatalf("copy after click = %q, selecting=%v, err=%v", text, v.Selecting, err)
	}
	v.HandleEvent(tcell.NewEventMouse(v.gutter, 0, tcell.Button1, 0))
	v.HandleEvent(tcell.NewEventMouse(v.gutter+3, 0, tcell.Button1, 0))
	v.HandleEvent(tcell.NewEventMouse(v.gutter+3, 0, tcell.ButtonNone, 0))
	start, end = v.Selection()
	text, _, err = v.Source.(*logfile.File).Copy(start, end)
	if err != nil || !v.Selecting || text != "fir" {
		t.Fatalf("copy after drag = %q, selecting=%v, err=%v", text, v.Selecting, err)
	}
}
func TestOpenInEditorKeepsCommittedLogTab(t *testing.T) {
	v := testLogViewer(t, "log content\n", 40, 5)
	path := v.Source.(*logfile.File).Path()
	g := NewEditorGroupWidget(nil, 4, true, "compact")
	g.OpenLogTab(path, v)
	g.OpenFileInEditor(path)
	other := filepath.Join(t.TempDir(), "other.txt")
	if err := os.WriteFile(other, []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g.OpenFile(other)
	if !g.SwitchToTabByPath(path) {
		t.Fatal("opening another file replaced the confirmed editor tab")
	}
}
