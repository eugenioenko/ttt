package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func linkEditor(lines []string, w, h int) *EditorPaneWidget {
	e := newRowsEditor(lines, w, h)
	e.OnOpenURL = func(string) {}
	e.Undo = &undo.UndoStack{}
	return e
}

func hoverEditorLinks(e *EditorPaneWidget) [][]term.Cell {
	renderGrid(e)
	r := e.GetRect()
	e.HandleEvent(tcell.NewEventMouse(r.X+e.GutterWidth(), r.Y, 0, tcell.ModCtrl))
	return renderGrid(e)
}

func TestEditorLinkFirstCtrlClick(t *testing.T) {
	for _, tt := range []struct {
		name string
		wrap bool
		left int
	}{
		{"plain", false, 0}, {"clipped", false, 12}, {"wrapped", true, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := linkEditor([]string{"界\t https://example.com/path"}, 23, 6)
			e.WordWrap, e.Viewport.LeftCol = tt.wrap, tt.left
			e.ensureMulti()
			e.Multi.Add(0, 2)
			cursors := len(e.Multi.Cursors)
			opened := 0
			e.OnOpenURL = func(url string) {
				opened++
				if url != "https://example.com/path" {
					t.Fatalf("opened %q", url)
				}
			}
			grid := renderGrid(e)
			x, y := e.GutterWidth()+5, 0
			if tt.wrap {
				x, y = e.GutterWidth()+2, 1
			}
			if grid[y][x].Underline {
				t.Fatal("link highlighted before Ctrl")
			}
			e.HandleEvent(tcell.NewEventMouse(x, y, 0, 0))
			e.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModCtrl))
			e.HandleEvent(tcell.NewEventMouse(x+1, y, tcell.Button1, tcell.ModCtrl))
			e.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
			if opened != 1 || e.mouseDown || e.Selection.Active || e.Multi == nil || len(e.Multi.Cursors) != cursors {
				t.Fatalf("opened=%d selecting=%v multicursor=%+v", opened, e.mouseDown, e.Multi)
			}
		})
	}
}

func TestEditorLinksRenderedGeometry(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		wrap       bool
		left       int
	}{
		{"punctuation", "(https://example.com), plain", false, 0},
		{"unicode and tabs", "界\t https://example.com/世界. plain", false, 0},
		{"invalid UTF-8 prefix", "\xff\t https://example.com/path", false, 0},
		{"horizontal clipping", "界\t https://example.com/世界. plain", false, 12},
		{"wrapped target", "界\t https://example.com/世界. plain", true, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := linkEditor([]string{tt.text}, 23, 8)
			e.WordWrap, e.Viewport.LeftCol = tt.wrap, tt.left
			var opened []string
			e.OnOpenURL = func(s string) { opened = append(opened, s) }
			grid := hoverEditorLinks(e)
			if len(e.links.hits) == 0 {
				t.Fatal("no visible links")
			}
			for p, span := range e.links.hits {
				if !grid[p.y][p.x].Underline || e.PointerShapeAt(p.x, p.y) != term.PointerHand {
					t.Fatalf("link feedback missing at %v", p)
				}
				for range 2 {
					e.HandleEvent(tcell.NewEventMouse(p.x, p.y, tcell.Button1, tcell.ModCtrl))
				}
				e.HandleEvent(tcell.NewEventMouse(p.x, p.y, 0, tcell.ModCtrl))
				if len(opened) != 1 || opened[0] != span.URL || e.mouseDown || e.Selection.Active {
					t.Fatalf("opened %v for %q, selecting=%v", opened, span.URL, e.mouseDown)
				}
				opened = nil
			}
			for y, row := range grid {
				for x, c := range row {
					if c.Underline {
						if _, ok := e.links.hits[linkPoint{x, y}]; !ok {
							t.Fatalf("decoration without hit at %d,%d", x, y)
						}
					}
				}
			}
		})
	}
}

func TestEditorLinksOnlyBufferText(t *testing.T) {
	e := linkEditor([]string{"https://visible.test", "hidden", "https://offscreen.test", "plain"}, 40, 8)
	e.Folds = fold.NewState()
	e.Folds.SetRanges([]fold.Range{{StartLine: 1, EndLine: 2}})
	e.Folds.Toggle(1)
	e.SetDiffOverlay(&DiffOverlay{Deleted: map[int][]diff.SideLine{0: {{Text: "https://deleted.test"}}}})
	grid := hoverEditorLinks(e)
	for p, span := range e.links.hits {
		if span.URL != "https://visible.test" || p.x < e.GutterWidth() || e.rowMap[p.y].isPhantom() {
			t.Fatalf("invalid hit at %v: %+v", p, span)
		}
	}
	if len(e.links.hits) == 0 || grid[0][e.GutterWidth()].Underline {
		t.Fatal("missing real link or decorated deleted row")
	}
	e = linkEditor([]string{"https://gap.test", "https://visible.test"}, 40, 8)
	e.SetDiffOverlay(&DiffOverlay{Labels: map[int]string{0: "https://gap.test"}})
	grid = hoverEditorLinks(e)
	if len(e.links.hits) == 0 {
		t.Fatal("real link below gap missing")
	}
	for x, cell := range grid[0] {
		_, hit := e.editorLinkAt(x, 0)
		if cell.Underline || hit {
			t.Fatalf("gap label feedback at %d", x)
		}
	}
}

func TestEditorLinksExcludeGuttersAndScrollbars(t *testing.T) {
	e := linkEditor([]string{
		"https://example.com/long/path/for/scrolling", "plain", "plain", "plain", "plain",
	}, 25, 4)
	e.LineNumbers = true
	grid := hoverEditorLinks(e)
	if len(e.links.hits) == 0 || !e.scrollbar.Visible() || !e.hscrollbar.Visible() {
		t.Fatal("missing link or scrollbar fixture")
	}
	for y, row := range grid {
		for x, cell := range row {
			if x < e.GutterWidth() || x == e.scrollbar.X || y == e.hscrollbar.Y {
				_, hit := e.editorLinkAt(x, y)
				if cell.Underline || hit {
					t.Fatalf("non-text feedback at %d,%d", x, y)
				}
			}
		}
	}
	e = linkEditor([]string{"https://example.com"}, 40, 5)
	grid = hoverEditorLinks(e)
	for x, cell := range grid[4] {
		_, hit := e.editorLinkAt(x, 4)
		if cell.Underline || hit {
			t.Fatal("blank row is interactive")
		}
	}
}

func TestEditorLinksKeepDiagnosticUnderline(t *testing.T) {
	e := linkEditor([]string{"https://example.com"}, 40, 4)
	e.Diagnostics = []Diagnostic{{EndCol: 5, Severity: DiagError}}
	e.diagByLine = map[int][]int{0: {0}}
	before := renderGrid(e)
	after := hoverEditorLinks(e)
	p := linkPoint{e.GutterWidth() + 1, 0}
	if before[p.y][p.x].UlStyle == 0 || after[p.y][p.x] != before[p.y][p.x] {
		t.Fatal("link replaced diagnostic styling")
	}
	if _, ok := e.editorLinkAt(p.x, p.y); !ok {
		t.Fatal("diagnostic disabled link navigation")
	}
}

func TestEditorLinksStoreStylesOnlyWhileActive(t *testing.T) {
	e := linkEditor([]string{"https://example.com"}, 40, 4)
	renderGrid(e)
	if len(e.links.cells) == 0 || len(e.links.styles) != 0 || e.links.hitsReady {
		t.Fatal("idle render did not keep only geometry")
	}
	hoverEditorLinks(e)
	if len(e.links.styles) != len(e.links.cells) || !e.links.hitsReady {
		t.Fatal("active render did not capture styles and prepare hit targets")
	}
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth(), 0, 0, 0))
	renderGrid(e)
	if len(e.links.styles) != 0 || e.links.hitsReady {
		t.Fatal("inactive render retained active feedback")
	}
}

func TestEditorLinksWideContinuationKeepsAttributes(t *testing.T) {
	e := linkEditor([]string{"https://example.com/界"}, 40, 4)
	e.SetDiffOverlay(&DiffOverlay{Spans: map[int][]ColSpan{
		0: {{Start: 0, End: 30, Style: term.StyleReverse}},
	}})
	grid := hoverEditorLinks(e)
	for x, cell := range grid[0] {
		if cell.Ch == '界' {
			continuation := grid[0][x+1]
			if cell.Attrs&term.CellAttrReverse == 0 || continuation.Attrs != cell.Attrs || continuation.Bold != cell.Bold {
				t.Fatalf("wide continuation lost attributes: leading=%+v next=%+v", cell, continuation)
			}
			return
		}
	}
	t.Fatal("wide link rune not visible")
}

func TestEditorLinkPressKeepsGestureAfterCtrlReleased(t *testing.T) {
	e := linkEditor([]string{"https://example.com"}, 40, 4)
	opened := 0
	e.OnOpenURL = func(string) { opened++ }
	renderGrid(e)
	x := e.GutterWidth() + 1
	e.HandleEvent(tcell.NewEventMouse(x, 0, tcell.Button1, tcell.ModCtrl))
	e.HandleEvent(tcell.NewEventMouse(x+5, 0, tcell.Button1, 0))
	if opened != 1 || e.mouseDown || e.Selection.Active {
		t.Fatal("releasing Ctrl during a link gesture started a text selection")
	}
	e.HandleEvent(tcell.NewEventMouse(x+5, 0, 0, 0))
	e.HandleEvent(tcell.NewEventMouse(x+5, 0, tcell.Button1, 0))
	if !e.mouseDown {
		t.Fatal("ordinary click was not restored after mouse release")
	}
}

func TestEditorLinkPointerYieldsToTextDrag(t *testing.T) {
	e := linkEditor([]string{"plain https://example.com"}, 40, 4)
	hoverEditorLinks(e)
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth(), 0, tcell.Button1, tcell.ModCtrl))
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+10, 0, tcell.Button1, tcell.ModCtrl))
	if !e.mouseDown || e.PointerShapeAt(e.GutterWidth()+10, 0) != term.PointerText {
		t.Fatal("link pointer overrode an ongoing text drag")
	}
}

func TestEditorLinksWithCallbacksRemoved(t *testing.T) {
	e := linkEditor([]string{"https://example.com"}, 40, 4)
	hoverEditorLinks(e)
	e.OnOpenURL = nil
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+1, 0, tcell.Button1, tcell.ModCtrl))
	if e.links.active {
		t.Fatal("link feedback remained active without callbacks")
	}
	e = linkEditor([]string{"https://example.com"}, 40, 4)
	e.OnOpenURL = nil
	e.links.detect = func(string, string) []linkSpan { t.Fatal("detection without callbacks"); return nil }
	hoverEditorLinks(e)
	if e.links.active || len(e.links.cells) != 0 {
		t.Fatal("disabled embedded pane collected link geometry")
	}
}

func TestEditorLinkPressRecoversAfterLostRelease(t *testing.T) {
	for _, ev := range []tcell.Event{tcell.NewEventFocus(false), tcell.NewEventKey(tcell.KeyEscape, "", 0)} {
		e := linkEditor([]string{"https://example.com"}, 40, 4)
		r := NewRoot(e)
		e.OnLinkHover = r.TrackEditorLinkHover
		hoverEditorLinks(e)
		r.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+1, 0, tcell.Button1, tcell.ModCtrl))
		r.HandleEvent(ev)
		if e.links.pressed || e.links.active || e.PointerShapeAt(e.GutterWidth()+1, 0) != term.PointerText {
			t.Fatal("focus/modifier transition left link press or pointer active")
		}
		r.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+8, 0, tcell.Button1, 0))
		if !e.mouseDown || e.Cursor.Col != 8 {
			t.Fatal("ordinary click was swallowed after a lost release")
		}
	}
}

func TestEditorLinksPreserveCellStylesAndSelection(t *testing.T) {
	e := linkEditor([]string{"https://example.com"}, 40, 5)
	e.Selection.Start(0, 1)
	e.Cursor.Col = 3
	e.SearchMatches = []FindMatch{{Line: 0, Col: 5, Len: 4}}
	e.searchByLine = map[int][]int{0: {0}}
	e.SetDiffOverlay(&DiffOverlay{Kinds: []diff.LineKind{diff.Added}})
	before := renderGrid(e)
	after := hoverEditorLinks(e)
	for p := range e.links.hits {
		c := after[p.y][p.x]
		c.Underline = false
		if c != before[p.y][p.x] {
			t.Fatalf("link changed existing styling at %v", p)
		}
	}
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+1, 0, tcell.Button1, 0))
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+4, 0, tcell.Button1, 0))
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+4, 0, 0, 0))
	if !e.Selection.Active || e.Cursor.Col != 4 || e.links.active {
		t.Fatal("ordinary drag selection changed")
	}
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+7, 0, tcell.Button1, tcell.ModAlt))
	if !e.isMultiActive() {
		t.Fatal("Alt-click did not create a cursor")
	}
}

func TestEditorLinksCacheAndInvalidation(t *testing.T) {
	e := linkEditor([]string{"https://first.test", "https://second.test"}, 30, 2)
	count := 0
	e.links.detect = func(s, dir string) []linkSpan {
		count++
		return detectLinks(s, dir)
	}
	hoverEditorLinks(e)
	initial := count
	for range 5 {
		e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+2, 0, 0, tcell.ModCtrl))
		renderGrid(e)
	}
	if count != initial {
		t.Fatalf("unchanged hover reran detection: %d -> %d", initial, count)
	}
	e = linkEditor([]string{
		"before https://first.test/long/path", "before https://second.test/long/path",
		"plain", "plain", "plain", "plain",
	}, 30, 3)
	hoverEditorLinks(e)
	for _, tt := range []struct {
		change func()
		url    string
	}{
		{func() { e.Viewport.LeftCol = 12 }, "https://first.test/long/path"},
		{func() { e.Viewport.TopLine = 1 }, "https://second.test/long/path"},
		{func() { r := e.GetRect(); r.W = 22; e.SetRect(r) }, "https://second.test/long/path"},
		{func() { e.exec(&undo.InsertStringCommand{Line: 1, Col: 15, Text: "new-"}) }, "https://new-second.test/long/path"},
		{func() { e.LinkWorkDir = t.TempDir() }, "https://new-second.test/long/path"},
	} {
		tt.change()
		x := e.GutterWidth() + 1
		if _, ok := e.editorLinkAt(x, 0); ok {
			t.Fatal("stale geometry remained clickable before redraw")
		}
		grid := renderGrid(e)
		span, ok := e.editorLinkAt(x, 0)
		if !ok || span.URL != tt.url || !grid[0][x].Underline {
			t.Fatalf("refreshed link at %d,0 = %+v, want %s", x, span, tt.url)
		}
	}
}

func TestEditorLinksVisibleWorkBounded(t *testing.T) {
	lines := make([]string, 100_000)
	for i := range lines {
		lines[i] = fmt.Sprintf("https://line%d.test", i)
	}
	e := linkEditor(lines, 30, 4)
	visited := []string{}
	e.links.detect = func(s, dir string) []linkSpan {
		visited = append(visited, s)
		return detectLinks(s, dir)
	}
	renderGrid(e)
	if len(visited) != 0 {
		t.Fatal("detection ran without Ctrl-hover")
	}
	hoverEditorLinks(e)
	if len(visited) != e.Viewport.Height {
		t.Fatalf("visited %d lines for viewport height %d", len(visited), e.Viewport.Height)
	}
	for i, s := range visited {
		if s != lines[i] {
			t.Fatalf("offscreen detection: %q", s)
		}
	}
	long := "https://" + strings.Repeat("x", 2*1024*1024)
	e = linkEditor([]string{long}, 35, 5)
	e.Viewport.LeftCol = 1_000_000
	calls, scanned := 0, 0
	e.links.detect = func(s, dir string) []linkSpan {
		calls++
		scanned += len(s)
		return detectLinks(s, dir)
	}
	hoverEditorLinks(e)
	for range 3 {
		renderGrid(e)
	}
	if calls != 1 || scanned > 2*editorLinkLookaround+35 || len(e.links.hits) != 0 {
		t.Fatalf("long line: calls=%d bytes=%d hits=%d", calls, scanned, len(e.links.hits))
	}
	e = linkEditor([]string{strings.Repeat("x", 1_000_000) + " https://tail.test " + strings.Repeat("x", 1_000_000)}, 35, 5)
	e.Viewport.LeftCol = 1_000_005
	e.links.detect = func(s, dir string) []linkSpan {
		if len(s) > 2*editorLinkLookaround+35 {
			t.Fatalf("scanned %d hidden bytes", len(s))
		}
		return detectLinks(s, dir)
	}
	opened := ""
	e.OnOpenURL = func(url string) { opened = url }
	renderGrid(e)
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+1, 0, tcell.Button1, tcell.ModCtrl))
	if opened != "https://tail.test" {
		t.Fatalf("first click on a very long line opened %q", opened)
	}
	e = linkEditor([]string{"https://example.com/" + strings.Repeat("a", 70)}, 20, 8)
	e.WordWrap = true
	calls = 0
	e.links.detect = func(s, dir string) []linkSpan { calls++; return detectLinks(s, dir) }
	hoverEditorLinks(e)
	if calls != 1 || len(e.links.hits) == 0 {
		t.Fatalf("wrapped line detection: calls=%d hits=%d", calls, len(e.links.hits))
	}
	e.scrollDown(1)
	renderGrid(e)
	for _, span := range e.links.hits {
		if span.URL != e.Buf.Lines[0] {
			t.Fatalf("wrapped continuation opened a truncated link: %q", span.URL)
		}
	}
}

func TestEditorFileLinksResolveOncePerContext(t *testing.T) {
	e := linkEditor([]string{"target.go:12:3"}, 40, 4)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "target.go"), []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	e.LinkWorkDir = dir
	var path string
	e.OnOpenFile = func(p string, line, col int) {
		path = p
		if line != 12 || col != 3 {
			t.Fatalf("position = %d:%d", line, col)
		}
	}
	calls := 0
	e.links.detect = func(s, dir string) []linkSpan { calls++; return detectLinks(s, dir) }
	hoverEditorLinks(e)
	for range 5 {
		renderGrid(e)
	}
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+1, 0, tcell.Button1, tcell.ModCtrl))
	if calls != 1 || path != filepath.Join(dir, "target.go") {
		t.Fatalf("calls=%d path=%q", calls, path)
	}
	e.LinkWorkDir = t.TempDir()
	renderGrid(e)
	if calls != 2 || len(e.links.hits) != 0 {
		t.Fatal("file resolution was not invalidated on directory change")
	}
}

func TestEditorLinkFeedbackClearedThroughRoot(t *testing.T) {
	for _, ev := range []tcell.Event{
		tcell.NewEventMouse(100, 100, 0, tcell.ModCtrl),
		tcell.NewEventMouse(2, 0, 0, 0),
		tcell.NewEventKey(tcell.KeyEscape, "", 0),
		tcell.NewEventFocus(false),
	} {
		e := linkEditor([]string{"https://example.com"}, 40, 4)
		r := NewRoot(e)
		e.OnLinkHover = r.TrackEditorLinkHover
		hoverEditorLinks(e)
		r.HandleEvent(ev)
		if e.links.active || len(e.links.hits) != 0 {
			t.Fatalf("feedback survived %T", ev)
		}
	}
	e := linkEditor([]string{"https://example.com"}, 40, 4)
	r := NewRoot(e)
	e.OnLinkHover = r.TrackEditorLinkHover
	hoverEditorLinks(e)
	r.PushOverlay(Overlay{Widget: &BaseWidget{}, Modal: true})
	if e.links.active {
		t.Fatal("feedback survived overlay")
	}
}
